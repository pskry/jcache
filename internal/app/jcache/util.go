package jcache

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
)

func Sha256File(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	sum256 := sha256.Sum256(bytes)
	return hex.EncodeToString(sum256[:]), nil
}
