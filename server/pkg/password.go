package pkg

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2Params holds the cost parameters for Argon2id.
// These are stored inside the encoded hash string so they are self-describing —
// you can change them for new passwords without breaking existing ones.
type argon2Params struct {
	memory      uint32 // KiB of memory to use
	iterations  uint32 // number of passes over memory
	parallelism uint8  // number of threads
	saltLen     uint32 // bytes of random salt
	keyLen      uint32 // bytes of derived key
}

// defaultParams are OWASP-recommended minimums for Argon2id (2024).
// memory=64MiB, iterations=3, parallelism=4 gives ~300ms on a modern server.
// Increase memory/iterations for higher-security deployments.
var defaultParams = argon2Params{
	memory:      64 * 1024, // 64 MiB
	iterations:  3,
	parallelism: 4,
	saltLen:     16,
	keyLen:      32,
}

// HashPassword derives an Argon2id hash from plain and returns it in PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<base64-salt>$<base64-hash>
//
// The returned string is safe to store directly in the database.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, defaultParams.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(plain),
		salt,
		defaultParams.iterations,
		defaultParams.memory,
		defaultParams.parallelism,
		defaultParams.keyLen,
	)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		defaultParams.memory,
		defaultParams.iterations,
		defaultParams.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// ComparePassword returns nil if plain matches the stored Argon2id hash,
// or an error if the hash is malformed or the password does not match.
// The comparison is constant-time to prevent timing attacks.
func ComparePassword(encoded, plain string) error {
	p, salt, hash, err := decodeHash(encoded)
	if err != nil {
		return fmt.Errorf("decoding hash: %w", err)
	}

	other := argon2.IDKey([]byte(plain), salt, p.iterations, p.memory, p.parallelism, p.keyLen)

	if subtle.ConstantTimeCompare(hash, other) != 1 {
		return errors.New("password does not match")
	}
	return nil
}

// decodeHash parses a PHC-format Argon2id string back into its components.
func decodeHash(encoded string) (p argon2Params, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=65536,t=3,p=4", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return p, nil, nil, errors.New("invalid hash format: wrong number of segments")
	}

	if parts[1] != "argon2id" {
		return p, nil, nil, fmt.Errorf("unsupported algorithm: %s", parts[1])
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, fmt.Errorf("parsing version: %w", err)
	}
	if version != argon2.Version {
		return p, nil, nil, fmt.Errorf("incompatible argon2 version: %d", version)
	}

	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return p, nil, nil, fmt.Errorf("parsing params: %w", err)
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, fmt.Errorf("decoding salt: %w", err)
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, fmt.Errorf("decoding hash: %w", err)
	}

	p.keyLen = uint32(len(hash))
	return p, salt, hash, nil
}
