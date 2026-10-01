package locations

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/config"
)

// sealVersion is the first byte of every sealed value.
const sealVersion byte = 1

// Sealer encrypts protected values (exact locations, contact details) with
// AES-256-GCM before they reach the database. Each ciphertext is bound to its
// row and field through additional authenticated data, so a ciphertext copied
// into another row or column fails to decrypt.
//
// Format: version(1) | keyIDLen(1) | keyID | nonce(12) | ciphertext+tag
type Sealer struct {
	primary string
	aeads   map[string]cipher.AEAD
}

// NewSealer builds a sealer from configured data keys. The first key is the
// primary key for new ciphertexts.
func NewSealer(keys []config.DataKey) (*Sealer, error) {
	if len(keys) == 0 {
		return nil, errors.New("at least one data encryption key is required")
	}
	s := &Sealer{primary: keys[0].ID, aeads: map[string]cipher.AEAD{}}
	for _, k := range keys {
		block, err := aes.NewCipher(k.Key)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k.ID, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k.ID, err)
		}
		s.aeads[k.ID] = aead
	}
	return s, nil
}

// Seal encrypts plaintext bound to aad. Empty plaintext yields nil.
func (s *Sealer) Seal(plaintext []byte, aad string) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	aead := s.aeads[s.primary]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 2+len(s.primary)+len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, sealVersion, byte(len(s.primary)))
	out = append(out, s.primary...)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plaintext, []byte(aad)), nil
}

// ErrUnsealable is returned when a ciphertext cannot be opened.
var ErrUnsealable = errors.New("protected value cannot be decrypted (unknown key or tampered data)")

// Open decrypts a sealed value. Nil input yields nil.
func (s *Sealer) Open(sealed []byte, aad string) ([]byte, error) {
	if len(sealed) == 0 {
		return nil, nil
	}
	if len(sealed) < 2 || sealed[0] != sealVersion {
		return nil, ErrUnsealable
	}
	idLen := int(sealed[1])
	if len(sealed) < 2+idLen {
		return nil, ErrUnsealable
	}
	keyID := string(sealed[2 : 2+idLen])
	aead, ok := s.aeads[keyID]
	if !ok {
		return nil, ErrUnsealable
	}
	rest := sealed[2+idLen:]
	if len(rest) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrUnsealable
	}
	nonce, ct := rest[:aead.NonceSize()], rest[aead.NonceSize():]
	pt, err := aead.Open(nil, nonce, ct, []byte(aad))
	if err != nil {
		return nil, ErrUnsealable
	}
	return pt, nil
}

// NeedsRotation reports whether a sealed value was produced with a
// non-primary key.
func (s *Sealer) NeedsRotation(sealed []byte) bool {
	if len(sealed) < 2 {
		return false
	}
	idLen := int(sealed[1])
	if len(sealed) < 2+idLen {
		return false
	}
	return string(sealed[2:2+idLen]) != s.primary
}

// AAD builds the additional authenticated data for a protected field.
func AAD(entity, id, field string) string { return entity + ":" + id + ":" + field }
