package jcache

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

type SourceInfo struct {
	Path    string
	ModTime time.Time
	Sha256  string
}

func UnmarshalSourceInfoSlice(path string) ([]SourceInfo, error) {
	var infoSlice []SourceInfo

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &infoSlice)
	if err != nil {
		return nil, err
	}

	return infoSlice, nil
}

func MarshalSourceInfoSlice(sources []string, path string) error {
	var infoSlice []SourceInfo

	for _, src := range sources {
		stat, err := os.Stat(src)
		if err != nil {
			return err
		}

		fileDigest, err := Sha256File(src)
		if err != nil {
			return err
		}

		info := SourceInfo{
			Path:    src,
			ModTime: stat.ModTime().UTC(),
			Sha256:  fileDigest,
		}
		infoSlice = append(infoSlice, info)
	}
	data, err := json.Marshal(infoSlice)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
