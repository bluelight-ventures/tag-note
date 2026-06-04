package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"

	"github.com/runminglu/tag-note/internal/model"
)

const (
	appleIssuer  = "https://appleid.apple.com"
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
)

// appleKeySet caches Apple's JSON Web Key Set (the public keys used to verify
// Sign in with Apple identity tokens). Apple rotates these keys, so the cache
// has a TTL and refreshes on a key-id miss.
type appleKeySet struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
	ttl     time.Duration
	url     string
	client  *http.Client
}

func newAppleKeySet(url string) *appleKeySet {
	return &appleKeySet{
		keys:   map[string]*rsa.PublicKey{},
		ttl:    time.Hour,
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// get returns the RSA public key for the given key id, refreshing the cache
// when the key is unknown or stale. When url is empty (tests inject keys
// directly) it never fetches.
func (ks *appleKeySet) get(kid string) (*rsa.PublicKey, error) {
	ks.mu.RLock()
	key, ok := ks.keys[kid]
	fresh := time.Since(ks.fetched) < ks.ttl
	ks.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}

	if ks.url != "" {
		if err := ks.refresh(); err != nil {
			if ok {
				return key, nil // fall back to the stale-but-known key
			}
			return nil, err
		}
		ks.mu.RLock()
		key, ok = ks.keys[kid]
		ks.mu.RUnlock()
	}

	if !ok {
		return nil, fmt.Errorf("apple: unknown key id %q", kid)
	}
	return key, nil
}

func (ks *appleKeySet) refresh() error {
	resp, err := ks.client.Get(ks.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apple jwks: status %d", resp.StatusCode)
	}

	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	ks.mu.Lock()
	ks.keys = keys
	ks.fetched = time.Now()
	ks.mu.Unlock()
	return nil
}

// jwkToRSAPublicKey builds an *rsa.PublicKey from the base64url-encoded modulus
// (n) and exponent (e) of a JWK.
func jwkToRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("apple jwk: decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("apple jwk: decode exponent: %w", err)
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() || e.Int64() < 2 {
		return nil, fmt.Errorf("apple jwk: invalid exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}

type appleClaims struct {
	Sub   string
	Email string
	Nonce string
}

// verifyAppleToken verifies an Apple identity token's signature and claims and
// returns the subject/email. When expectedNonce is non-empty, the token's nonce
// claim must equal the SHA-256 hex of expectedNonce.
func (a *AuthService) verifyAppleToken(identityToken, expectedNonce string) (*appleClaims, error) {
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithIssuer(appleIssuer),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	_, err := parser.ParseWithClaims(identityToken, claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("apple: token missing key id")
		}
		return a.appleKeys.get(kid)
	})
	if err != nil {
		return nil, err
	}

	// The token audience must be one of our configured client IDs (the native
	// app bundle id and/or the web Services ID).
	if !appleAudienceMatches(claims["aud"], a.appleClientIDs) {
		return nil, fmt.Errorf("apple: token audience mismatch")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("apple: token missing subject")
	}
	email, _ := claims["email"].(string)
	nonce, _ := claims["nonce"].(string)

	if expectedNonce != "" {
		if !constantTimeCompare(nonce, sha256Hex(expectedNonce)) {
			return nil, fmt.Errorf("apple: nonce mismatch")
		}
	}

	return &appleClaims{Sub: sub, Email: email, Nonce: nonce}, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// appleAudienceMatches reports whether the token's aud claim (a string, or an
// array of strings) contains any of the allowed client IDs.
func appleAudienceMatches(aud interface{}, allowed []string) bool {
	switch v := aud.(type) {
	case string:
		return containsString(allowed, v)
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && containsString(allowed, s) {
				return true
			}
		}
	}
	return false
}

// AppleLogin handles Sign in with Apple authentication, mirroring GoogleLogin:
// find by stable Apple subject, else link an existing account by email, else
// create a new account.
func (a *AuthService) AppleLogin(ctx context.Context, req model.AppleAuthRequest) (*model.AuthResponse, error) {
	if len(a.appleClientIDs) == 0 {
		return nil, fmt.Errorf("Apple login is not configured")
	}

	claims, err := a.verifyAppleToken(req.IdentityToken, req.Nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid Apple token: %w", err)
	}

	// Returning user: identified by the stable Apple subject. Apple omits the
	// email after the first authorization, so this path must not require it.
	if user, err := a.repo.FindUserByAppleID(ctx, claims.Sub); err == nil {
		token, err := a.generateToken(user.ID, user.Email)
		if err != nil {
			return nil, err
		}
		return &model.AuthResponse{Token: token, User: *user}, nil
	}

	// Beyond this point we need the email (only present on first authorization).
	if claims.Email == "" {
		return nil, fmt.Errorf("Apple did not provide an email; please sign in again")
	}

	// Link to an existing account with the same email.
	if user, _, err := a.repo.FindUserByEmail(ctx, claims.Email); err == nil {
		if err := a.repo.LinkAppleID(ctx, user.ID, claims.Sub); err != nil {
			return nil, fmt.Errorf("link Apple ID: %w", err)
		}
		if !user.EmailVerified {
			if err := a.repo.SetEmailVerified(ctx, user.ID, true); err != nil {
				return nil, fmt.Errorf("set email verified: %w", err)
			}
		}
		user.HasApple = true
		user.EmailVerified = true
		token, err := a.generateToken(user.ID, user.Email)
		if err != nil {
			return nil, err
		}
		return &model.AuthResponse{Token: token, User: *user}, nil
	}

	// Create a new user (email auto-verified). Private-relay addresses are
	// accepted as-is.
	now := time.Now().UTC()
	id := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
	displayName := strings.TrimSpace(req.FullName)
	if displayName == "" {
		displayName = claims.Email
	}

	if err := a.repo.CreateUserWithApple(ctx, id, claims.Email, claims.Sub, displayName, now); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	a.seedOnboardingContent(ctx, id)

	token, err := a.generateToken(id, claims.Email)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: model.User{
			ID:            id,
			Email:         claims.Email,
			DisplayName:   displayName,
			CreatedAt:     now,
			EmailVerified: true,
			HasApple:      true,
		},
	}, nil
}
