package jcache

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"os"
	"time"
)

type (
	EncoderFacade interface{ Encode(v interface{}) error }
	DecoderFacade interface{ Decode(v interface{}) error }

	FileInfo struct {
		Path    string
		ModTime time.Time
		Sha256  string
	}
)

func NewEncoder(w io.Writer) EncoderFacade {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}
func NewDecoder(w io.Reader) DecoderFacade {
	dec := json.NewDecoder(w)
	dec.DisallowUnknownFields()
	return dec
}

func MarshalExecInfo(info *ExecInfo, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()

	enc := NewEncoder(file)
	return enc.Encode(info)
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

func MarshalFileInfoSlice(paths []string, outFile string) error {
	file, err := os.Create(outFile)
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()

	var infoSlice []FileInfo
	for _, src := range paths {
		stat, err := os.Stat(src)
		if err != nil {
			return errors.WithStack(err)
		}

		fileDigest, err := Sha256File(src)
		if err != nil {
			return errors.WithStack(err)
		}

		info := FileInfo{
			Path:    src,
			ModTime: stat.ModTime().UTC(),
			Sha256:  fileDigest,
		}
		infoSlice = append(infoSlice, info)
	}

	enc := NewEncoder(file)
	return enc.Encode(infoSlice)
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
