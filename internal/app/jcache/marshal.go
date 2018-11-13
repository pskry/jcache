package jcache

import (
	"encoding/json"
	"io"
)

type EncoderFacade interface{ Encode(v interface{}) error }
type DecoderFacade interface{ Decode(v interface{}) error }

var NewEncoder = func(w io.Writer) EncoderFacade {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}

var NewDecoder = func(w io.Reader) DecoderFacade {
	dec := json.NewDecoder(w)
	dec.DisallowUnknownFields()
	return dec
}
