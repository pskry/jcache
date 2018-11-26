package jcache

import (
	"os"
)

type ExecInfo struct {
	Stdout string
	Stderr string
	Exit   int
}

func UnmarshalExecInfo(path string) (info *ExecInfo, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	info = &ExecInfo{}
	dec := NewDecoder(file)
	err = dec.Decode(info)
	return
}

func MarshalExecInfo(info *ExecInfo, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := NewEncoder(file)
	return enc.Encode(info)
}
