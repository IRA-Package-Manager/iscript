package iscript

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/scanner"
)

const (
	Install int = iota
	Remove
	Update
)

type Parser struct {
	scan       scanner.Scanner
	mu         sync.Mutex
	token      rune
	installDir string
	SrcDir     string
	working    bool
}

func (p *Parser) Init(path string, installDir string) error {
	if p.working {
		return fmt.Errorf("Parser is working now. Stop it before re-initialising")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file %s: %v", path, err)
	}
	p.scan = scanner.Scanner{Mode: scanner.GoTokens}
	p.scan.Init(file)
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		return fmt.Errorf("installDir doesn't exist")
	}
	p.installDir = installDir
	return nil
}

func (p *Parser) next()        { p.token = p.scan.Scan() }
func (p *Parser) text() string { return p.scan.TokenText() }

func (p *Parser) Start(mode int) error {
	p.mu.Lock()
	p.working = true
	var flag string
	flagParsed := false
	switch mode {
	case Install:
		flag = ":install:"
		if p.SrcDir == "" {
			return fmt.Errorf("SrcDir wasn't set. Please set it before strarting installation")
		}
		if _, err := os.Stat(p.SrcDir); os.IsNotExist(err) {
			return fmt.Errorf("SrcDir doesn't exist")
		}
	case Remove, Update:
		return fmt.Errorf("mode %d not yet implemented", mode)
	default:
		return fmt.Errorf("wrong mode. Must be 0 (install), 1 (remove) or 2 (update)")
	}
	for p.next(); p.token != scanner.EOF; p.next() {
		switch p.token {
		case scanner.Ident:
			if p.text() == flag {
				if flagParsed {
					return fmt.Errorf("flag %s already parsed", flag)
				}
				flagParsed = true
			}
			if !flagParsed {
				continue
			}
			p.parseCommand(mode)
		}
	}
	p.working = false
	p.mu.Unlock()
	return nil
}
func (p *Parser) parseCommand(mode int) error {
	switch p.text() {
	case "cmdlin":
		if runtime.GOOS != "linux" {
			return nil
		}
		p.next()
		p.runCmd(mode)
	case "cmdwin":
		if runtime.GOOS != "windows" {
			return nil
		}
		p.next()
		p.runCmd(mode)
	case "install":
		if mode != Install {
			return fmt.Errorf("install operation is allowed only in install mode")
		}
		p.next()
		if p.token != scanner.Int {
			return fmt.Errorf("bad syntax after install: want int, got %q", p.text())
		}
		perm, err := strconv.ParseInt(p.text(), 0, 0)
		if err != nil {
			// strange situation, drop panic
			panic(err)
		}
		p.next()
		if p.token != scanner.Ident {
			return fmt.Errorf("bad syntax after install %d: want identifier, got %q", perm, p.text())
		}
		flag := p.text()
		p.next()
		if p.token != scanner.String {
			return fmt.Errorf("bad syntax after install %d %s: want string, got %q", perm, flag, p.text())
		}
		dest, err := strconv.Unquote(p.text())
		if err != nil {
			// scanner.String is double-quoted strng, must be no errors
			panic(err)
		}
		oldDest := dest
		dest, ok := validatePath(dest)
		if !ok {
			return fmt.Errorf("incorrect path %q", dest)
		}
		dest = filepath.Join(p.installDir, dest)
		err = createIfNotExists(dest, fs.FileMode(perm))
		if err != nil {
			return fmt.Errorf("creating destination dir: %v", err)
		}
		if flag == "mkdir" {
			return nil
		}
		p.next()
		if p.token != scanner.String {
			return fmt.Errorf("bad syntax after install %d %s %q: want string, got %q", perm, flag, oldDest, p.text())
		}
		src, err := strconv.Unquote(p.text())
		if err != nil {
			// scanner.String is double-quoted strng, must be no errors
			panic(err)
		}
		src, ok = validatePath(src)
		if !ok {
			return fmt.Errorf("incorrect path %q", src)
		}
		src = filepath.Join(p.SrcDir, src)
		stats, err := os.Stat(src)
		if os.IsNotExist(err) {
			return fmt.Errorf("source dir %q doesn't exists", src)
		} else if err != nil {
			return fmt.Errorf("getting stats about %q: %v", src, err)
		}
		switch stats.Mode() & os.ModeType {
		case os.ModeDir:
			err = copyDirectory(src, dest)
		case os.ModeSymlink:
			err = copySymLink(src, dest)
		default:
			err = copy(src, dest)
		}
		if err != nil {
			return fmt.Errorf("copying %q to %q: %v", src, dest, err)
		}
	case "activate":
		if mode != Install {
			return fmt.Errorf("activation is allowed only in install mode")
		}
		p.next()
		if p.token != scanner.String {
			return fmt.Errorf("bad syntax after activate: want string, got %q", p.text())
		}
		path, err := strconv.Unquote(p.text())
		if err != nil {
			// scanner.String is double-quoted strng, must be no errors
			panic(err)
		}
		oldPath := path
		path, ok := validatePath(path)
		if !ok {
			return fmt.Errorf("incorrect path %q", path)
		}
		path = filepath.Join(p.installDir, path)

		p.next()
		if p.token != scanner.String {
			return fmt.Errorf("bad syntax after activate %q: want string, got %q", oldPath, p.text())
		}
		symlink := p.text()
		if !filepath.IsAbs(symlink) {
			return fmt.Errorf("%q must be absolute path", symlink)
		}

		err = createIfNotExists(filepath.Join(p.installDir, ".ira"), os.ModePerm)
		if err != nil {
			return fmt.Errorf("creating configuration dir: %v", err)
		}

		log, err := os.OpenFile(filepath.Join(p.installDir, ".ira", "activate.log"), os.O_APPEND&os.O_CREATE&os.O_RDWR, os.ModePerm)
		if err != nil {
			return fmt.Errorf("opening activation log: %v", err)
		}
		defer log.Close()
		err = os.Symlink(path, symlink)
		if err != nil {
			return fmt.Errorf("creating symbolic link: %v", err)
		}
		_, err = log.Write(append([]byte(symlink), '\n'))
		if err != nil {
			return fmt.Errorf("writing %q in log: %v", symlink, err)
		}
	}
	return nil
}

func (p *Parser) runCmd(mode int) error {
	if p.token != scanner.String {
		return fmt.Errorf("bad syntax after cmd: want string, got %q", p.text())
	}
	str, err := strconv.Unquote(p.text())
	if err != nil {
		// scanner.String is double-quoted strng, must be no errors
		panic(err)
	}
	switch mode {
	case Install:
		return runInstallCmd(str, p.SrcDir, p.installDir)
	}
	return fmt.Errorf("unsupported mode: %d", mode)
}

func validatePath(path string) (string, bool) {
	path = strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "/")
	depth := 0
	directories := strings.Split(path, "/")
	for _, directory := range directories {
		if directory == ".." {
			depth--
		} else {
			depth++
		}
	}
	return filepath.Join(directories...), depth >= 0
}

func runInstallCmd(str, srcdir, destdir string) error {
	str = strings.ReplaceAll(str, "$destdir", destdir)
	str = strings.ReplaceAll(str, "$srcdir", srcdir)
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd(): %v", err)
	}
	err = os.Chdir(srcdir)
	if err != nil {
		return fmt.Errorf("os.Chdir(): %v", err)
	}
	args := strings.Split(str, " ")
	var cmd *exec.Cmd
	if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("running command: %v", err)
	}
	log.Println(out)
	err = os.Chdir(currentDir)
	if err != nil {
		return fmt.Errorf("restoring work dir: %v", err)
	}
	return nil
}

func copyDirectory(scrDir, dest string) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := createIfNotExists(destPath, 0755); err != nil {
				return err
			}
			if err := copyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := copySymLink(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := copy(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}

	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func createIfNotExists(dir string, perm os.FileMode) error {
	if exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func copySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}
