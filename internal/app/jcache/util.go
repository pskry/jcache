package jcache

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
)

func Sha256File(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	sum256 := sha256.Sum256(bytes)
	return hex.EncodeToString(sum256[:]), nil
}

func CopyFile(from, to string) (int64, error) {
	src, err := os.Open(from)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	dst, err := os.Create(to)
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	return io.Copy(dst, src)
}

func DoesNotExist(dir string) bool {
	_, err := os.Stat(dir)
	return os.IsNotExist(err)
}
