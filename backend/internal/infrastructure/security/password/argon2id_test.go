package password

import "testing"

func TestArgon2IDHasherHashAndVerify(t *testing.T) {
	hasher := NewArgon2IDHasher()
	hash, err := hasher.Hash("super-secure-password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !hasher.Verify(hash, "super-secure-password") {
		t.Fatalf("expected password verification success")
	}
	if hasher.Verify(hash, "wrong-password") {
		t.Fatalf("expected password verification failure")
	}
}
