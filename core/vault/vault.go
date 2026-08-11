// Package vault implements app-level envelope encryption for tenant
// connector credentials. See docs/architecture/SECURITY.md §3 for the
// design decision this implements.
//
// Each secret is encrypted with a random per-credential data key (DEK);
// the DEK itself is encrypted ("wrapped") with a single root key that
// never touches the database. Losing a Mongo dump alone is not enough to
// recover a usable credential.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const keySize = 32 // AES-256

type Vault struct {
	rootKey []byte
}

// New builds a Vault from a base64-encoded 32-byte root key, as supplied
// via the VAULT_ROOT_KEY env var.
func New(rootKeyB64 string) (*Vault, error) {
	if rootKeyB64 == "" {
		return nil, errors.New("vault: root key is empty")
	}
	key, err := base64.StdEncoding.DecodeString(rootKeyB64)
	if err != nil {
		return nil, fmt.Errorf("vault: decode root key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("vault: root key must be %d bytes, got %d", keySize, len(key))
	}
	return &Vault{rootKey: key}, nil
}

// Sealed is the at-rest representation of an encrypted secret: the
// ciphertext under a fresh DEK, and that DEK wrapped under the root key.
type Sealed struct {
	Ciphertext []byte
	Nonce      []byte
	WrappedDEK []byte
	DEKNonce   []byte
}

// Seal encrypts plaintext under a newly generated DEK, then wraps that DEK
// under the vault's root key.
func (v *Vault) Seal(plaintext []byte) (*Sealed, error) {
	dek := make([]byte, keySize)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("vault: generate dek: %w", err)
	}

	ciphertext, nonce, err := gcmEncrypt(dek, plaintext)
	if err != nil {
		return nil, fmt.Errorf("vault: encrypt secret: %w", err)
	}

	wrappedDEK, dekNonce, err := gcmEncrypt(v.rootKey, dek)
	if err != nil {
		return nil, fmt.Errorf("vault: wrap dek: %w", err)
	}

	return &Sealed{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		WrappedDEK: wrappedDEK,
		DEKNonce:   dekNonce,
	}, nil
}

// Open unwraps the DEK under the root key, then decrypts the ciphertext.
func (v *Vault) Open(s *Sealed) ([]byte, error) {
	dek, err := gcmDecrypt(v.rootKey, s.WrappedDEK, s.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("vault: unwrap dek: %w", err)
	}
	plaintext, err := gcmDecrypt(dek, s.Ciphertext, s.Nonce)
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt secret: %w", err)
	}
	return plaintext, nil
}

func gcmEncrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func gcmDecrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
