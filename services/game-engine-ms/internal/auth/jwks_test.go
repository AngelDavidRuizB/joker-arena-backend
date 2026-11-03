package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	p, _ := json.Marshal(claims)
	unsigned := b64(h) + "." + b64(p)
	d := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, d[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + b64(sig)
}

func jwksServer(t *testing.T, key *rsa.PrivateKey, kid string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
}

func TestVerify(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := jwksServer(t, key, "k1")
	defer srv.Close()
	v := NewVerifier(srv.URL)
	exp := time.Now().Add(time.Hour).Unix()

	c, err := v.Verify(sign(t, key, "k1", map[string]any{"sub": "p-1", "alias": "Ana", "role": "player", "exp": exp}))
	if err != nil || c.Subject != "p-1" || c.Alias != "Ana" {
		t.Fatalf("valid token rejected: %v %+v", err, c)
	}
	cases := map[string]string{
		"expired":     sign(t, key, "k1", map[string]any{"sub": "p-1", "exp": time.Now().Add(-time.Minute).Unix()}),
		"wrong key":   sign(t, other, "k1", map[string]any{"sub": "p-1", "exp": exp}),
		"no subject":  sign(t, key, "k1", map[string]any{"exp": exp}),
		"malformed":   "abc.def",
		"unknown kid": sign(t, key, "zz", map[string]any{"sub": "p-1", "exp": exp}),
	}
	for name, tok := range cases {
		if _, err := v.Verify(tok); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: expected ErrInvalidToken, got %v", name, err)
		}
	}
}
