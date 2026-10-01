package core

import (
	"testing"
	"time"
)

func validRecord() Record {
	return Record{
		Spec:        SpecVersion,
		ID:          RecordIDPrefix + repeat("a", 64),
		Publisher:   testPublisher,
		URL:         "https://" + testPublisher + "/p",
		Title:       "Title",
		PublishedAt: time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC),
		MediaType:   MediaTypeMarkdown,
		Chunks:      []ChunkRef{{ID: ChunkID("text")}},
		Sig:         &Signature{KeyID: "2026a", Value: "ed25519:x"},
	}
}

func TestRecordValidate(t *testing.T) {
	prev := RecordIDPrefix + repeat("b", 64)
	tests := []struct {
		name string
		edit func(*Record)
		ok   bool
	}{
		{"valid", func(*Record) {}, true},
		{"plain text", func(r *Record) { r.MediaType = MediaTypePlain }, true},
		{"revision", func(r *Record) { r.Supersedes, r.Change = prev, ChangeRevision }, true},
		{"correction with note", func(r *Record) { r.Supersedes, r.Change, r.Note = prev, ChangeCorrection, "fixed" }, true},
		{"withdrawal", func(r *Record) { r.Supersedes, r.Change, r.Chunks = prev, ChangeWithdrawal, nil }, true},
		{"reference with record and chunk", func(r *Record) {
			r.References = []Reference{{URL: "https://x.test/", Record: prev, Chunk: ChunkID("t")}}
		}, true},

		{"wrong spec", func(r *Record) { r.Spec = "sourced/0.1" }, false},
		{"no publisher", func(r *Record) { r.Publisher = "" }, false},
		{"http url", func(r *Record) { r.URL = "http://" + testPublisher + "/p" }, false},
		{"url with port", func(r *Record) { r.URL = "https://" + testPublisher + ":8443/p" }, false},
		{"no title", func(r *Record) { r.Title = "" }, false},
		{"no published_at", func(r *Record) { r.PublishedAt = time.Time{} }, false},
		{"bad media type", func(r *Record) { r.MediaType = "text/html" }, false},
		{"no sig", func(r *Record) { r.Sig = nil }, false},
		{"bad id", func(r *Record) { r.ID = "sr:sha256:xyz" }, false},
		{"bad chunk id", func(r *Record) { r.Chunks = []ChunkRef{{ID: "sc:md5:abc"}} }, false},
		{"no chunks", func(r *Record) { r.Chunks = nil }, false},
		{"withdrawal with chunks", func(r *Record) { r.Supersedes, r.Change = prev, ChangeWithdrawal }, false},
		{"change without supersedes", func(r *Record) { r.Change = ChangeRevision }, false},
		{"supersedes without change", func(r *Record) { r.Supersedes = prev }, false},
		{"unknown change", func(r *Record) { r.Supersedes, r.Change = prev, "edit" }, false},
		{"bad supersedes", func(r *Record) { r.Supersedes, r.Change = "nope", ChangeRevision }, false},
		{"retraction without note", func(r *Record) { r.Supersedes, r.Change = prev, ChangeRetraction }, false},
		{"reference without url", func(r *Record) { r.References = []Reference{{Record: prev}} }, false},
		{"reference with bad record", func(r *Record) { r.References = []Reference{{URL: "https://x.test/", Record: "x"}} }, false},
		{"reference with bad chunk", func(r *Record) { r.References = []Reference{{URL: "https://x.test/", Chunk: "x"}} }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord()
			tt.edit(&r)
			err := r.Validate()
			if tt.ok && err != nil {
				t.Fatalf("want valid, got %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("want an error, got none")
			}
		})
	}
}

func TestManifestValidate(t *testing.T) {
	valid := func() Manifest {
		return Manifest{
			Spec:        SpecVersion,
			Publisher:   testPublisher,
			GeneratedAt: time.Date(2026, 9, 28, 10, 5, 0, 0, time.UTC),
			Entries:     map[string]string{"https://" + testPublisher + "/p": RecordIDPrefix + repeat("a", 64)},
			Sig:         &Signature{KeyID: "2026a", Value: "ed25519:x"},
		}
	}
	tests := []struct {
		name string
		edit func(*Manifest)
		ok   bool
	}{
		{"valid", func(*Manifest) {}, true},
		{"sharded root", func(m *Manifest) { m.Entries, m.Shards = nil, "sha256-prefix-2" }, true},
		{"wrong spec", func(m *Manifest) { m.Spec = "x" }, false},
		{"no publisher", func(m *Manifest) { m.Publisher = "" }, false},
		{"no generated_at", func(m *Manifest) { m.GeneratedAt = time.Time{} }, false},
		{"no sig", func(m *Manifest) { m.Sig = nil }, false},
		{"shards and entries", func(m *Manifest) { m.Shards = "sha256-prefix-2" }, false},
		{"bad entry id", func(m *Manifest) { m.Entries = map[string]string{"https://" + testPublisher + "/p": "x"} }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			tt.edit(&m)
			if err := m.validate(); (err == nil) != tt.ok {
				t.Fatalf("validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestVerifyErrorMessage(t *testing.T) {
	err := fail(ReasonHashMismatch, "chunk %d", 3)
	if err.Error() != "hash-mismatch: chunk 3" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if ReasonOf(nil) != "" {
		t.Fatal("ReasonOf(nil) should be empty")
	}
}
