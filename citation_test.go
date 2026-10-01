package core

import "testing"

func TestCitationRoundTrip(t *testing.T) {
	c := Citation{
		Publisher: testPublisher,
		Record:    RecordIDPrefix + repeat("9f", 32),
		Chunk:     ChunkIDPrefix + repeat("7c", 32),
	}
	s := c.String()
	want := "https://example-library.test/.well-known/sourced/records/" + repeat("9f", 32) + ".json#chunk=" + repeat("7c", 32)
	if s != want {
		t.Fatalf("String() = %s\nwant       %s", s, want)
	}
	got, err := ParseCitation(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != c {
		t.Fatalf("ParseCitation = %+v, want %+v", got, c)
	}
	if u, _ := RecordURL(c.Publisher, c.Record); u+"#chunk="+repeat("7c", 32) != s {
		t.Fatalf("RecordURL %s does not match citation %s", u, s)
	}
}

func TestParseCitationRejects(t *testing.T) {
	h := repeat("ab", 32)
	for name, s := range map[string]string{
		"http":           "http://example.test/.well-known/sourced/records/" + h + ".json#chunk=" + h,
		"port":           "https://example.test:8443/.well-known/sourced/records/" + h + ".json#chunk=" + h,
		"wrong path":     "https://example.test/records/" + h + ".json#chunk=" + h,
		"no fragment":    "https://example.test/.well-known/sourced/records/" + h + ".json",
		"short record":   "https://example.test/.well-known/sourced/records/abc.json#chunk=" + h,
		"uppercase hex":  "https://example.test/.well-known/sourced/records/" + repeat("AB", 32) + ".json#chunk=" + h,
		"wrong fragment": "https://example.test/.well-known/sourced/records/" + h + ".json#passage=" + h,
	} {
		if _, err := ParseCitation(s); err == nil {
			t.Errorf("%s: ParseCitation(%q) succeeded, want error", name, s)
		}
	}
}

func TestIDHex(t *testing.T) {
	if _, err := IDHex(ChunkID("x"), ChunkIDPrefix); err != nil {
		t.Fatal(err)
	}
	if _, err := IDHex(ChunkID("x"), RecordIDPrefix); err == nil {
		t.Fatal("chunk id accepted as record id")
	}
	if _, err := RecordURL(testPublisher, "sr:sha256:nothex"); err == nil {
		t.Fatal("RecordURL accepted a bad id")
	}
}
