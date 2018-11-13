package jcache

import (
	"bytes"
	"os/exec"
	"syscall"
)

func Command(name string, args ...string) (fOut string, fErr string, exit int, err error) {
	cmd := exec.Command(name, args...)

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	cmdErr := cmd.Run()
	exit = 0
	fOut = outBuf.String()
	fErr = errBuf.String()

	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			ws := exitErr.Sys().(syscall.WaitStatus)
			exit = ws.ExitStatus()
		} else {
			err = cmdErr
		}
	}

	return
}
