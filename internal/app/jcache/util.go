package jcache

import (
	"crypto/sha256"
	"encoding/hex"
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

func DoesNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
