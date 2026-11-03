// Package auth validates the RS256 JWTs issued by auth-identity-ms against its JWKS.
package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Claims are the fields of the token used by the engine.
type Claims struct {
	Subject string `json:"sub"`
	Alias   string `json:"alias"`
	Role    string `json:"role"`
	Exp     int64  `json:"exp"`
	JTI     string `json:"jti"`
}

// ErrInvalidToken covers every validation failure (format, signature, expiry).
var ErrInvalidToken = errors.New("invalid token")

// Verifier validates tokens; keys are fetched from the JWKS URL and cached.
type Verifier struct {
	jwksURL string
	client  *http.Client
	ttl     time.Duration
	now     func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// NewVerifier creates a verifier for the given JWKS endpoint.
func NewVerifier(jwksURL string) *Verifier {
	return &Verifier{jwksURL: jwksURL, client: &http.Client{Timeout: 3 * time.Second}, ttl: 5 * time.Minute, now: time.Now}
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *Verifier) refresh() error {
	resp, err := v.client.Get(v.jwksURL)
	if err != nil {
		return fmt.Errorf("JWKS unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS answered %d", resp.StatusCode)
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("invalid JWKS: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return errors.New("JWKS has no RSA keys")
	}
	v.keys, v.fetched = keys, v.now()
	return nil
}

func (v *Verifier) key(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys == nil || v.now().Sub(v.fetched) > v.ttl {
		if err := v.refresh(); err != nil {
			return nil, err
		}
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	// Unknown kid: the issuer may have rotated keys; refresh once.
	if err := v.refresh(); err != nil {
		return nil, err
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
}

// Verify checks the signature (RS256) and expiry, and returns the claims.
func (v *Verifier) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: malformed", ErrInvalidToken)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(hb, &header) != nil || header.Alg != "RS256" {
		return nil, fmt.Errorf("%w: unsupported header", ErrInvalidToken)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: bad signature encoding", ErrInvalidToken)
	}
	pub, err := v.key(header.Kid)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		return nil, fmt.Errorf("%w: bad signature", ErrInvalidToken)
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: bad payload", ErrInvalidToken)
	}
	var c Claims
	if err := json.Unmarshal(pb, &c); err != nil || c.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject", ErrInvalidToken)
	}
	if c.Exp == 0 || v.now().Unix() >= c.Exp {
		return nil, fmt.Errorf("%w: expired", ErrInvalidToken)
	}
	return &c, nil
}
