package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	p := DefaultArgon2Params()
	hash, err := HashPassword("correct horse battery staple", p)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected PHC argon2id encoding, got %q", hash)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("expected correct password to verify")
	}

	ok, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatalf("verify wrong: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}

func TestHashPasswordSaltIsRandom(t *testing.T) {
	p := DefaultArgon2Params()
	h1, _ := HashPassword("same", p)
	h2, _ := HashPassword("same", p)
	if h1 == h2 {
		t.Fatal("expected distinct hashes for the same password (random salt)")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "plain", "$argon2id$bad", "$bcrypt$v=1$x$y$z"} {
		if _, err := VerifyPassword("x", bad); err == nil {
			t.Fatalf("expected error for malformed hash %q", bad)
		}
	}
}

func TestGenerateOpaqueToken(t *testing.T) {
	raw, hash, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("expected non-empty raw and hash")
	}
	if raw == hash {
		t.Fatal("raw token must not equal its stored hash")
	}
	if HashOpaqueToken(raw) != hash {
		t.Fatal("HashOpaqueToken must be deterministic and match GenerateOpaqueToken")
	}

	raw2, _, _ := GenerateOpaqueToken()
	if raw == raw2 {
		t.Fatal("expected distinct tokens across calls")
	}
}
