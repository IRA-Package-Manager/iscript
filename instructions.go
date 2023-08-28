package iscript

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"
)

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

func (p *Parser) runCmd(mode int, srcDir string) error {
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
		return runInstallCmd(str, srcDir, p.installDir)
	}
	return fmt.Errorf("unsupported mode: %d", mode)
}

func (p *Parser) installPath(mode int, srcDir string) error {
	if p.token != scanner.Int {
		return fmt.Errorf("bad syntax after install: want int, got %q", p.text())
	}
	perm, err := strconv.ParseInt(p.text(), 8, 0)
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
	fmt.Println(perm, fs.FileMode(perm))
	dest = filepath.Join(p.installDir, dest)
	err = createIfNotExists(filepath.Dir(dest), fs.FileMode(perm))
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
	src = filepath.Join(srcDir, src)
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
	return nil
}

func (p *Parser) createLinkFromPackage(mode int, srcDir string) error {
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
	symlink, err := strconv.Unquote(p.text())
	if err != nil {
		// scanner.String is double-quoted strng, must be no errors
		panic(err)
	}
	if !filepath.IsAbs(symlink) {
		return fmt.Errorf("%q must be absolute path", symlink)
	}

	err = createIfNotExists(filepath.Join(p.installDir, ".ira"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating configuration dir: %v", err)
	}

	log, err := os.OpenFile(filepath.Join(p.installDir, ".ira", "activate.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
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
	return nil
}
