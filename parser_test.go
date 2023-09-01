package iscript_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ira-package-manager/iscript"
)

func TestParsingInstallSection(t *testing.T) {
	os.RemoveAll("./test/out/.ira")
	os.RemoveAll("./test/out/newdir")
	parser, err := iscript.NewParser(filepath.Join(".", "test", "pkg", "iscript_test"), filepath.Join(".", "test", "out"))
	if err != nil {
		t.Fatal(err)
	}
	parser.Debug = true
	err = parser.Start(iscript.Install, filepath.Join(".", "test", "pkg"))
	if err != nil {
		t.Fatal(err)
	}
	if !exists("./test/out/newdir") {
		t.Fatal("Directory wasn't created")
	}
	if !exists("./test/out/newdir/script.bat") {
		t.Fatal("Script wasn't installed")
	}
	if !exists("/home/andev/link") {
		t.Fatal("Link wasn't created")
	}
	os.Remove("/home/andev/link")
}

func TestParsingRemoveSection(t *testing.T) {
	parser, err := iscript.NewParser(filepath.Join(".", "test", "pkg", "iscript_test"), filepath.Join(".", "test", "pkg"))
	if err != nil {
		t.Fatal(err)
	}
	parser.Debug = true
	err = parser.Start(iscript.Remove, "")
	if err != nil {
		t.Fatal(err)
	}
}

func exists(filePath string) bool {
	if _, err := os.Lstat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}
