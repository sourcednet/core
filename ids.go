package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ID prefixes for records and chunks.
const (
	RecordIDPrefix = "sr:sha256:"
	ChunkIDPrefix  = "sc:sha256:"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ChunkID returns the ID of a chunk: the SHA-256 of its UTF-8 text bytes.
func ChunkID(text string) string {
	return ChunkIDPrefix + sha256Hex([]byte(text))
}

// IDHex checks that id has the given prefix followed by a SHA-256 digest in
// lowercase hex, and returns the digest.
func IDHex(id, prefix string) (string, error) {
	h, ok := strings.CutPrefix(id, prefix)
	if !ok {
		return "", fmt.Errorf("id %q: want prefix %q", id, prefix)
	}
	if !isHexDigest(h) {
		return "", fmt.Errorf("id %q: want 64 lowercase hex characters after the prefix", id)
	}
	return h, nil
}

func isHexDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
