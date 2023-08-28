package iscript

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"text/scanner"
)

// Parser is used to parse IScript
type Parser struct {
	scan       scanner.Scanner
	mu         sync.Mutex
	token      rune
	installDir string
	working    bool
}

func (p *Parser) init(path string, installDir string) error {
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

// NewParser creates a new parser and initialise it.
// path is a path to script. installDir is a directory where package is (or would be) installed
func NewParser(path string, installDir string) (*Parser, error) {
	parser := new(Parser)
	err := parser.init(path, installDir)
	return parser, err
}

// This function should be executed when either path or destination changed
// Parser must be stopped before resetting
// Syntax is the same as in NewParser
func (p *Parser) Reset(path string, installDir string) error {
	if p.working {
		return fmt.Errorf("Parser is working now. Stop it before re-initialising")
	}
	return p.init(path, installDir)
}

func (p *Parser) next()        { p.token = p.scan.Scan() }
func (p *Parser) text() string { return p.scan.TokenText() }

// This function starts parser. Mode set which section will be executed
// srcDir is a path to unpacked package. This parameter is required in Install and Update mode,
// but shouldn't be set in Remove mode (shold be blank string "")
func (p *Parser) Start(mode int, srcDir string) error {
	p.mu.Lock()
	defer func() {
		p.working = false
		p.mu.Unlock()
	}()
	p.working = true

	flag, ok := GetFlag(mode)
	if !ok {
		return fmt.Errorf("invalid modifier")
	}
	flagParsed := false
	if mode == Install || mode == Update {
		if srcDir == "" {
			return fmt.Errorf("srcDir wasn't set. You need to set it before strarting installation")
		}
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return fmt.Errorf("srcDir doesn't exist")
		}
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
			if IsFlag(p.text()) {
				break
			}
			p.parseCommand(mode, srcDir)
		}
	}
	return nil
}
func (p *Parser) parseCommand(mode int, srcDir string) error {
	switch p.text() {
	case "cmdlin":
		if runtime.GOOS != "linux" {
			return nil
		}
		p.next()
		return p.runCmd(mode, srcDir)
	case "cmdwin":
		if runtime.GOOS != "windows" {
			return nil
		}
		p.next()
		return p.runCmd(mode, srcDir)
	case "install":
		if mode != Install {
			return fmt.Errorf("install operation is allowed only in install mode")
		}
		p.next()
		return p.installPath(mode, srcDir)
	case "activate":
		if mode != Install {
			return fmt.Errorf("activation is allowed only in install mode")
		}
		p.next()
		return p.createLinkFromPackage(mode, srcDir)
	default:
		return fmt.Errorf("invalid command: %q", p.text())
	}
}
