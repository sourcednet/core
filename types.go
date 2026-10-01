package core

import (
	"bytes"
	"encoding/json"
	"time"
)

// SpecVersion is the value of the "spec" field in every v1 document.
const SpecVersion = "sourced/1"

// Media types a record's chunks may use.
const (
	MediaTypeMarkdown = "text/markdown"
	MediaTypePlain    = "text/plain"
)

// Change is the kind of change a successor record declares.
type Change string

const (
	ChangeRevision   Change = "revision"
	ChangeCorrection Change = "correction"
	ChangeRetraction Change = "retraction"
	ChangeWithdrawal Change = "withdrawal"
)

// KeyStatus is the lifecycle state of a publisher key.
type KeyStatus string

const (
	KeyActive  KeyStatus = "active"
	KeyRetired KeyStatus = "retired"
	KeyRevoked KeyStatus = "revoked"
)

// Signature is the "sig" member of records and manifests.
type Signature struct {
	KeyID string `json:"key_id"`
	Value string `json:"value"`
}

// ChunkRef is one entry of a record's ordered chunk list.
type ChunkRef struct {
	ID      string   `json:"id"`
	Section []string `json:"section,omitempty"`
}

// Reference is a URL a page cites, optionally pinned to a record and chunk.
type Reference struct {
	URL    string `json:"url"`
	Record string `json:"record,omitempty"`
	Chunk  string `json:"chunk,omitempty"`
}

// Record is a signed, immutable statement that a domain published one
// version of one page, made of an ordered list of chunks.
type Record struct {
	Spec        string      `json:"spec"`
	ID          string      `json:"id,omitempty"`
	Publisher   string      `json:"publisher"`
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	PublishedAt time.Time   `json:"published_at"`
	Language    string      `json:"language,omitempty"`
	MediaType   string      `json:"media_type"`
	Chunks      []ChunkRef  `json:"chunks"`
	References  []Reference `json:"references,omitempty"`
	Supersedes  string      `json:"supersedes,omitempty"`
	Change      Change      `json:"change,omitempty"`
	Note        string      `json:"note,omitempty"`
	License     string      `json:"license,omitempty"`
	Sig         *Signature  `json:"sig,omitempty"`
}

// BundleChunk is one chunk's text inside a bundle.
type BundleChunk struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Bundle holds the text of every chunk in a record, in order.
// Bundles are not signed: each chunk is checked against the signed record.
type Bundle struct {
	Record string        `json:"record"`
	Chunks []BundleChunk `json:"chunks"`
}

// Manifest maps a publisher's URLs to their current record IDs.
type Manifest struct {
	Spec        string            `json:"spec"`
	Publisher   string            `json:"publisher"`
	GeneratedAt time.Time         `json:"generated_at"`
	Entries     map[string]string `json:"entries,omitempty"`
	Shards      string            `json:"shards,omitempty"`
	Sig         *Signature        `json:"sig,omitempty"`
}

// Key is one public key in a publisher's key set.
type Key struct {
	ID        string    `json:"id"`
	Alg       string    `json:"alg"`
	PublicKey string    `json:"public_key"`
	Status    KeyStatus `json:"status"`
}

// KeySet is the content of a publisher's keys.json.
type KeySet struct {
	Spec      string `json:"spec"`
	Publisher string `json:"publisher"`
	Keys      []Key  `json:"keys"`
}

// EncodeJSON renders v the way sourced files are written: indented with two
// spaces, without HTML escaping, and ending in a newline. Formatting never
// affects verification, which always works from canonical bytes.
func EncodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ResolverInfo is the content of a resolver's /.well-known/sourced/resolver.json.
type ResolverInfo struct {
	Spec            string `json:"spec"`
	Resolver        string `json:"resolver"`
	Operator        string `json:"operator,omitempty"`
	API             string `json:"api"`
	MCP             string `json:"mcp,omitempty"`
	Keys            []Key  `json:"keys"`
	RetentionPolicy string `json:"retention_policy,omitempty"`
}

// KeySet returns the resolver's keys as a key set, for checking its signed answers.
func (ri *ResolverInfo) KeySet() *KeySet {
	return &KeySet{Spec: SpecVersion, Publisher: ri.Resolver, Keys: ri.Keys}
}

// ResolverInfoURL returns the URL of a resolver's resolver.json.
func ResolverInfoURL(resolver string) string { return wellKnownBase(resolver) + "resolver.json" }
