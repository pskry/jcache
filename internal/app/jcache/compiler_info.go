package jcache

import (
	"encoding/json"
	"io/ioutil"
)

type CompilerInfo struct {
	Out  string
	Err  string
	Exit int
}

func UnmarshalCompilerInfo(path string) (info CompilerInfo, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &info)
	if err != nil {
		return
	}

	return
}

func MarshalCompilerInfo(info CompilerInfo, path string) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
