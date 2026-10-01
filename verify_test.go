package core

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSignAndVerifyRecord(t *testing.T) {
	ts := newTestSigner(t)
	chunks := testChunks(t, "Keep books away from direct sunlight.", "Store them upright on the shelf.")
	r, raw := ts.signedRecord(t, "2026a", testMeta("/guides/book-care"), chunks)

	got, err := VerifyRecord(raw, ts.keys, testPublisher)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID || !strings.HasPrefix(got.ID, RecordIDPrefix) {
		t.Fatalf("verified id %q, signed id %q", got.ID, r.ID)
	}
	if len(got.Chunks) != 2 || got.Chunks[0].ID != chunks[0].ID() {
		t.Fatalf("chunks not preserved: %+v", got.Chunks)
	}
}

func TestRecordIDDoesNotDependOnKey(t *testing.T) {
	ts := newTestSigner(t)
	chunks := testChunks(t, "Same content, two keys.")
	a, rawA := ts.signedRecord(t, "2026a", testMeta("/p"), chunks)
	b, rawB := ts.signedRecord(t, "2025a", testMeta("/p"), chunks)

	if a.ID != b.ID {
		t.Fatalf("ids differ across keys: %s vs %s", a.ID, b.ID)
	}
	if a.Sig.Value == b.Sig.Value {
		t.Fatal("signatures should differ across keys")
	}
	for _, raw := range [][]byte{rawA, rawB} { // the retired key still verifies
		if _, err := VerifyRecord(raw, ts.keys, testPublisher); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVerifyRecordIgnoresFormatting(t *testing.T) {
	ts := newTestSigner(t)
	_, raw := ts.signedRecord(t, "2026a", testMeta("/p"), testChunks(t, "Formatting must not matter."))
	compact := mutate(t, raw, func(map[string]any) {}) // compact, keys reordered
	if _, err := VerifyRecord(compact, ts.keys, testPublisher); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRecordForwardCompatible(t *testing.T) {
	ts := newTestSigner(t)
	r := NewRecord(testMeta("/p"), testChunks(t, "A future version adds a field."))
	base, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	// Sign a record carrying a member this version doesn't know about.
	future := mutate(t, base, func(o map[string]any) { o["future_field"] = map[string]any{"x": 1} })
	b, err := signingBytes(future, "id", "sig")
	if err != nil {
		t.Fatal(err)
	}
	priv := ts.privs["2026a"]
	future = mutate(t, future, func(o map[string]any) {
		o["id"] = RecordIDPrefix + sha256Hex(b)
		o["sig"] = map[string]any{"key_id": "2026a", "value": sigValuePrefix + base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b))}
	})

	if _, err := VerifyRecord(future, ts.keys, testPublisher); err != nil {
		t.Fatalf("record with an unknown field should verify: %v", err)
	}
	// The unknown field is covered: dropping it breaks the ID.
	stripped := mutate(t, future, func(o map[string]any) { delete(o, "future_field") })
	_, err = VerifyRecord(stripped, ts.keys, testPublisher)
	wantReason(t, err, ReasonHashMismatch)
}

func TestVerifyRecordFailures(t *testing.T) {
	ts := newTestSigner(t)
	chunks := testChunks(t, "Original text.")
	_, raw := ts.signedRecord(t, "2026a", testMeta("/p"), chunks)
	_, rawRevoked := ts.signedRecord(t, "2024x", testMeta("/p"), chunks)
	offHost := testMeta("/p")
	offHost.URL = "https://elsewhere.test/p"
	_, rawOffHost := ts.signedRecord(t, "2026a", offHost, chunks)
	_, otherPriv := testKey("not the publisher")

	tests := []struct {
		name      string
		raw       []byte
		publisher string
		want      Reason
	}{
		{"not json", []byte("{"), testPublisher, ReasonMalformed},
		{"missing title", mutate(t, raw, func(o map[string]any) { delete(o, "title") }), testPublisher, ReasonMalformed},
		{"correction without note", mutate(t, raw, func(o map[string]any) {
			o["supersedes"] = RecordIDPrefix + repeat("a", 64)
			o["change"] = "correction"
		}), testPublisher, ReasonMalformed},
		{"tampered title", mutate(t, raw, func(o map[string]any) { o["title"] = "Something else" }), testPublisher, ReasonHashMismatch},
		{"tampered chunk list", mutate(t, raw, func(o map[string]any) {
			o["chunks"] = []any{map[string]any{"id": ChunkID("other text")}}
		}), testPublisher, ReasonHashMismatch},
		{"signature by another key", mutate(t, raw, func(o map[string]any) {
			o["sig"].(map[string]any)["value"] = sigValuePrefix + base64.StdEncoding.EncodeToString(ed25519.Sign(otherPriv, []byte("x")))
		}), testPublisher, ReasonBadSignature},
		{"garbage signature", mutate(t, raw, func(o map[string]any) {
			o["sig"].(map[string]any)["value"] = "ed25519:not base64!"
		}), testPublisher, ReasonBadSignature},
		{"signature without prefix", mutate(t, raw, func(o map[string]any) {
			o["sig"].(map[string]any)["value"] = "rsa:AAAA"
		}), testPublisher, ReasonBadSignature},
		{"unknown key", mutate(t, raw, func(o map[string]any) {
			o["sig"].(map[string]any)["key_id"] = "nope"
		}), testPublisher, ReasonUnknownKey},
		{"revoked key", rawRevoked, testPublisher, ReasonRevokedKey},
		{"served by another domain", raw, "evil.test", ReasonURLMismatch},
		{"url on another host", rawOffHost, testPublisher, ReasonURLMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyRecord(tt.raw, ts.keys, tt.publisher)
			wantReason(t, err, tt.want)
		})
	}
}

func TestVerifyBundle(t *testing.T) {
	ts := newTestSigner(t)
	chunks := testChunks(t, "First passage.", "Second passage.")
	r, _ := ts.signedRecord(t, "2026a", testMeta("/p"), chunks)
	good, err := NewBundle(r, chunks)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyBundle(r, good); err != nil {
		t.Fatal(err)
	}

	clone := func(f func(*Bundle)) *Bundle {
		b := &Bundle{Record: good.Record, Chunks: append([]BundleChunk(nil), good.Chunks...)}
		f(b)
		return b
	}
	tests := map[string]*Bundle{
		"tampered text": clone(func(b *Bundle) { b.Chunks[0].Text = "Tampered passage." }),
		"reordered":     clone(func(b *Bundle) { b.Chunks[0], b.Chunks[1] = b.Chunks[1], b.Chunks[0] }),
		"missing chunk": clone(func(b *Bundle) { b.Chunks = b.Chunks[:1] }),
		"wrong record":  clone(func(b *Bundle) { b.Record = RecordIDPrefix + repeat("0", 64) }),
	}
	for name, b := range tests {
		t.Run(name, func(t *testing.T) { wantReason(t, VerifyBundle(r, b), ReasonHashMismatch) })
	}
}

func TestVerifyChunk(t *testing.T) {
	ts := newTestSigner(t)
	chunks := testChunks(t, "Listed passage.")
	r, _ := ts.signedRecord(t, "2026a", testMeta("/p"), chunks)

	if err := VerifyChunk(r, BundleChunk{ID: chunks[0].ID(), Text: chunks[0].Text}); err != nil {
		t.Fatal(err)
	}
	wantReason(t, VerifyChunk(r, BundleChunk{ID: chunks[0].ID(), Text: "Changed."}), ReasonHashMismatch)
	wantReason(t, VerifyChunk(r, BundleChunk{ID: ChunkID("Unlisted."), Text: "Unlisted."}), ReasonHashMismatch)
}

func TestVerifyManifest(t *testing.T) {
	ts := newTestSigner(t)
	generated := time.Date(2026, 9, 28, 10, 5, 0, 0, time.UTC)
	m := &Manifest{
		Spec:        SpecVersion,
		Publisher:   testPublisher,
		GeneratedAt: generated,
		Entries:     map[string]string{"https://" + testPublisher + "/p": RecordIDPrefix + repeat("b", 64)},
	}
	if err := SignManifest(m, "2026a", ts.privs["2026a"]); err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeJSON(m)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := VerifyManifest(raw, ts.keys, testPublisher, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyManifest(raw, ts.keys, testPublisher, generated); err != nil {
		t.Fatalf("same generated_at must not count as stale: %v", err)
	}
	_, err = VerifyManifest(raw, ts.keys, testPublisher, generated.Add(time.Minute))
	wantReason(t, err, ReasonStaleManifest)

	tampered := mutate(t, raw, func(o map[string]any) {
		o["entries"].(map[string]any)["https://"+testPublisher+"/p"] = RecordIDPrefix + repeat("c", 64)
	})
	_, err = VerifyManifest(tampered, ts.keys, testPublisher, time.Time{})
	wantReason(t, err, ReasonBadSignature)

	offHost := mutate(t, raw, func(o map[string]any) {
		o["entries"] = map[string]any{"https://elsewhere.test/p": RecordIDPrefix + repeat("b", 64)}
	})
	_, err = VerifyManifest(offHost, ts.keys, testPublisher, time.Time{})
	wantReason(t, err, ReasonURLMismatch)
}

func TestParseKeySet(t *testing.T) {
	ts := newTestSigner(t)
	raw, err := EncodeJSON(ts.keys)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseKeySet(raw, testPublisher); err != nil {
		t.Fatal(err)
	}
	_, err = ParseKeySet(raw, "evil.test")
	wantReason(t, err, ReasonURLMismatch)

	bad := mutate(t, raw, func(o map[string]any) {
		o["keys"].([]any)[0].(map[string]any)["public_key"] = "too short"
	})
	_, err = ParseKeySet(bad, testPublisher)
	wantReason(t, err, ReasonMalformed)

	dup := mutate(t, raw, func(o map[string]any) {
		keys := o["keys"].([]any)
		o["keys"] = append(keys, keys[0])
	})
	_, err = ParseKeySet(dup, testPublisher)
	wantReason(t, err, ReasonMalformed)
}
