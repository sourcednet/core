package core

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const testPublisher = "example-library.test"

// testKey derives a deterministic key pair from a seed phrase. Test use only.
func testKey(seed string) (ed25519.PublicKey, ed25519.PrivateKey) {
	s := sha256.Sum256([]byte(seed))
	priv := ed25519.NewKeyFromSeed(s[:])
	return priv.Public().(ed25519.PublicKey), priv
}

type testSigner struct {
	keys  *KeySet
	privs map[string]ed25519.PrivateKey
}

// newTestSigner returns a key set with an active, a retired, and a revoked key.
func newTestSigner(t *testing.T) *testSigner {
	t.Helper()
	ts := &testSigner{
		keys:  &KeySet{Spec: SpecVersion, Publisher: testPublisher},
		privs: map[string]ed25519.PrivateKey{},
	}
	for _, k := range []struct {
		id     string
		status KeyStatus
	}{{"2026a", KeyActive}, {"2025a", KeyRetired}, {"2024x", KeyRevoked}} {
		pub, priv := testKey("test key " + k.id)
		key := NewKey(k.id, pub)
		key.Status = k.status
		ts.keys.Keys = append(ts.keys.Keys, key)
		ts.privs[k.id] = priv
	}
	return ts
}

func testChunks(t *testing.T, texts ...string) []Chunk {
	t.Helper()
	var cs []Chunk
	for i, s := range texts {
		cs = append(cs, Chunk{Text: s, Section: []string{"Section " + string(rune('A'+i))}})
	}
	return cs
}

func testMeta(path string) PageMeta {
	return PageMeta{
		Publisher:   testPublisher,
		URL:         "https://" + testPublisher + path,
		Title:       "Caring for old books",
		PublishedAt: time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC),
		Language:    "en",
	}
}

// signedRecord builds and signs a record, returning it and its raw JSON.
func (ts *testSigner) signedRecord(t *testing.T, keyID string, meta PageMeta, chunks []Chunk) (*Record, []byte) {
	t.Helper()
	r := NewRecord(meta, chunks)
	if err := SignRecord(r, keyID, ts.privs[keyID]); err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	return r, raw
}

// mutate decodes raw JSON, applies f, and re-encodes it.
func mutate(t *testing.T, raw []byte, f func(map[string]any)) []byte {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	f(obj)
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func wantReason(t *testing.T, err error, want Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want %s", want)
	}
	if got := ReasonOf(err); got != want {
		t.Fatalf("got reason %q (%v), want %q", got, err, want)
	}
}

func repeat(s string, n int) string { return strings.Repeat(s, n) }
