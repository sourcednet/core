package core

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the golden test vectors in testdata/vectors")

const (
	vectorsDir       = "testdata/vectors"
	vectorsPublisher = "example-library.test"
)

// vectorKeySeed is the seed phrase for a test vector key. The keys are public
// by design: they exist so other implementations can reproduce the vectors.
func vectorKeySeed(id string) string { return "sourced.net test vector key " + id }

type vectorExpected struct {
	Publisher      string                  `json:"publisher"`
	KeySeeds       map[string]string       `json:"key_seeds"`
	ChunkParams    ChunkParams             `json:"chunk_params"`
	Records        map[string]vectorRecord `json:"records"`
	Manifest       string                  `json:"manifest_generated_at"`
	Citation       string                  `json:"example_citation"`
	UnchangedAfter int                     `json:"chunks_unchanged_by_correction"`
}

type vectorRecord struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	KeyID  string `json:"key_id"`
	Chunks int    `json:"chunks"`
	State  State  `json:"state"`
}

// buildVectors derives the whole vector tree from the Markdown sources. Every
// input is fixed, and Ed25519 signatures are deterministic, so the output is
// identical on every run.
func buildVectors(t *testing.T) map[string][]byte {
	t.Helper()
	keys := &KeySet{Spec: SpecVersion, Publisher: vectorsPublisher}
	privs := map[string]ed25519.PrivateKey{}
	seeds := map[string]string{}
	for _, k := range []struct {
		id     string
		status KeyStatus
	}{{"2026a", KeyActive}, {"2025a", KeyRetired}, {"2024x", KeyRevoked}} {
		pub, priv := testKey(vectorKeySeed(k.id))
		key := NewKey(k.id, pub)
		key.Status = k.status
		keys.Keys = append(keys.Keys, key)
		privs[k.id], seeds[k.id] = priv, vectorKeySeed(k.id)
	}

	chunkSource := func(name string) []Chunk {
		src, err := os.ReadFile(filepath.Join(vectorsDir, "source", name))
		if err != nil {
			t.Fatal(err)
		}
		cs, err := ChunkMarkdown(string(src), DefaultChunkParams)
		if err != nil {
			t.Fatal(err)
		}
		return cs
	}
	meta := func(path, title string, at time.Time) PageMeta {
		return PageMeta{
			Publisher:   vectorsPublisher,
			URL:         "https://" + vectorsPublisher + path,
			Title:       title,
			PublishedAt: at,
			Language:    "en",
			License:     "https://" + vectorsPublisher + "/license.xml",
		}
	}
	sign := func(r *Record, keyID string) {
		if err := SignRecord(r, keyID, privs[keyID]); err != nil {
			t.Fatal(err)
		}
	}

	careV1Chunks := chunkSource("book-care-v1.md")
	careV1 := NewRecord(meta("/guides/book-care", "Caring for old books", time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)), careV1Chunks)
	sign(careV1, "2026a")

	careV2Chunks := chunkSource("book-care-v2.md")
	careV2, err := NewSuccessor(careV1, ChangeCorrection,
		"Corrected the recommended relative humidity from 60–70% to 40–50%.",
		meta("/guides/book-care", "Caring for old books", time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)), careV2Chunks)
	if err != nil {
		t.Fatal(err)
	}
	sign(careV2, "2026a")

	digChunks := chunkSource("digitization.md")
	dig := NewRecord(meta("/guides/digitization", "Digitizing a collection", time.Date(2025, 6, 1, 8, 30, 0, 0, time.UTC)), digChunks)
	sign(dig, "2025a") // signed while 2025a was active; now retired but still valid

	manifest := &Manifest{
		Spec:        SpecVersion,
		Publisher:   vectorsPublisher,
		GeneratedAt: time.Date(2026, 9, 30, 9, 5, 0, 0, time.UTC),
		Entries: map[string]string{
			careV2.URL: careV2.ID,
			dig.URL:    dig.ID,
		},
	}
	if err := SignManifest(manifest, "2026a", privs["2026a"]); err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{}
	put := func(path string, v any) {
		b, err := EncodeJSON(v)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = b
	}
	base := vectorsPublisher + WellKnownPath
	put(base+"keys.json", keys)
	put(base+"manifest.json", manifest)
	for _, rc := range []struct {
		r      *Record
		chunks []Chunk
	}{{careV1, careV1Chunks}, {careV2, careV2Chunks}, {dig, digChunks}} {
		h, _ := IDHex(rc.r.ID, RecordIDPrefix)
		put(base+"records/"+h+".json", rc.r)
		b, err := NewBundle(rc.r, rc.chunks)
		if err != nil {
			t.Fatal(err)
		}
		put(base+"bundles/"+h+".json", b)
	}

	unchanged := 0
	for _, a := range careV1.Chunks {
		for _, b := range careV2.Chunks {
			if a.ID == b.ID {
				unchanged++
			}
		}
	}
	rec := func(r *Record, st State) vectorRecord {
		return vectorRecord{ID: r.ID, URL: r.URL, KeyID: r.Sig.KeyID, Chunks: len(r.Chunks), State: st}
	}
	put("expected.json", vectorExpected{
		Publisher:   vectorsPublisher,
		KeySeeds:    seeds,
		ChunkParams: DefaultChunkParams,
		Records: map[string]vectorRecord{
			"book-care-v1":    rec(careV1, StateCorrected),
			"book-care-v2":    rec(careV2, StateCurrent),
			"digitization-v1": rec(dig, StateCurrent),
		},
		Manifest:       manifest.GeneratedAt.Format(time.RFC3339),
		Citation:       Citation{Publisher: vectorsPublisher, Record: careV1.ID, Chunk: careV1.Chunks[2].ID}.String(),
		UnchangedAfter: unchanged,
	})
	return files
}

func TestVectors(t *testing.T) {
	files := buildVectors(t)
	if *update {
		for path, b := range files {
			full := filepath.Join(vectorsDir, path)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(vectorsDir, path))
		if err != nil {
			t.Fatalf("%v (run `make vectors` after a deliberate format change)", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from what the code produces (run `make vectors` if the change is deliberate)", path)
		}
	}
	checkVectorTree(t)
}

// checkVectorTree verifies the files on disk the way a verifier would,
// starting from keys.json and the manifest.
func checkVectorTree(t *testing.T) {
	base := filepath.Join(vectorsDir, vectorsPublisher, filepath.FromSlash(strings.TrimPrefix(WellKnownPath, "/")))
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	expected, err := os.ReadFile(filepath.Join(vectorsDir, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want vectorExpected
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}

	ks, err := ParseKeySet(read("keys.json"), vectorsPublisher)
	if err != nil {
		t.Fatal(err)
	}
	m, err := VerifyManifest(read("manifest.json"), ks, vectorsPublisher, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	load := func(id string) (*Record, error) {
		h, err := IDHex(id, RecordIDPrefix)
		if err != nil {
			return nil, err
		}
		return VerifyRecord(read("records/"+h+".json"), ks, vectorsPublisher)
	}

	for name, exp := range want.Records {
		r, err := load(exp.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		h, _ := IDHex(r.ID, RecordIDPrefix)
		var b Bundle
		if err := json.Unmarshal(read("bundles/"+h+".json"), &b); err != nil {
			t.Fatal(err)
		}
		if err := VerifyBundle(r, &b); err != nil {
			t.Fatalf("%s bundle: %v", name, err)
		}
		current, err := load(m.Entries[r.URL])
		if err != nil {
			t.Fatalf("%s current: %v", name, err)
		}
		st, _, err := ResolveState(r.ID, current, load)
		if err != nil {
			t.Fatalf("%s state: %v", name, err)
		}
		if st != exp.State {
			t.Errorf("%s: state %s, want %s", name, st, exp.State)
		}
	}

	c, err := ParseCitation(want.Citation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := load(c.Record); err != nil {
		t.Fatalf("example citation: %v", err)
	}
}
