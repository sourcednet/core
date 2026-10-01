package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// PageMeta is what a publisher knows about a page besides its text.
type PageMeta struct {
	Publisher   string
	URL         string
	Title       string
	PublishedAt time.Time
	Language    string
	MediaType   string // defaults to text/markdown
	References  []Reference
	License     string
}

// NewRecord builds an unsigned first-version record for a page's chunks.
func NewRecord(meta PageMeta, chunks []Chunk) *Record {
	mt := meta.MediaType
	if mt == "" {
		mt = MediaTypeMarkdown
	}
	refs := make([]ChunkRef, 0, len(chunks))
	for _, c := range chunks {
		refs = append(refs, ChunkRef{ID: c.ID(), Section: c.Section})
	}
	return &Record{
		Spec:        SpecVersion,
		Publisher:   strings.ToLower(meta.Publisher),
		URL:         meta.URL,
		Title:       meta.Title,
		PublishedAt: meta.PublishedAt.UTC().Truncate(time.Second),
		Language:    meta.Language,
		MediaType:   mt,
		Chunks:      refs,
		References:  meta.References,
		License:     meta.License,
	}
}

// NewSuccessor builds an unsigned record that supersedes prev with the given
// change. A withdrawal always has no chunks.
func NewSuccessor(prev *Record, change Change, note string, meta PageMeta, chunks []Chunk) (*Record, error) {
	if prev.ID == "" {
		return nil, errors.New("successor of an unsigned record")
	}
	if change == ChangeWithdrawal {
		chunks = nil
	}
	r := NewRecord(meta, chunks)
	r.Supersedes, r.Change, r.Note = prev.ID, change, note
	return r, nil
}

// NewBundle builds the bundle for a signed record from the chunks it was built from.
func NewBundle(r *Record, chunks []Chunk) (*Bundle, error) {
	if r.ID == "" {
		return nil, errors.New("bundle for an unsigned record")
	}
	if len(chunks) != len(r.Chunks) {
		return nil, fmt.Errorf("bundle: %d chunks for a record with %d", len(chunks), len(r.Chunks))
	}
	b := &Bundle{Record: r.ID, Chunks: make([]BundleChunk, len(chunks))}
	for i, c := range chunks {
		id := c.ID()
		if id != r.Chunks[i].ID {
			return nil, fmt.Errorf("bundle: chunk %d is %s, record says %s", i, id, r.Chunks[i].ID)
		}
		b.Chunks[i] = BundleChunk{ID: id, Text: c.Text}
	}
	return b, nil
}
