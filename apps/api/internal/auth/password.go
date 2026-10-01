package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. These follow the OWASP Password Storage Cheat Sheet
// recommendation (m=19 MiB, t=2, p=1) which keeps memory use moderate for
// small self-hosted devices such as a Raspberry Pi in a field shelter.
// Hashes embed their parameters, so they can be raised later without
// invalidating existing passwords.
type Argon2Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultParams are used for new hashes.
var DefaultParams = Argon2Params{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}

// hashSlots bounds concurrent Argon2id computations to cap memory usage.
var hashSlots = make(chan struct{}, 4)

func acquire(ctx context.Context) error {
	select {
	case hashSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func release() { <-hashSlots }

// HashPassword returns a PHC-formatted Argon2id hash.
func HashPassword(ctx context.Context, password string) (string, error) {
	return hashWith(ctx, password, DefaultParams)
}

func hashWith(ctx context.Context, password string, p Argon2Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	if err := acquire(ctx); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, p.KeyLen)
	release()
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.MemoryKiB, p.Iterations,
		p.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

var errInvalidHash = errors.New("invalid password hash format")

// VerifyPassword checks password against an encoded hash in constant time.
func VerifyPassword(ctx context.Context, password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errInvalidHash
	}
	var p Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return false, errInvalidHash
	}
	if p.MemoryKiB > 1<<20 || p.Iterations > 20 || p.Parallelism == 0 {
		return false, errInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errInvalidHash
	}
	if err := acquire(ctx); err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(want)))
	release()
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is verified against when a username does not exist so response
// timing does not reveal which accounts exist.
var dummyHash = func() string {
	h, err := hashWith(context.Background(), "reliefmesh-timing-equalizer", DefaultParams)
	if err != nil {
		panic(err)
	}
	return h
}()

// Password policy, following NIST SP 800-63B: length over composition rules
// and a blocklist of very common passwords.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

var commonPasswords = map[string]bool{
	"123456789012": true, "password1234": true, "passwordpassword": true, "qwertyuiopas": true,
	"111111111111": true, "000000000000": true, "letmeinletmein": true, "iloveyou1234": true,
	"administrator": true, "adminadminadmin": true, "reliefmesh123": true, "reliefmesh2026": true,
	"reliefmeshadmin": true, "changeme1234": true, "welcome12345": true, "123412341234": true,
	"qwerty123456": true, "1q2w3e4r5t6y": true, "abcdefghijkl": true, "passw0rd1234": true,
}

// ValidatePassword returns a user-facing message when the password does not
// meet the policy, or "" when it is acceptable.
func ValidatePassword(password, username string) string {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLength {
		return fmt.Sprintf("Use at least %d characters. A passphrase of several words works well.", MinPasswordLength)
	}
	if n > MaxPasswordLength {
		return fmt.Sprintf("Use at most %d characters.", MaxPasswordLength)
	}
	lower := strings.ToLower(password)
	if commonPasswords[lower] {
		return "This password is too common. Choose a different one."
	}
	if username != "" && strings.Contains(lower, strings.ToLower(username)) {
		return "The password must not contain the username."
	}
	if strings.Count(password, string([]rune(password)[0])) == n {
		return "The password must not repeat a single character."
	}
	return ""
}

// unambiguous alphabet for temporary passwords (no 0/O, 1/l/I).
const tempAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// GenerateTemporaryPassword returns a random password like
// "k7mq-x2vd-pq9r-hn4t" (about 79 bits of entropy).
func GenerateTemporaryPassword() (string, error) {
	const groups, groupLen = 4, 4
	buf := make([]byte, groups*groupLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, b := range buf {
		if i > 0 && i%groupLen == 0 {
			sb.WriteByte('-')
		}
		// Modulo bias is negligible for a 31-symbol alphabet over a byte
		// and acceptable for single-use temporary credentials.
		sb.WriteByte(tempAlphabet[int(b)%len(tempAlphabet)])
	}
	return sb.String(), nil
}
