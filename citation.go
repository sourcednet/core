package core

import (
	"fmt"
	"net/url"
	"strings"
)

// WellKnownPath is where every publisher file lives.
const WellKnownPath = "/.well-known/sourced/"

func wellKnownBase(publisher string) string { return "https://" + publisher + WellKnownPath }

// KeysURL returns the URL of a publisher's keys.json.
func KeysURL(publisher string) string { return wellKnownBase(publisher) + "keys.json" }

// ManifestURL returns the URL of a publisher's root manifest.
func ManifestURL(publisher string) string { return wellKnownBase(publisher) + "manifest.json" }

// RecordURL returns the URL of a record file.
func RecordURL(publisher, recordID string) (string, error) {
	h, err := IDHex(recordID, RecordIDPrefix)
	if err != nil {
		return "", err
	}
	return wellKnownBase(publisher) + "records/" + h + ".json", nil
}

// BundleURL returns the URL of a record's bundle.
func BundleURL(publisher, recordID string) (string, error) {
	h, err := IDHex(recordID, RecordIDPrefix)
	if err != nil {
		return "", err
	}
	return wellKnownBase(publisher) + "bundles/" + h + ".json", nil
}

// Citation points to one chunk in one record.
type Citation struct {
	Publisher string
	Record    string
	Chunk     string
}

// String renders the citation as the record's URL with the chunk as a
// fragment: https://<publisher>/.well-known/sourced/records/<hex>.json#chunk=<hex>
func (c Citation) String() string {
	rh := strings.TrimPrefix(c.Record, RecordIDPrefix)
	ch := strings.TrimPrefix(c.Chunk, ChunkIDPrefix)
	return wellKnownBase(c.Publisher) + "records/" + rh + ".json#chunk=" + ch
}

// ParseCitation parses the form produced by Citation.String.
func ParseCitation(s string) (Citation, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Citation{}, fmt.Errorf("citation: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.Port() != "" {
		return Citation{}, fmt.Errorf("citation %q: want an https URL on a domain", s)
	}
	name, ok := strings.CutPrefix(u.Path, WellKnownPath+"records/")
	if !ok {
		return Citation{}, fmt.Errorf("citation %q: path is not under %srecords/", s, WellKnownPath)
	}
	rh, ok := strings.CutSuffix(name, ".json")
	if !ok || !isHexDigest(rh) {
		return Citation{}, fmt.Errorf("citation %q: want records/<64 hex>.json", s)
	}
	ch, ok := strings.CutPrefix(u.Fragment, "chunk=")
	if !ok || !isHexDigest(ch) {
		return Citation{}, fmt.Errorf("citation %q: want fragment chunk=<64 hex>", s)
	}
	return Citation{
		Publisher: strings.ToLower(u.Hostname()),
		Record:    RecordIDPrefix + rh,
		Chunk:     ChunkIDPrefix + ch,
	}, nil
}
