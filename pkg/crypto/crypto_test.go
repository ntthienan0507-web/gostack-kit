package crypto

import (
	"encoding/hex"
	"testing"
)

func testKey() string {
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

func TestEncryptDecrypt(t *testing.T) {
	enc, err := NewEncryptor(testKey())
	if err != nil { t.Fatalf("NewEncryptor: %v", err) }
	encrypted, err := enc.Encrypt("john@example.com")
	if err != nil { t.Fatalf("Encrypt: %v", err) }
	if encrypted == "john@example.com" { t.Error("should differ") }
	decrypted, err := enc.Decrypt(encrypted)
	if err != nil { t.Fatalf("Decrypt: %v", err) }
	if decrypted != "john@example.com" { t.Errorf("got %q", decrypted) }
}

func TestEncrypt_DifferentOutput(t *testing.T) {
	enc, _ := NewEncryptor(testKey())
	a, _ := enc.Encrypt("same")
	b, _ := enc.Encrypt("same")
	if a == b { t.Error("random nonce should differ") }
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, _ := NewEncryptor(testKey())
	enc2, _ := NewEncryptor(hex.EncodeToString(make([]byte, 32)))
	encrypted, _ := enc1.Encrypt("secret")
	_, err := enc2.Decrypt(encrypted)
	if err == nil { t.Error("wrong key should fail") }
}

func TestNewEncryptor_Invalid(t *testing.T) {
	for _, k := range []string{"zzz", "0123456789abcdef", ""} {
		if _, err := NewEncryptor(k); err == nil { t.Errorf("key %q should fail", k) }
	}
}

func TestHashPassword(t *testing.T) {
	hash, _ := HashPassword("pass")
	if !CheckPassword("pass", hash) { t.Error("should match") }
	if CheckPassword("wrong", hash) { t.Error("should not match") }
}

func TestSHA256Hex(t *testing.T) {
	if SHA256Hex("hello") != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Error("wrong hash")
	}
}

func TestRandomHex(t *testing.T) {
	a, _ := RandomHex(16)
	b, _ := RandomHex(16)
	if len(a) != 32 { t.Error("wrong length") }
	if a == b { t.Error("should differ") }
}
