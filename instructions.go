package iscript

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
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
	// Note: ignoring errors
	absInstallDir, _ := filepath.Abs(p.installDir)
	var absSrcDir string
	if srcDir == "" {
		absSrcDir = ""
	} else {
		absSrcDir, _ = filepath.Abs(srcDir)
	}
	switch mode {
	case Install:
		str = strings.ReplaceAll(str, "$srcdir", absSrcDir)
		str = strings.ReplaceAll(str, "$destdir", absInstallDir)
	case Remove:
		str = strings.ReplaceAll(str, "$pkg", absInstallDir)
	case Update:
		str = strings.ReplaceAll(str, "$oldpkg", absSrcDir)
		str = strings.ReplaceAll(str, "$newpkg", absInstallDir)
	}
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd(): %v", err)
	}
	var workDir string
	if mode == Install || mode == Update {
		workDir = srcDir
	} else {
		workDir = p.installDir
	}
	err = os.Chdir(workDir)
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
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("running command: %v", err)
	}
	if len(out.Bytes()) != 0 {
		log.Println(out.String())
	}
	err = os.Chdir(currentDir)
	if err != nil {
		return fmt.Errorf("restoring work dir: %v", err)
	}
	return nil
}

func (p *Parser) installPath(srcDir string) error {
	if p.token != scanner.Int {
		return fmt.Errorf("bad syntax after install: want int, got %q", p.text())
	}
	perm, err := strconv.ParseInt(p.text(), 8, 0)
	if err != nil {
		// strange situation, drop panic
		panic(err)
	}
	p.next()
	if p.token != scanner.String {
		return fmt.Errorf("bad syntax after install %d: want string, got %q", perm, p.text())
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
	if p.Debug {
		fmt.Println(perm, fs.FileMode(perm))
	}
	dest = filepath.Join(p.installDir, dest)
	err = createIfNotExists(filepath.Dir(dest), fs.FileMode(perm))
	if err != nil {
		return fmt.Errorf("creating destination dir: %v", err)
	}
	p.next()
	if p.token != scanner.String {
		return fmt.Errorf("bad syntax after install %d %q: want string, got %q", perm, oldDest, p.text())
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

func (p *Parser) createLinkFromPackage(srcDir string) error {
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
	_, err = log.WriteString(path + " " + symlink + "\n")
	if err != nil {
		return fmt.Errorf("writing %q in log: %v", symlink, err)
	}
	return nil
}

func (p *Parser) remove() error {
	if p.token != scanner.String {
		return fmt.Errorf("bad syntax after remove: want string, got %q", p.text())
	}
	path, err := strconv.Unquote(p.text())
	if err != nil {
		panic(err)
	}
	path, ok := validatePath(path)
	if !ok {
		return fmt.Errorf("incorrect path %q", path)
	}
	err = os.RemoveAll(filepath.Join(p.installDir, path))
	if err != nil {
		return fmt.Errorf("removing %q: %v", path, err)
	}
	return nil
}
func (p *Parser) mkdir() error {
	if p.token != scanner.String {
		return fmt.Errorf("bad syntax after mkdir: want string, got %q", p.text())
	}
	path, err := strconv.Unquote(p.text())
	if err != nil {
		panic(err)
	}
	newPath, ok := validatePath(path)
	if !ok {
		return fmt.Errorf("incorrect path %q", path)
	}
	p.next()
	if p.token != scanner.Int {
		return fmt.Errorf("bad syntax after mkdir %q: want int, got %q", path, p.text())
	}
	perm, err := strconv.ParseInt(p.text(), 8, 0)
	if err != nil {
		panic(err)
	}
	if p.Debug {
		fmt.Println("mkdir", newPath, fs.FileMode(perm))
	}
	err = createIfNotExists(filepath.Join(p.installDir, newPath), fs.FileMode(perm))
	if err != nil {
		return fmt.Errorf("creating %q: %v", newPath, err)
	}
	return nil
}
