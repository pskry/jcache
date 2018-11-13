package jcache

import (
	"os"
	"time"
)

type FileInfo struct {
	Path    string
	ModTime time.Time
	Sha256  string
}

func UnmarshalFileInfoSlice(path string) (infoSlice []FileInfo, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	dec := NewDecoder(file)
	err = dec.Decode(&infoSlice)
	return
}

func MarshalFileInfoSlice(paths []string, outFile string) error {
	file, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer file.Close()

	infoSlice, err := collectFileInfos(paths)
	if err != nil {
		return err
	}

	enc := NewEncoder(file)
	return enc.Encode(infoSlice)
}

func collectFileInfos(paths []string) ([]FileInfo, error) {
	var infoSlice []FileInfo

	for _, src := range paths {
		stat, err := os.Stat(src)
		if err != nil {
			return nil, err
		}

		fileDigest, err := Sha256File(src)
		if err != nil {
			return nil, err
		}

		info := FileInfo{
			Path:    src,
			ModTime: stat.ModTime().UTC(),
			Sha256:  fileDigest,
		}
		infoSlice = append(infoSlice, info)
	}

	return infoSlice, nil
}
