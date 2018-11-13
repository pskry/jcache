package jcache

import (
	"encoding/json"
	"os"
)

type CompilerInfo struct {
	Out  string
	Err  string
	Exit int
}

func UnmarshalCompilerInfo(path string) (info CompilerInfo, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	err = dec.Decode(&info)
	return
}

func MarshalCompilerInfo(info CompilerInfo, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	return enc.Encode(info)
}
