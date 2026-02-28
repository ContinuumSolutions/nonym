package integrations

import (
	"strings"
	"testing"
)

func TestParseKey_Valid(t *testing.T) {
	hex64 := strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	key, err := ParseKey(hex64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("want 32-byte key, got %d bytes", len(key))
	}
}

func TestParseKey_TooShort(t *testing.T) {
	_, err := ParseKey("deadbeef") // 8 hex chars = 4 bytes
	if err == nil {
		t.Error("expected error for short key, got nil")
	}
}

func TestParseKey_InvalidHex(t *testing.T) {
	_, err := ParseKey(strings.Repeat("zz", 32)) // invalid hex
	if err == nil {
		t.Error("expected error for invalid hex, got nil")
	}
}

func TestParseKey_EmptyString(t *testing.T) {
	_, err := ParseKey("")
	if err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	plaintext := "super-secret-api-key-12345"

	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == plaintext {
		t.Error("ciphertext should differ from plaintext")
	}

	recovered, err := decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if recovered != plaintext {
		t.Errorf("want %q, got %q", plaintext, recovered)
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	key := make([]byte, 32)

	ct, err := encrypt(key, "")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	if ct != "" {
		t.Errorf("encrypting empty string should return empty, got %q", ct)
	}

	pt, err := decrypt(key, "")
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}
	if pt != "" {
		t.Errorf("decrypting empty string should return empty, got %q", pt)
	}
}

func TestEncryptDecrypt_DifferentCiphertextsEachCall(t *testing.T) {
	// AES-GCM uses a random nonce so each encryption should be unique.
	key := make([]byte, 32)
	ct1, _ := encrypt(key, "same-plaintext")
	ct2, _ := encrypt(key, "same-plaintext")
	if ct1 == ct2 {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts (nonce randomness)")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	ct, _ := encrypt(key, "secret")
	// Flip the last byte of the base64 string to simulate tampering.
	tampered := ct[:len(ct)-1] + "X"
	if _, err := decrypt(key, tampered); err == nil {
		t.Error("decrypting tampered ciphertext should fail")
	}
}

func TestMaskKey_Long(t *testing.T) {
	got := maskKey("abcdefgh1234")
	if got != "••••1234" {
		t.Errorf("want ••••1234, got %q", got)
	}
}

func TestMaskKey_Short(t *testing.T) {
	got := maskKey("ab")
	if got != "••••" {
		t.Errorf("want ••••, got %q", got)
	}
}

func TestMaskKey_Empty(t *testing.T) {
	if got := maskKey(""); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}
