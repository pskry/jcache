package jcache

import (
	"bytes"
	"os/exec"
	"syscall"
)

type ExecInfo struct {
	Stdout string
	Stderr string
	Exit   int
}

func (e ExecInfo) Combined() string {
	return e.Stdout + e.Stderr
}

func Command(name string, args ...string) (*ExecInfo, error) {
	cmd := exec.Command(name, args...)

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	info := &ExecInfo{
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
