package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/gowebpki/jcs"
)

// signingBytes returns B for a signed JSON object: the object without the
// excluded top-level members, serialized with the JSON Canonicalization
// Scheme (RFC 8785).
//
// B is always derived from the raw bytes, never from a decoded struct, so
// members this version doesn't know about stay covered by the hash and the
// signature. That is what keeps v1 verifiers compatible with later versions.
func signingBytes(raw []byte, exclude ...string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if obj == nil {
		return nil, errors.New("not a JSON object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON object")
	}
	for _, k := range exclude {
		delete(obj, k)
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return jcs.Transform(b)
}

// CanonicalBytes returns the JCS form of a JSON object without the excluded
// top-level members: the bytes that get hashed and signed. Anything signed
// the sourced way, such as a resolver's answers, uses it.
func CanonicalBytes(raw []byte, exclude ...string) ([]byte, error) {
	return signingBytes(raw, exclude...)
}
