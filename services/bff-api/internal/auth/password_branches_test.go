package auth

import (
	"strings"
	"testing"
)

// VerifyPassword has several distinct error branches past the coarse
// "len != 6 / not argon2id" guard already covered by TestVerifyPasswordRejectsMalformed.
// Here we corrupt one PHC field at a time on an otherwise-valid hash.
func TestVerifyPassword_FieldLevelCorruption(t *testing.T) {
	good, err := HashPassword("pw", DefaultArgon2Params())
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	parts := strings.Split(good, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", <salt>, <hash>]
	if len(parts) != 6 {
		t.Fatalf("unexpected PHC layout: %v", parts)
	}

	rebuild := func(mutate func(p []string)) string {
		cp := append([]string(nil), parts...)
		mutate(cp)
		return strings.Join(cp, "$")
	}

	cases := []struct {
		name    string
		encoded string
	}{
		{"unparseable version", rebuild(func(p []string) { p[2] = "v=notanumber" })},
		{"incompatible version", rebuild(func(p []string) { p[2] = "v=1" })},
		{"unparseable params", rebuild(func(p []string) { p[3] = "m=x,t=y,p=z" })},
		{"undecodable salt", rebuild(func(p []string) { p[4] = "!!not-base64!!" })},
		{"undecodable hash", rebuild(func(p []string) { p[5] = "!!not-base64!!" })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := VerifyPassword("pw", tc.encoded)
			if err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
			if ok {
				t.Fatalf("a malformed hash must never verify true (%s)", tc.name)
			}
		})
	}
}

// A correctly-formed hash with the wrong key bytes must verify false WITHOUT an
// error (the constant-time-compare branch, distinct from the malformed branches).
func TestVerifyPassword_WrongPasswordIsFalseNotError(t *testing.T) {
	good, err := HashPassword("right", DefaultArgon2Params())
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("wrong", good)
	if err != nil {
		t.Fatalf("a well-formed hash must not error on a wrong password: %v", err)
	}
	if ok {
		t.Fatal("wrong password must verify false")
	}
}

// HashPassword must honour custom (non-default) cost parameters and still
// round-trip through VerifyPassword.
func TestHashPassword_CustomParamsRoundTrip(t *testing.T) {
	p := Argon2Params{MemoryKiB: 8192, Iterations: 1, Parallelism: 2, SaltLen: 8, KeyLen: 16}
	h, err := HashPassword("secret", p)
	if err != nil {
		t.Fatalf("hash with custom params: %v", err)
	}
	if !strings.Contains(h, "m=8192,t=1,p=2") {
		t.Fatalf("encoded params not reflected in hash: %q", h)
	}
	ok, err := VerifyPassword("secret", h)
	if err != nil || !ok {
		t.Fatalf("custom-param hash must round-trip: ok=%v err=%v", ok, err)
	}
}
