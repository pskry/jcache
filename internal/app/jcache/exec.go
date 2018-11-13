package jcache

import (
	"bytes"
	"os/exec"
	"syscall"
)

func Command(name string, args ...string) (*CompilerInfo, error) {
	cmd := exec.Command(name, args...)

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	info := &CompilerInfo{
		Stdout: outBuf.String(),
		Stderr: errBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			ws := exitErr.Sys().(syscall.WaitStatus)
			info.Exit = ws.ExitStatus()
		} else {
			return nil, err
		}
	}

	return info, nil
}
