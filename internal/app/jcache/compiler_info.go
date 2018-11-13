package jcache

import (
	"os"
)

type CompilerInfo struct {
	Stdout string
	Stderr string
	Exit   int
}

func UnmarshalCompilerInfo(path string) (info *CompilerInfo, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	info = &CompilerInfo{}
	dec := NewDecoder(file)
	err = dec.Decode(info)
	return
}

func MarshalCompilerInfo(info *CompilerInfo, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := NewEncoder(file)
	return enc.Encode(info)
}
