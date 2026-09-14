package auth

import "golang.org/x/crypto/bcrypt"

// MinPasswordLength is enforced for the bootstrap admin password. There is
// no in-app password-change flow, so this is the only chance to reject a
// trivially guessable credential.
const MinPasswordLength = 12

// DummyPasswordHash is compared against when a login names an unknown user,
// so the response takes the same time as a real bcrypt check and doesn't
// reveal whether the username exists. It's a hash of a random throwaway
// value, generated once at startup.
var DummyPasswordHash = func() string {
	hash, err := bcrypt.GenerateFromPassword([]byte("nonexistent-user-placeholder"), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}()

// HashPassword bcrypt-hashes a plaintext password for storage.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword reports whether plaintext matches the given bcrypt hash.
func VerifyPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}
