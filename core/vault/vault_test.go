package vault

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestNewRejectsEmptyKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty root key")
	}
}

func TestNewRejectsWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := New(short); err == nil {
		t.Fatal("expected error for wrong-length root key")
	}
}

func TestNewRejectsInvalidBase64(t *testing.T) {
	if _, err := New("not-valid-base64!!"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	v, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	plaintext := []byte("sk-super-secret-token-12345")
	sealed, err := v.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if string(sealed.Ciphertext) == string(plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}

	opened, err := v.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(opened) != string(plaintext) {
		t.Fatalf("got %q, want %q", opened, plaintext)
	}
}

func TestSealProducesDistinctCiphertextsForSameInput(t *testing.T) {
	v, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	a, err := v.Seal([]byte("same-secret"))
	if err != nil {
		t.Fatalf("Seal a: %v", err)
	}
	b, err := v.Seal([]byte("same-secret"))
	if err != nil {
		t.Fatalf("Seal b: %v", err)
	}

	if string(a.Ciphertext) == string(b.Ciphertext) {
		t.Fatal("sealing the same plaintext twice must not yield the same ciphertext (fresh DEK/nonce each time)")
	}
	if string(a.WrappedDEK) == string(b.WrappedDEK) {
		t.Fatal("each Seal call must generate its own DEK")
	}
}

func TestOpenFailsWithWrongRootKey(t *testing.T) {
	v1, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New v1: %v", err)
	}
	v2, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New v2: %v", err)
	}

	sealed, err := v1.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := v2.Open(sealed); err == nil {
		t.Fatal("expected Open to fail when the root key doesn't match the one that sealed the secret")
	}
}

func TestOpenFailsOnTamperedCiphertext(t *testing.T) {
	v, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sealed, err := v.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	sealed.Ciphertext[0] ^= 0xFF

	if _, err := v.Open(sealed); err == nil {
		t.Fatal("expected Open to fail on tampered ciphertext (GCM auth tag should reject it)")
	}
}
