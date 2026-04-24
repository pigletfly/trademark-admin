// Package auth provides authentication primitives and middleware.
//
// This file exposes password hashing/verification using argon2id. Parameters
// match the spec (§12): memory=64MiB, iterations=3, parallelism=2.
package auth

import "github.com/alexedwards/argon2id"

var passwordParams = &argon2id.Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword returns a PHC-encoded argon2id hash of the password. The salt is
// generated per call, so two hashes of the same password differ.
func HashPassword(plain string) (string, error) {
	return argon2id.CreateHash(plain, passwordParams)
}

// VerifyPassword reports whether plain matches the stored PHC-encoded hash.
// It never returns a nil-error-with-ok=false panic: a mismatch is a clean
// (false, nil).
func VerifyPassword(plain, encoded string) (bool, error) {
	return argon2id.ComparePasswordAndHash(plain, encoded)
}
