// Package arweave implements a real, self-signing Arweave publisher.
//
// The package speaks Arweave's HTTP wire protocol directly: it loads an
// Arweave JWK wallet (RSA-4096, RSA-PSS-SHA256), builds format-2
// transactions, computes the deep-hash signature payload, signs it with
// crypto/rsa, and posts the tx + data chunks to any Arweave node.
//
// No third-party Arweave SDK is required.
package arweave

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
)

// JWK is the JSON Web Key shape used by Arweave wallets.
// All values are unpadded base64url-encoded big integers (RFC 7518).
type JWK struct {
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
	D   string `json:"d"`
	P   string `json:"p"`
	Q   string `json:"q"`
	Dp  string `json:"dp"`
	Dq  string `json:"dq"`
	Qi  string `json:"qi"`
}

// Wallet is a parsed Arweave JWK ready for signing.
type Wallet struct {
	Key     *rsa.PrivateKey
	OwnerN  []byte // raw modulus bytes (used as the tx Owner field)
	Address string // sha256(modulus) base64url
}

// LoadWalletFromFile reads and parses an Arweave JWK file.
func LoadWalletFromFile(path string) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("arweave: read wallet: %w", err)
	}
	return LoadWalletFromJSON(data)
}

// LoadWalletFromJSON parses a JWK byte blob.
func LoadWalletFromJSON(data []byte) (*Wallet, error) {
	var jwk JWK
	if err := json.Unmarshal(data, &jwk); err != nil {
		return nil, fmt.Errorf("arweave: parse JWK: %w", err)
	}
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("arweave: unsupported kty %q (want RSA)", jwk.Kty)
	}
	n, err := b64URLDecode(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("arweave: bad n: %w", err)
	}
	e, err := b64URLDecode(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("arweave: bad e: %w", err)
	}
	d, err := b64URLDecode(jwk.D)
	if err != nil {
		return nil, fmt.Errorf("arweave: bad d: %w", err)
	}
	p, err := b64URLDecode(jwk.P)
	if err != nil {
		return nil, fmt.Errorf("arweave: bad p: %w", err)
	}
	q, err := b64URLDecode(jwk.Q)
	if err != nil {
		return nil, fmt.Errorf("arweave: bad q: %w", err)
	}

	priv := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		},
		D: new(big.Int).SetBytes(d),
		Primes: []*big.Int{
			new(big.Int).SetBytes(p),
			new(big.Int).SetBytes(q),
		},
	}
	if err := priv.Validate(); err != nil {
		return nil, fmt.Errorf("arweave: invalid RSA key: %w", err)
	}
	priv.Precompute()

	addr := sha256.Sum256(n)
	return &Wallet{
		Key:     priv,
		OwnerN:  n,
		Address: b64URLEncode(addr[:]),
	}, nil
}

// b64URLEncode encodes b as unpadded base64url, matching Arweave's wire format.
func b64URLEncode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// b64URLDecode tolerates both padded and unpadded base64url encodings.
func b64URLDecode(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	// Strip any padding to allow either form.
	for len(s) > 0 && s[len(s)-1] == '=' {
		s = s[:len(s)-1]
	}
	return base64.RawURLEncoding.DecodeString(s)
}
