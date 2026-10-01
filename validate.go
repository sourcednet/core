package core

import (
	"errors"
	"fmt"
)

// Validate checks a record's structure against the spec, without checking
// its ID or signature.
func (r *Record) Validate() error {
	switch {
	case r.Spec != SpecVersion:
		return fmt.Errorf("spec %q, want %q", r.Spec, SpecVersion)
	case r.Publisher == "":
		return errors.New("missing publisher")
	case hostOf(r.URL) == "":
		return fmt.Errorf("url %q is not an https URL on a domain", r.URL)
	case r.Title == "":
		return errors.New("missing title")
	case r.PublishedAt.IsZero():
		return errors.New("missing published_at")
	case r.MediaType != MediaTypeMarkdown && r.MediaType != MediaTypePlain:
		return fmt.Errorf("media_type %q, want %q or %q", r.MediaType, MediaTypeMarkdown, MediaTypePlain)
	case r.Sig == nil || r.Sig.KeyID == "" || r.Sig.Value == "":
		return errors.New("missing sig")
	}
	if _, err := IDHex(r.ID, RecordIDPrefix); err != nil {
		return err
	}
	for i, c := range r.Chunks {
		if _, err := IDHex(c.ID, ChunkIDPrefix); err != nil {
			return fmt.Errorf("chunk %d: %w", i, err)
		}
	}
	if r.Change == ChangeWithdrawal {
		if len(r.Chunks) != 0 {
			return errors.New("a withdrawal must have no chunks")
		}
	} else if len(r.Chunks) == 0 {
		return errors.New("missing chunks")
	}
	if r.Supersedes == "" {
		if r.Change != "" {
			return fmt.Errorf("change %q without supersedes", r.Change)
		}
	} else {
		if _, err := IDHex(r.Supersedes, RecordIDPrefix); err != nil {
			return fmt.Errorf("supersedes: %w", err)
		}
		switch r.Change {
		case ChangeRevision, ChangeWithdrawal:
		case ChangeCorrection, ChangeRetraction:
			if r.Note == "" {
				return fmt.Errorf("a %s needs a note", r.Change)
			}
		default:
			return fmt.Errorf("supersedes needs a change of revision, correction, retraction, or withdrawal, got %q", r.Change)
		}
	}
	for i, ref := range r.References {
		if ref.URL == "" {
			return fmt.Errorf("reference %d: missing url", i)
		}
		if ref.Record != "" {
			if _, err := IDHex(ref.Record, RecordIDPrefix); err != nil {
				return fmt.Errorf("reference %d: %w", i, err)
			}
		}
		if ref.Chunk != "" {
			if _, err := IDHex(ref.Chunk, ChunkIDPrefix); err != nil {
				return fmt.Errorf("reference %d: %w", i, err)
			}
		}
	}
	return nil
}

func (m *Manifest) validate() error {
	switch {
	case m.Spec != SpecVersion:
		return fmt.Errorf("spec %q, want %q", m.Spec, SpecVersion)
	case m.Publisher == "":
		return errors.New("missing publisher")
	case m.GeneratedAt.IsZero():
		return errors.New("missing generated_at")
	case m.Sig == nil || m.Sig.KeyID == "" || m.Sig.Value == "":
		return errors.New("missing sig")
	case m.Shards != "" && len(m.Entries) > 0:
		return errors.New("a sharded root manifest must not have entries")
	}
	for u, id := range m.Entries {
		if _, err := IDHex(id, RecordIDPrefix); err != nil {
			return fmt.Errorf("entry %q: %w", u, err)
		}
	}
	return nil
}
