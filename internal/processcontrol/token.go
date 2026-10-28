package processcontrol

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	tokenVersion     byte = 1
	tokenLifetime         = 15 * time.Minute
	tokenPayloadSize      = 1 + 4 + 8 + 8 + 8
)

// TokenIssuer issues opaque, backend-owned target references. A token binds a
// PID to the native creation time observed by the inspection engine, so the
// frontend never needs to send a timestamp and cannot substitute a target.
type TokenIssuer struct {
	key [32]byte
	now func() time.Time
}

// NewTokenIssuer creates a token issuer with a process-local random key.
func NewTokenIssuer() (*TokenIssuer, error) {
	issuer := &TokenIssuer{now: time.Now}
	if _, err := rand.Read(issuer.key[:]); err != nil {
		return nil, fmt.Errorf("cannot create token key: %w", err)
	}
	return issuer, nil
}

func (i *TokenIssuer) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, i.key[:])
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

// Issue creates an opaque token for a verified identity.
func (i *TokenIssuer) Issue(identity ProcessIdentity) (string, error) {
	if i == nil {
		return "", fmt.Errorf("token issuer unavailable")
	}
	if identity.PID <= 0 || identity.CreateTime.IsZero() {
		return "", fmt.Errorf("valid process identity required")
	}
	now := i.now()
	payload := make([]byte, tokenPayloadSize)
	payload[0] = tokenVersion
	binary.LittleEndian.PutUint32(payload[1:5], uint32(identity.PID))
	binary.LittleEndian.PutUint64(payload[5:13], uint64(identity.CreateTime.UnixNano()))
	binary.LittleEndian.PutUint64(payload[13:21], uint64(now.Unix()))
	binary.LittleEndian.PutUint64(payload[21:29], uint64(now.Add(tokenLifetime).Unix()))
	signature := i.sign(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// Verify validates a token and returns its bound PID and creation time.
func (i *TokenIssuer) Verify(token string, pid int32) (time.Time, error) {
	if i == nil {
		return time.Time{}, fmt.Errorf("token issuer unavailable")
	}
	for index := 0; index < len(token); index++ {
		if token[index] == '.' {
			payloadBytes, err := base64.RawURLEncoding.DecodeString(token[:index])
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid target token")
			}
			signature, err := base64.RawURLEncoding.DecodeString(token[index+1:])
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid target token")
			}
			if len(payloadBytes) != tokenPayloadSize || payloadBytes[0] != tokenVersion {
				return time.Time{}, fmt.Errorf("unsupported target token")
			}
			if !hmac.Equal(signature, i.sign(payloadBytes)) {
				return time.Time{}, fmt.Errorf("invalid target token")
			}
			tokenPID := int32(binary.LittleEndian.Uint32(payloadBytes[1:5]))
			if tokenPID != pid {
				return time.Time{}, fmt.Errorf("target token does not match PID")
			}
			expires := int64(binary.LittleEndian.Uint64(payloadBytes[21:29]))
			if i.now().Unix() > expires {
				return time.Time{}, fmt.Errorf("target token expired; refresh the process list")
			}
			created := int64(binary.LittleEndian.Uint64(payloadBytes[5:13]))
			return time.Unix(0, created), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid target token")
}
