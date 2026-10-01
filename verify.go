package core

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Reason is why verification failed. The values match the spec's result
// reasons, plus ReasonMalformed for documents that can't be checked at all.
type Reason string

const (
	ReasonMalformed     Reason = "malformed"
	ReasonBadSignature  Reason = "bad-signature"
	ReasonHashMismatch  Reason = "hash-mismatch"
	ReasonUnknownKey    Reason = "unknown-key"
	ReasonRevokedKey    Reason = "revoked-key"
	ReasonURLMismatch   Reason = "url-mismatch"
	ReasonStaleManifest Reason = "stale-manifest"
)

// VerifyError reports a failed check with its reason.
type VerifyError struct {
	Reason Reason
	Detail string
}

func (e *VerifyError) Error() string { return string(e.Reason) + ": " + e.Detail }

func fail(r Reason, format string, args ...any) error {
	return &VerifyError{Reason: r, Detail: fmt.Sprintf(format, args...)}
}

// ReasonOf returns the Reason of a verification error, or "" if err is not one.
func ReasonOf(err error) Reason {
	var ve *VerifyError
	if errors.As(err, &ve) {
		return ve.Reason
	}
	return ""
}

// ParseKeySet decodes and checks a keys.json document served by publisher.
func ParseKeySet(raw []byte, publisher string) (*KeySet, error) {
	var ks KeySet
	if err := json.Unmarshal(raw, &ks); err != nil {
		return nil, fail(ReasonMalformed, "key set: %v", err)
	}
	if ks.Spec != SpecVersion {
		return nil, fail(ReasonMalformed, "key set: spec %q, want %q", ks.Spec, SpecVersion)
	}
	if !strings.EqualFold(ks.Publisher, publisher) {
		return nil, fail(ReasonURLMismatch, "key set publisher %q served by %q", ks.Publisher, publisher)
	}
	seen := make(map[string]bool, len(ks.Keys))
	for _, k := range ks.Keys {
		if k.ID == "" || seen[k.ID] {
			return nil, fail(ReasonMalformed, "key set: missing or duplicate key id %q", k.ID)
		}
		seen[k.ID] = true
		if k.Alg != AlgEd25519 {
			return nil, fail(ReasonMalformed, "key %q: alg %q, want %q", k.ID, k.Alg, AlgEd25519)
		}
		switch k.Status {
		case KeyActive, KeyRetired, KeyRevoked:
		default:
			return nil, fail(ReasonMalformed, "key %q: unknown status %q", k.ID, k.Status)
		}
		if _, err := decodePublicKey(k.PublicKey); err != nil {
			return nil, fail(ReasonMalformed, "key %q: %v", k.ID, err)
		}
	}
	return &ks, nil
}

// verifyKey returns the public key for keyID if it may verify signatures.
func (ks *KeySet) verifyKey(keyID string) (ed25519.PublicKey, error) {
	for _, k := range ks.Keys {
		if k.ID != keyID {
			continue
		}
		if k.Status == KeyRevoked {
			return nil, fail(ReasonRevokedKey, "key %q is revoked", keyID)
		}
		pub, err := decodePublicKey(k.PublicKey)
		if err != nil {
			return nil, fail(ReasonMalformed, "key %q: %v", keyID, err)
		}
		return pub, nil
	}
	return nil, fail(ReasonUnknownKey, "key %q is not in the key set", keyID)
}

func decodePublicKey(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("public key: %v", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key: %d bytes, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

func checkSignature(ks *KeySet, sig *Signature, b []byte) error {
	pub, err := ks.verifyKey(sig.KeyID)
	if err != nil {
		return err
	}
	v, ok := strings.CutPrefix(sig.Value, sigValuePrefix)
	if !ok {
		return fail(ReasonBadSignature, "signature value lacks %q prefix", sigValuePrefix)
	}
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil || !ed25519.Verify(pub, b, raw) {
		return fail(ReasonBadSignature, "signature does not verify with key %q", sig.KeyID)
	}
	return nil
}

// VerifyRecord checks a record's raw JSON as served by publisher, whose key
// set is ks. It checks structure, that publisher, URL host, and origin agree,
// the ID against the content, and the signature. The ID and signature are
// computed from raw, so unknown fields stay covered.
func VerifyRecord(raw []byte, ks *KeySet, publisher string) (*Record, error) {
	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fail(ReasonMalformed, "record: %v", err)
	}
	if err := r.Validate(); err != nil {
		return nil, fail(ReasonMalformed, "record: %v", err)
	}
	if !strings.EqualFold(ks.Publisher, publisher) {
		return nil, fail(ReasonURLMismatch, "key set is for %q, not %q", ks.Publisher, publisher)
	}
	if !strings.EqualFold(r.Publisher, publisher) {
		return nil, fail(ReasonURLMismatch, "record publisher %q served by %q", r.Publisher, publisher)
	}
	if host := hostOf(r.URL); !strings.EqualFold(host, r.Publisher) {
		return nil, fail(ReasonURLMismatch, "record url host %q is not publisher %q", host, r.Publisher)
	}
	b, err := signingBytes(raw, "id", "sig")
	if err != nil {
		return nil, fail(ReasonMalformed, "record: %v", err)
	}
	if want := RecordIDPrefix + sha256Hex(b); r.ID != want {
		return nil, fail(ReasonHashMismatch, "record id %s does not match its content (%s)", r.ID, want)
	}
	if err := checkSignature(ks, r.Sig, b); err != nil {
		return nil, err
	}
	return &r, nil
}

// VerifyChunk checks one chunk's text against a verified record: the chunk
// must be listed in the record and its text must hash to its ID.
func VerifyChunk(r *Record, c BundleChunk) error {
	if ChunkID(c.Text) != c.ID {
		return fail(ReasonHashMismatch, "chunk %s: text does not match its id", c.ID)
	}
	for _, ref := range r.Chunks {
		if ref.ID == c.ID {
			return nil
		}
	}
	return fail(ReasonHashMismatch, "chunk %s is not in record %s", c.ID, r.ID)
}

// VerifyBundle checks that b holds exactly the chunks of the verified record
// r, in order, each matching its ID.
func VerifyBundle(r *Record, b *Bundle) error {
	if b.Record != r.ID {
		return fail(ReasonHashMismatch, "bundle is for record %s, not %s", b.Record, r.ID)
	}
	if len(b.Chunks) != len(r.Chunks) {
		return fail(ReasonHashMismatch, "bundle has %d chunks, record has %d", len(b.Chunks), len(r.Chunks))
	}
	for i, c := range b.Chunks {
		if c.ID != r.Chunks[i].ID {
			return fail(ReasonHashMismatch, "bundle chunk %d is %s, record says %s", i, c.ID, r.Chunks[i].ID)
		}
		if ChunkID(c.Text) != c.ID {
			return fail(ReasonHashMismatch, "bundle chunk %d: text does not match its id", i)
		}
	}
	return nil
}

// VerifyManifest checks a manifest's raw JSON as served by publisher. If
// lastSeen is not zero, a manifest generated before it is rejected as stale
// (rollback protection).
func VerifyManifest(raw []byte, ks *KeySet, publisher string, lastSeen time.Time) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fail(ReasonMalformed, "manifest: %v", err)
	}
	if err := m.validate(); err != nil {
		return nil, fail(ReasonMalformed, "manifest: %v", err)
	}
	if !strings.EqualFold(ks.Publisher, publisher) {
		return nil, fail(ReasonURLMismatch, "key set is for %q, not %q", ks.Publisher, publisher)
	}
	if !strings.EqualFold(m.Publisher, publisher) {
		return nil, fail(ReasonURLMismatch, "manifest publisher %q served by %q", m.Publisher, publisher)
	}
	for u := range m.Entries {
		if host := hostOf(u); !strings.EqualFold(host, m.Publisher) {
			return nil, fail(ReasonURLMismatch, "manifest entry %q is not on %q", u, m.Publisher)
		}
	}
	b, err := signingBytes(raw, "sig")
	if err != nil {
		return nil, fail(ReasonMalformed, "manifest: %v", err)
	}
	if err := checkSignature(ks, m.Sig, b); err != nil {
		return nil, err
	}
	if !lastSeen.IsZero() && m.GeneratedAt.Before(lastSeen) {
		return nil, fail(ReasonStaleManifest, "manifest generated %s, already saw %s",
			m.GeneratedAt.Format(time.RFC3339), lastSeen.Format(time.RFC3339))
	}
	return &m, nil
}

// hostOf returns the lowercase host of an https URL, or "" if u is not one.
func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" || p.Port() != "" {
		return ""
	}
	return strings.ToLower(p.Hostname())
}

// VerifySignature checks sig over canonical bytes b with the named key from
// ks. Revoked and unknown keys fail.
func (ks *KeySet) VerifySignature(sig *Signature, b []byte) error {
	if sig == nil {
		return fail(ReasonBadSignature, "missing signature")
	}
	return checkSignature(ks, sig, b)
}
