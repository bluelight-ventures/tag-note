package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/runminglu/tag-note/internal/model"
	"github.com/runminglu/tag-note/internal/repo"
)

const testAppleAud = "com.tag-note.tagnote"

// newAppleTestService builds an AuthService whose Apple key set contains only
// the provided public key under kid, with no network fetch.
func newAppleTestService(t *testing.T, kid string, pub *rsa.PublicKey, r repo.Repository) *AuthService {
	t.Helper()
	t.Setenv("TAGNOTE_ALLOW_DEV_SECRET", "1")
	auth, err := NewAuth(r, NewEmailService(), t.TempDir())
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	auth.appleClientID = testAppleAud
	auth.appleKeys = &appleKeySet{
		keys:    map[string]*rsa.PublicKey{kid: pub},
		fetched: time.Now(),
		ttl:     time.Hour,
		// url empty -> never fetches; uses injected key only.
	}
	return auth
}

func mintAppleToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func validAppleClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":   appleIssuer,
		"aud":   testAppleAud,
		"sub":   "000111.abcdef0123456789.0000",
		"email": "appleuser@example.com",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"nonce": sha256Hex("raw-nonce-123"),
	}
}

func TestVerifyAppleToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "test-kid"
	auth := newAppleTestService(t, kid, &key.PublicKey, nil)

	t.Run("valid", func(t *testing.T) {
		token := mintAppleToken(t, key, kid, validAppleClaims())
		claims, err := auth.verifyAppleToken(token, "raw-nonce-123")
		if err != nil {
			t.Fatalf("verifyAppleToken() error = %v", err)
		}
		if claims.Sub != "000111.abcdef0123456789.0000" {
			t.Errorf("sub = %q", claims.Sub)
		}
		if claims.Email != "appleuser@example.com" {
			t.Errorf("email = %q", claims.Email)
		}
	})

	t.Run("valid without nonce check", func(t *testing.T) {
		token := mintAppleToken(t, key, kid, validAppleClaims())
		if _, err := auth.verifyAppleToken(token, ""); err != nil {
			t.Fatalf("verifyAppleToken(no nonce) error = %v", err)
		}
	})

	negatives := []struct {
		name    string
		mutate  func(jwt.MapClaims)
		nonce   string
		signKey *rsa.PrivateKey
		kid     string
	}{
		{name: "wrong audience", mutate: func(c jwt.MapClaims) { c["aud"] = "com.attacker.app" }, nonce: "raw-nonce-123"},
		{name: "wrong issuer", mutate: func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" }, nonce: "raw-nonce-123"},
		{name: "expired", mutate: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, nonce: "raw-nonce-123"},
		{name: "missing subject", mutate: func(c jwt.MapClaims) { delete(c, "sub") }, nonce: "raw-nonce-123"},
		{name: "nonce mismatch", mutate: func(c jwt.MapClaims) {}, nonce: "wrong-nonce"},
		{name: "unknown kid", mutate: func(c jwt.MapClaims) {}, nonce: "raw-nonce-123", kid: "other-kid"},
		{name: "tampered signature", mutate: func(c jwt.MapClaims) {}, nonce: "raw-nonce-123", signKey: otherKey},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			claims := validAppleClaims()
			tc.mutate(claims)
			signKey := key
			if tc.signKey != nil {
				signKey = tc.signKey
			}
			useKid := kid
			if tc.kid != "" {
				useKid = tc.kid
			}
			token := mintAppleToken(t, signKey, useKid, claims)
			if _, err := auth.verifyAppleToken(token, tc.nonce); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestAppleLoginCreatesLinksAndReturns(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	const kid = "test-kid"

	ctx := context.Background()
	r, err := repo.NewSQLiteRepo(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo: %v", err)
	}
	defer r.Close()
	auth := newAppleTestService(t, kid, &key.PublicKey, r)

	// 1) First sign-in creates a new account with email + name.
	first := validAppleClaims()
	first["sub"] = "apple-sub-1"
	first["email"] = "new@example.com"
	tok := mintAppleToken(t, key, kid, first)
	resp, err := auth.AppleLogin(ctx, appleAuthReq(tok, "raw-nonce-123", "Ada Lovelace"))
	if err != nil {
		t.Fatalf("AppleLogin(create) error = %v", err)
	}
	if resp.Token == "" || !resp.User.HasApple || resp.User.Email != "new@example.com" {
		t.Fatalf("create response = %+v", resp.User)
	}
	if resp.User.DisplayName != "Ada Lovelace" {
		t.Errorf("display name = %q, want full name", resp.User.DisplayName)
	}
	createdID := resp.User.ID

	// 2) Returning sign-in with NO email (Apple omits it) still logs in via sub.
	returning := validAppleClaims()
	returning["sub"] = "apple-sub-1"
	delete(returning, "email")
	tok2 := mintAppleToken(t, key, kid, returning)
	resp2, err := auth.AppleLogin(ctx, appleAuthReq(tok2, "raw-nonce-123", ""))
	if err != nil {
		t.Fatalf("AppleLogin(returning) error = %v", err)
	}
	if resp2.User.ID != createdID {
		t.Fatalf("returning user id = %q, want %q", resp2.User.ID, createdID)
	}

	// 3) Apple sign-in on an email that already has a password account links it.
	now := time.Now().UTC()
	if err := r.CreateUser(ctx, "pw-user", "linkme@example.com", "hash", "PW User", now); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	link := validAppleClaims()
	link["sub"] = "apple-sub-2"
	link["email"] = "linkme@example.com"
	tok3 := mintAppleToken(t, key, kid, link)
	resp3, err := auth.AppleLogin(ctx, appleAuthReq(tok3, "raw-nonce-123", ""))
	if err != nil {
		t.Fatalf("AppleLogin(link) error = %v", err)
	}
	if resp3.User.ID != "pw-user" || !resp3.User.HasApple {
		t.Fatalf("link response = %+v", resp3.User)
	}
	linked, err := r.FindUserByAppleID(ctx, "apple-sub-2")
	if err != nil || linked.ID != "pw-user" {
		t.Fatalf("FindUserByAppleID after link = %+v, err = %v", linked, err)
	}
}

func appleAuthReq(identityToken, nonce, fullName string) model.AppleAuthRequest {
	return model.AppleAuthRequest{IdentityToken: identityToken, Nonce: nonce, FullName: fullName}
}
