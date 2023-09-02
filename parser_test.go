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
		t.Error("Directory wasn't created")
	} else if !exists("./test/out/newdir/script.bat") {
		t.Error("Script wasn't installed")
	}
	if !exists("./test/pkg/testfile.txt") {
		t.Error("Command wasn't executed")
	}
	if !exists("/tmp/link") {
		t.Error("Link wasn't created")
	}
	os.Remove("/tmp/link")
	os.Remove("./test/pkg/testfile.txt")
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
