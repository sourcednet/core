package core

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// AlgEd25519 is the only signing algorithm in v1.
const AlgEd25519 = "ed25519"

const sigValuePrefix = "ed25519:"

// GenerateKey creates an Ed25519 key pair. A nil rand uses crypto/rand.
func GenerateKey(rand io.Reader) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand)
}

// NewKey returns an active key set entry for pub.
func NewKey(id string, pub ed25519.PublicKey) Key {
	return Key{
		ID:        id,
		Alg:       AlgEd25519,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
		Status:    KeyActive,
	}
}

// SignRecord sets r's ID and signature. The ID covers everything except
// "id" and "sig", so re-signing with another key keeps the same ID.
func SignRecord(r *Record, keyID string, priv ed25519.PrivateKey) error {
	r.ID, r.Sig = "", nil
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("sign record: %w", err)
	}
	b, err := signingBytes(raw, "id", "sig")
	if err != nil {
		return fmt.Errorf("sign record: %w", err)
	}
	r.ID = RecordIDPrefix + sha256Hex(b)
	r.Sig = sign(keyID, priv, b)
	return nil
}

// SignManifest sets m's signature. Manifests have no ID.
func SignManifest(m *Manifest, keyID string, priv ed25519.PrivateKey) error {
	m.Sig = nil
	raw, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("sign manifest: %w", err)
	}
	b, err := signingBytes(raw, "sig")
	if err != nil {
		return fmt.Errorf("sign manifest: %w", err)
	}
	m.Sig = sign(keyID, priv, b)
	return nil
}

func sign(keyID string, priv ed25519.PrivateKey, b []byte) *Signature {
	return &Signature{
		KeyID: keyID,
		Value: sigValuePrefix + base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b)),
	}
}

// SignBytes signs canonical bytes b with priv, naming keyID.
func SignBytes(keyID string, priv ed25519.PrivateKey, b []byte) *Signature {
	return sign(keyID, priv, b)
}
