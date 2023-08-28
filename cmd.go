package iscript

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func runInstallCmd(str, srcDir, destdir string) error {
	str = strings.ReplaceAll(str, "$destdir", destdir)
	str = strings.ReplaceAll(str, "$srcDir", srcDir)
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd(): %v", err)
	}
	err = os.Chdir(srcDir)
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
	log.Println(out)
	err = os.Chdir(currentDir)
	if err != nil {
		return fmt.Errorf("restoring work dir: %v", err)
	}
	return nil
}
