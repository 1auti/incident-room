package auth

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// The decoy hash equalizes login cost for unknown emails, so it must be a valid
// bcrypt hash with the same cost as real password hashes. Timing itself is not asserted.
func TestDummyPasswordHash(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy hash is not valid bcrypt: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Errorf("cost = %d, want %d (same as stored hashes)", cost, bcrypt.DefaultCost)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte("whatever")); !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		t.Errorf("compare err = %v, want mismatch", err)
	}
}
