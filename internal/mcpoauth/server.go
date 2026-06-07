package mcpoauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/runminglu/tag-note/internal/model"
	"github.com/runminglu/tag-note/internal/service"
)

const (
	ScopeRead   = "mcp:read"
	ScopeWrite  = "mcp:write"
	ScopeDelete = "mcp:delete"

	defaultAccessTokenTTL  = time.Hour
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
	defaultCodeTTL         = 10 * time.Minute
	sessionCookieName      = "tagnote_mcp_session"
)

var supportedScopes = []string{ScopeRead, ScopeWrite, ScopeDelete}

// Config controls the MCP OAuth authorization server.
type Config struct {
	Issuer              string
	Resource            string
	ResourceMetadataURL string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	CodeTTL             time.Duration
	GoogleClientID      string
	AppleClientID       string
	AppleRedirectURI    string
}

// Server implements the MCP OAuth 2.1 endpoints.
type Server struct {
	cfg         Config
	store       *Store
	authService *service.AuthService
	sessionKey  []byte
}

func NewServer(cfg Config, store *Store, authService *service.AuthService) (*Server, error) {
	cfg.Issuer = strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	cfg.Resource = strings.TrimSpace(cfg.Resource)
	cfg.ResourceMetadataURL = strings.TrimSpace(cfg.ResourceMetadataURL)
	cfg.GoogleClientID = strings.TrimSpace(cfg.GoogleClientID)
	cfg.AppleClientID = strings.TrimSpace(cfg.AppleClientID)
	cfg.AppleRedirectURI = strings.TrimSpace(cfg.AppleRedirectURI)
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}
	if cfg.Resource == "" {
		return nil, fmt.Errorf("resource is required")
	}
	if cfg.ResourceMetadataURL == "" {
		cfg.ResourceMetadataURL = cfg.Issuer + "/.well-known/oauth-protected-resource/mcp"
	}
	if cfg.AccessTokenTTL <= 0 {
		cfg.AccessTokenTTL = defaultAccessTokenTTL
	}
	if cfg.RefreshTokenTTL <= 0 {
		cfg.RefreshTokenTTL = defaultRefreshTokenTTL
	}
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = defaultCodeTTL
	}
	key := []byte(os.Getenv("JWT_SECRET"))
	if len(key) == 0 {
		if os.Getenv("TAGNOTE_TEST_MODE") == "1" || os.Getenv("TAGNOTE_ALLOW_DEV_SECRET") == "1" {
			key = []byte("tagnote-dev-secret")
		} else {
			return nil, fmt.Errorf("JWT_SECRET is required for MCP OAuth sessions")
		}
	}
	return &Server{cfg: cfg, store: store, authService: authService, sessionKey: key}, nil
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	protected := sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:                          s.cfg.Resource,
		AuthorizationServers:              []string{s.cfg.Issuer},
		ScopesSupported:                   supportedScopes,
		BearerMethodsSupported:            []string{"header"},
		ResourceName:                      "TagNote MCP",
		ResourceDocumentation:             strings.TrimSuffix(s.cfg.Issuer, "/") + "/",
		ResourceSigningAlgValuesSupported: nil,
	})
	mux.Handle("/.well-known/oauth-protected-resource", protected)
	mux.Handle("/.well-known/oauth-protected-resource/mcp", protected)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthorizationServerMetadata)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleAuthorizationServerMetadata)
	mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	mux.HandleFunc("/oauth/register", s.handleRegister)
	mux.HandleFunc("/oauth/authorize", s.handleAuthorize)
	mux.HandleFunc("/oauth/login", s.handleLogin)
	mux.HandleFunc("/oauth/login/google", s.handleGoogleLogin)
	mux.HandleFunc("/oauth/login/apple", s.handleAppleLogin)
	mux.HandleFunc("/oauth/approve", s.handleApprove)
	mux.HandleFunc("/oauth/token", s.handleToken)
	mux.HandleFunc("/oauth/revoke", s.handleRevoke)
}

func (s *Server) BearerMiddleware(next http.Handler) http.Handler {
	return sdkauth.RequireBearerToken(s.VerifyBearerToken, &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: s.cfg.ResourceMetadataURL,
	})(next)
}

func (s *Server) VerifyBearerToken(ctx context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
	rec, err := s.store.FindAccessToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sdkauth.ErrInvalidToken
		}
		return nil, err
	}
	if rec.ExpiresAt.Before(time.Now()) {
		return nil, sdkauth.ErrInvalidToken
	}
	return &sdkauth.TokenInfo{
		Scopes:     strings.Fields(rec.Scope),
		Expiration: rec.ExpiresAt,
		UserID:     rec.UserID,
		Extra: map[string]any{
			"client_id": rec.ClientID,
		},
	}, nil
}

func (s *Server) handleAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	meta := oauthex.AuthServerMeta{
		Issuer:                            s.cfg.Issuer,
		AuthorizationEndpoint:             s.cfg.Issuer + "/oauth/authorize",
		TokenEndpoint:                     s.cfg.Issuer + "/oauth/token",
		RegistrationEndpoint:              s.cfg.Issuer + "/oauth/register",
		JWKSURI:                           s.cfg.Issuer + "/.well-known/jwks.json",
		RevocationEndpoint:                s.cfg.Issuer + "/oauth/revoke",
		ScopesSupported:                   supportedScopes,
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		CodeChallengeMethodsSupported:     []string{"S256"},
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": []any{}})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req oauthex.ClientRegistrationMetadata
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "invalid JSON request body")
		return
	}
	if len(req.RedirectURIs) == 0 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris is required")
		return
	}
	for _, redirectURI := range req.RedirectURIs {
		if !validRedirectURI(redirectURI) {
			writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect URI must be https or loopback http")
			return
		}
	}
	scope := normalizeRequestedScope(req.Scope)
	if scope == "" {
		scope = strings.Join(supportedScopes, " ")
	}
	method := req.TokenEndpointAuthMethod
	if method == "" {
		method = "none"
	}
	if method != "none" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "only public clients with token_endpoint_auth_method=none are supported")
		return
	}

	clientID, err := randomToken(24)
	if err != nil {
		http.Error(w, "generate client id", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	client := Client{
		ID:                      "mcp_" + clientID,
		Name:                    strings.TrimSpace(req.ClientName),
		RedirectURIs:            req.RedirectURIs,
		Scope:                   scope,
		TokenEndpointAuthMethod: method,
		CreatedAt:               now,
	}
	if err := s.store.RegisterClient(r.Context(), client); err != nil {
		http.Error(w, "register client", http.StatusInternalServerError)
		return
	}
	req.Scope = scope
	req.TokenEndpointAuthMethod = method
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  client.ID,
		"client_id_issued_at":        now.Unix(),
		"redirect_uris":              req.RedirectURIs,
		"token_endpoint_auth_method": req.TokenEndpointAuthMethod,
		"grant_types":                req.GrantTypes,
		"response_types":             req.ResponseTypes,
		"client_name":                req.ClientName,
		"scope":                      req.Scope,
	})
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	params, err := s.validateAuthorizeRequest(r.Context(), r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := s.sessionUserID(r)
	if !ok {
		s.renderOAuthPage(w, "Connect TagNote MCP", loginPageData{Params: params})
		return
	}
	userName := userID
	s.renderOAuthPage(w, "Approve TagNote MCP", consentPageData{
		Params:     params,
		ClientName: params.ClientName,
		UserName:   userName,
		Scopes:     strings.Fields(params.Scope),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	params, err := s.validateAuthorizeRequest(r.Context(), r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.authService.Login(r.Context(), model.LoginRequest{
		Email:    r.Form.Get("email"),
		Password: r.Form.Get("password"),
	})
	if err != nil || resp.PendingVerify {
		data := loginPageData{Params: params, Error: "Invalid email/password or unverified email."}
		s.renderOAuthPage(w, "Connect TagNote MCP", data)
		return
	}
	s.setSessionCookie(w, resp.User.ID)
	http.Redirect(w, r, "/oauth/authorize?"+params.Values().Encode(), http.StatusSeeOther)
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	params, err := s.validateAuthorizeRequest(r.Context(), r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.authService.GoogleLogin(r.Context(), model.GoogleAuthRequest{
		IDToken: strings.TrimSpace(r.Form.Get("id_token")),
	})
	if err != nil {
		data := loginPageData{Params: params, Error: "Google login failed. Try again or use email and password."}
		s.renderOAuthPage(w, "Connect TagNote MCP", data)
		return
	}
	s.setSessionCookie(w, resp.User.ID)
	http.Redirect(w, r, "/oauth/authorize?"+params.Values().Encode(), http.StatusSeeOther)
}

func (s *Server) handleAppleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	params, err := s.validateAuthorizeRequest(r.Context(), r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.authService.AppleLogin(r.Context(), model.AppleAuthRequest{
		IdentityToken: strings.TrimSpace(r.Form.Get("identity_token")),
		FullName:      strings.TrimSpace(r.Form.Get("full_name")),
	})
	if err != nil {
		data := loginPageData{Params: params, Error: "Apple login failed. Try again or use email and password."}
		s.renderOAuthPage(w, "Connect TagNote MCP", data)
		return
	}
	s.setSessionCookie(w, resp.User.ID)
	http.Redirect(w, r, "/oauth/authorize?"+params.Values().Encode(), http.StatusSeeOther)
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	params, err := s.validateAuthorizeRequest(r.Context(), r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := s.sessionUserID(r)
	if !ok {
		http.Redirect(w, r, "/oauth/authorize?"+params.Values().Encode(), http.StatusSeeOther)
		return
	}
	redirectURL, _ := url.Parse(params.RedirectURI)
	q := redirectURL.Query()
	if r.Form.Get("decision") != "approve" {
		q.Set("error", "access_denied")
		if params.State != "" {
			q.Set("state", params.State)
		}
		redirectURL.RawQuery = q.Encode()
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return
	}

	code, err := randomToken(32)
	if err != nil {
		http.Error(w, "generate code", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	err = s.store.CreateAuthorizationCode(r.Context(), code, AuthorizationCode{
		ClientID:            params.ClientID,
		UserID:              userID,
		RedirectURI:         params.RedirectURI,
		Scope:               params.Scope,
		CodeChallenge:       params.CodeChallenge,
		CodeChallengeMethod: params.CodeChallengeMethod,
		ExpiresAt:           now.Add(s.cfg.CodeTTL),
	}, now)
	if err != nil {
		http.Error(w, "store authorization code", http.StatusInternalServerError)
		return
	}
	q.Set("code", code)
	if params.State != "" {
		q.Set("state", params.State)
	}
	redirectURL.RawQuery = q.Encode()
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		s.handleAuthorizationCodeToken(w, r)
	case "refresh_token":
		s.handleRefreshToken(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported grant_type")
	}
}

func (s *Server) handleAuthorizationCodeToken(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.Form.Get("client_id"))
	client, err := s.store.FindClient(r.Context(), clientID)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "unknown client")
		return
	}
	codeRec, err := s.store.ConsumeAuthorizationCode(r.Context(), r.Form.Get("code"))
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "invalid authorization code")
		return
	}
	if codeRec.ExpiresAt.Before(time.Now()) || codeRec.ClientID != client.ID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "invalid authorization code")
		return
	}
	if redirectURI := r.Form.Get("redirect_uri"); redirectURI != "" && redirectURI != codeRec.RedirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}
	if !validPKCE(r.Form.Get("code_verifier"), codeRec.CodeChallenge) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "invalid PKCE verifier")
		return
	}
	s.issueTokenPair(w, r.Context(), TokenRecord{
		ClientID:  codeRec.ClientID,
		UserID:    codeRec.UserID,
		Scope:     codeRec.Scope,
		ExpiresAt: time.Now().UTC().Add(s.cfg.AccessTokenTTL),
	})
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.Form.Get("client_id"))
	client, err := s.store.FindClient(r.Context(), clientID)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "unknown client")
		return
	}
	accessToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "generate access token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "generate refresh token", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	rec := TokenRecord{
		ClientID:  client.ID,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}
	old, err := s.store.RotateRefreshToken(r.Context(), r.Form.Get("refresh_token"), refreshToken, rec, now)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}
	accessRec := TokenRecord{
		ClientID:  old.ClientID,
		UserID:    old.UserID,
		Scope:     old.Scope,
		ExpiresAt: now.Add(s.cfg.AccessTokenTTL),
	}
	if err := s.store.CreateAccessToken(r.Context(), accessToken, accessRec, now); err != nil {
		http.Error(w, "store access token", http.StatusInternalServerError)
		return
	}
	writeTokenResponse(w, accessToken, refreshToken, accessRec.Scope, s.cfg.AccessTokenTTL)
}

func (s *Server) issueTokenPair(w http.ResponseWriter, ctx context.Context, rec TokenRecord) {
	accessToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "generate access token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "generate refresh token", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	if err := s.store.CreateAccessToken(ctx, accessToken, rec, now); err != nil {
		http.Error(w, "store access token", http.StatusInternalServerError)
		return
	}
	refreshRec := rec
	refreshRec.ExpiresAt = now.Add(s.cfg.RefreshTokenTTL)
	if err := s.store.CreateRefreshToken(ctx, refreshToken, refreshRec, now); err != nil {
		http.Error(w, "store refresh token", http.StatusInternalServerError)
		return
	}
	writeTokenResponse(w, accessToken, refreshToken, rec.Scope, s.cfg.AccessTokenTTL)
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseForm()
	_ = s.store.RevokeRefreshToken(r.Context(), r.Form.Get("token"), time.Now().UTC())
	w.WriteHeader(http.StatusOK)
}

type authorizeParams struct {
	ResponseType        string
	ClientID            string
	ClientName          string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

func (p authorizeParams) Values() url.Values {
	v := url.Values{}
	v.Set("response_type", p.ResponseType)
	v.Set("client_id", p.ClientID)
	v.Set("redirect_uri", p.RedirectURI)
	v.Set("scope", p.Scope)
	v.Set("code_challenge", p.CodeChallenge)
	v.Set("code_challenge_method", p.CodeChallengeMethod)
	if p.State != "" {
		v.Set("state", p.State)
	}
	if p.Resource != "" {
		v.Set("resource", p.Resource)
	}
	return v
}

func (s *Server) validateAuthorizeRequest(ctx context.Context, v url.Values) (authorizeParams, error) {
	p := authorizeParams{
		ResponseType:        v.Get("response_type"),
		ClientID:            strings.TrimSpace(v.Get("client_id")),
		RedirectURI:         strings.TrimSpace(v.Get("redirect_uri")),
		Scope:               normalizeRequestedScope(v.Get("scope")),
		State:               v.Get("state"),
		CodeChallenge:       strings.TrimSpace(v.Get("code_challenge")),
		CodeChallengeMethod: strings.TrimSpace(v.Get("code_challenge_method")),
		Resource:            strings.TrimSpace(v.Get("resource")),
	}
	if p.ResponseType != "code" {
		return p, fmt.Errorf("response_type=code is required")
	}
	client, err := s.store.FindClient(ctx, p.ClientID)
	if err != nil {
		return p, fmt.Errorf("unknown client")
	}
	p.ClientName = client.Name
	if p.ClientName == "" {
		p.ClientName = p.ClientID
	}
	if !slices.Contains(client.RedirectURIs, p.RedirectURI) {
		return p, fmt.Errorf("redirect_uri is not registered")
	}
	if p.Scope == "" {
		p.Scope = client.Scope
	}
	if !scopeSubset(p.Scope, client.Scope) {
		return p, fmt.Errorf("requested scope is not registered")
	}
	if p.Resource != "" && p.Resource != s.cfg.Resource {
		return p, fmt.Errorf("resource mismatch")
	}
	if p.CodeChallenge == "" || p.CodeChallengeMethod != "S256" {
		return p, fmt.Errorf("PKCE S256 is required")
	}
	return p, nil
}

func (s *Server) sessionUserID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		return "", false
	}
	userID := parts[0]
	expUnix := parts[1]
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(userID + "." + expUnix))
	want := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(want)) != 1 {
		return "", false
	}
	expSeconds, err := strconv.ParseInt(expUnix, 10, 64)
	if err != nil || time.Unix(expSeconds, 0).Before(time.Now()) {
		return "", false
	}
	return userID, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, userID string) {
	expires := time.Now().UTC().Add(30 * time.Minute)
	exp := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(userID + "." + exp))
	value := userID + "." + exp + "." + hex.EncodeToString(mac.Sum(nil))
	secure := strings.HasPrefix(s.cfg.Issuer, "https://")
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/oauth/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func validRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	return u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}

func normalizeRequestedScope(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	seen := map[string]bool{}
	var out []string
	for _, scope := range strings.Fields(raw) {
		if !slices.Contains(supportedScopes, scope) || seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return strings.Join(out, " ")
}

func scopeSubset(requested, allowed string) bool {
	allowedMap := map[string]bool{}
	for _, scope := range strings.Fields(allowed) {
		allowedMap[scope] = true
	}
	for _, scope := range strings.Fields(requested) {
		if !allowedMap[scope] {
			return false
		}
	}
	return true
}

func validPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func randomToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func writeTokenResponse(w http.ResponseWriter, accessToken, refreshToken, scope string, ttl time.Duration) {
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(ttl.Seconds()),
		"refresh_token": refreshToken,
		"scope":         scope,
	})
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{
		"error":             code,
		"error_description": description,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type loginPageData struct {
	Params authorizeParams
	Error  string
}

type consentPageData struct {
	Params     authorizeParams
	ClientName string
	UserName   string
	Scopes     []string
}

func (s *Server) renderOAuthPage(w http.ResponseWriter, title string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := oauthTemplate
	if err := page.Execute(w, map[string]any{
		"Title":            title,
		"Data":             data,
		"GoogleClientID":   s.cfg.GoogleClientID,
		"AppleClientID":    s.cfg.AppleClientID,
		"AppleRedirectURI": s.cfg.AppleRedirectURI,
		"HasSocialLogin":   s.cfg.GoogleClientID != "" || s.cfg.AppleClientID != "",
	}); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

var oauthTemplate = template.Must(template.New("oauth").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
{{if .GoogleClientID}}<script>window.GOOGLE_CLIENT_ID={{.GoogleClientID}};</script><script src="https://accounts.google.com/gsi/client" async defer></script>{{end}}
{{if .AppleClientID}}<script>window.APPLE_CLIENT_ID={{.AppleClientID}};window.APPLE_REDIRECT_URI={{.AppleRedirectURI}};</script><script src="https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js" async defer></script>{{end}}
<style>
/* Everforest Light — matches the TagNote web app default theme. */
:root{--bg:#f3ead3;--bg-card:#fdf6e3;--text:#5c6a72;--text-secondary:#829181;--text-muted:#a6b0a0;--border:#d5c4a1;--accent:#8da101;--accent-hover:#93b259;--bg-on-accent:#fdf6e3;--tag-bg:#eae2cc;--red:#f85552;--red-bg:rgba(248,85,82,0.1);--radius:8px;--radius-sm:6px;--transition:150ms ease}
/* Everforest Dark — follows the OS preference, like the web app. */
@media (prefers-color-scheme:dark){:root{--bg:#272e33;--bg-card:#2d353b;--text:#d3c6aa;--text-secondary:#9da9a0;--text-muted:#7a8478;--border:#374145;--accent:#a7c080;--accent-hover:#83c092;--bg-on-accent:#272e33;--tag-bg:#374145;--red:#e67e80;--red-bg:rgba(230,126,128,0.12)}}
*{box-sizing:border-box}
html,body{min-height:100%;overflow-x:hidden}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Inter",sans-serif;margin:0;background:var(--bg);color:var(--text);line-height:1.5;-webkit-font-smoothing:antialiased}
.auth-container{max-width:420px;margin:80px auto;padding:0 20px;text-align:center}
.auth-brand-icon{margin-bottom:12px}
.auth-logo{font-size:2.5rem;font-weight:800;letter-spacing:0;color:var(--text)}
.auth-subtitle{color:var(--text-muted);font-size:14px;margin:4px 0 0}
.auth-features{list-style:none;padding:0;margin:20px auto 0;max-width:280px;display:flex;flex-direction:column;gap:8px;text-align:left}
.auth-features li{font-size:13px;color:var(--text-secondary)}
.auth-form{margin-top:28px;padding:28px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);text-align:left}
.auth-context{font-size:13px;color:var(--text-secondary);margin:0 0 18px;text-align:center}
.auth-tabs{display:flex;gap:0;margin-bottom:24px;border-bottom:1px solid var(--border)}
.auth-tab{flex:1;padding:12px 16px;font-size:14px;font-weight:500;font-family:inherit;color:var(--text-secondary);background:none;border:none;border-bottom:2px solid transparent;margin-bottom:-1px}
.auth-tab.active{color:var(--accent);border-bottom-color:var(--accent)}
.input-group{margin-bottom:16px}
.input-group label{display:block;font-size:13px;font-weight:500;color:var(--text);margin-bottom:6px}
.input-group input{width:100%;padding:10px 12px;font-size:14px;border:1px solid var(--border);border-radius:var(--radius-sm);font-family:inherit;outline:none;background:var(--bg-card);color:var(--text);transition:border-color var(--transition)}
.input-group input:focus{border-color:var(--accent)}
.password-input-wrapper{position:relative}
.password-input-wrapper input{padding-right:42px}
.password-toggle{position:absolute;right:8px;top:50%;transform:translateY(-50%);padding:4px;background:none;border:none;cursor:pointer;color:var(--text-secondary);display:flex;align-items:center;justify-content:center;transition:color var(--transition)}
.password-toggle:hover{color:var(--text)}
.btn{width:100%;min-height:44px;margin-top:8px;padding:10px 14px;border-radius:var(--radius-sm);border:1px solid transparent;font:inherit;font-size:14px;font-weight:650;cursor:pointer;display:flex;align-items:center;justify-content:center;gap:8px;transition:background var(--transition),border-color var(--transition)}
.btn-primary{background:var(--accent);border-color:var(--accent);color:var(--bg-on-accent)}
.btn-secondary{background:var(--bg-card);border-color:var(--border);color:var(--text)}
.btn-google{background:var(--bg-card);border-color:var(--border);color:var(--text)}
.btn-google:hover{background:var(--tag-bg);border-color:var(--border)}
.btn-google svg,.btn-apple svg{flex-shrink:0}
.btn-apple{background:#000;border-color:#000;color:#fff}
.btn-apple:hover{background:#1a1a1a}
.auth-divider{display:flex;align-items:center;margin:20px 0;color:var(--text-muted);font-size:13px}
.auth-divider:before,.auth-divider:after{content:"";flex:1;border-bottom:1px solid var(--border)}
.auth-divider span{padding:0 12px}
.error-msg{background:var(--red-bg);color:var(--red);border-radius:var(--radius-sm);padding:8px 12px;margin-bottom:16px;font-size:13px}
.meta{color:var(--text-secondary);font-size:14px;line-height:1.45;margin:0 0 16px}.scopes{padding-left:18px;margin-top:0}.row{display:flex;gap:10px;margin-top:18px}.row .btn{margin-top:0}
@media (max-width:520px){.auth-container{margin:48px auto;padding:0 16px}.auth-form{padding:22px}.auth-logo{font-size:2rem}}
</style>
</head>
<body><main class="auth-container">
<svg class="auth-brand-icon" width="48" height="48" viewBox="0 0 32 32" aria-hidden="true"><rect width="32" height="32" rx="6" fill="var(--accent)"/><path d="M8 10.5C8 9.67 8.67 9 9.5 9H17.59c.4 0 .78.16 1.06.44l5.91 5.91a1.5 1.5 0 010 2.12l-6.21 6.21a1.5 1.5 0 01-2.12 0l-5.91-5.91A1.5 1.5 0 019.88 17H9.5A1.5 1.5 0 018 15.5V10.5z" fill="none" stroke="var(--bg-on-accent)" stroke-width="1.5"/><circle cx="12.5" cy="13" r="1.5" fill="var(--bg-on-accent)"/></svg>
<div class="auth-logo">TagNote</div>
<p class="auth-subtitle">Tag your thinking. Find it instantly.</p>
{{with .Data}}{{if eq (printf "%T" .) "mcpoauth.loginPageData"}}
<ul class="auth-features"><li>✏️ Markdown-powered notes</li><li>🏷️ Organize with tags, find with search</li><li>📱 Works offline as a PWA</li></ul>
{{end}}{{end}}
<section class="auth-form">
{{with .Data}}
{{if eq (printf "%T" .) "mcpoauth.loginPageData"}}
<p class="auth-context">Connect TagNote MCP</p>
<div class="auth-tabs"><button class="auth-tab active" type="button">Login</button></div>
{{if .Error}}<div class="error-msg">{{.Error}}</div>{{end}}
<form method="post" action="/oauth/login">
{{template "params" .Params}}
<div class="input-group"><label for="oauth-email">Email</label><input id="oauth-email" name="email" type="email" autocomplete="username" required autofocus></div>
<div class="input-group"><label for="oauth-password">Password</label><div class="password-input-wrapper"><input id="oauth-password" name="password" type="password" autocomplete="current-password" required><button type="button" class="password-toggle" id="oauth-password-toggle" title="Show password" aria-label="Show password"><svg class="eye-open" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg><svg class="eye-closed" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:none"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg></button></div></div>
<button class="btn btn-primary" type="submit">Login</button>
</form>
{{if $.HasSocialLogin}}
<div class="auth-divider"><span>or</span></div>
{{if $.GoogleClientID}}<button id="google-signin-btn" class="btn btn-google" type="button">
<svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
Continue with Google
</button>{{end}}
{{if $.AppleClientID}}<button id="apple-signin-btn" class="btn btn-apple" type="button">
<svg width="16" height="16" viewBox="0 0 24 24" fill="#fff" aria-hidden="true"><path d="M17.05 12.04c-.03-2.6 2.12-3.85 2.22-3.91-1.21-1.77-3.09-2.01-3.76-2.04-1.6-.16-3.12.94-3.93.94-.81 0-2.06-.92-3.39-.89-1.74.03-3.35 1.01-4.25 2.57-1.81 3.14-.46 7.79 1.3 10.34.86 1.25 1.88 2.65 3.22 2.6 1.29-.05 1.78-.83 3.34-.83 1.56 0 2 .83 3.37.81 1.39-.03 2.27-1.27 3.12-2.53.98-1.45 1.39-2.85 1.41-2.92-.03-.01-2.71-1.04-2.74-4.12zM14.6 4.5c.71-.86 1.19-2.06 1.06-3.25-1.02.04-2.26.68-2.99 1.54-.66.76-1.23 1.98-1.08 3.15 1.14.09 2.3-.58 3.01-1.44z"/></svg>
Continue with Apple
</button>{{end}}
<form id="oauth-google-form" method="post" action="/oauth/login/google" hidden>{{template "params" .Params}}<input id="oauth-google-token" name="id_token"></form>
<form id="oauth-apple-form" method="post" action="/oauth/login/apple" hidden>{{template "params" .Params}}<input id="oauth-apple-token" name="identity_token"><input id="oauth-apple-full-name" name="full_name"></form>
{{end}}
{{else}}
<p class="auth-context">Approve TagNote MCP</p>
<p class="meta"><strong>{{.ClientName}}</strong> is requesting access to TagNote as <strong>{{.UserName}}</strong>.</p>
<ul class="meta scopes">{{range .Scopes}}<li>{{.}}</li>{{end}}</ul>
<form method="post" action="/oauth/approve">
{{template "params" .Params}}
<div class="row"><button class="btn btn-primary" name="decision" value="approve" type="submit">Approve</button><button class="btn btn-secondary" name="decision" value="deny" type="submit">Deny</button></div>
</form>
{{end}}
{{end}}
</section></main>
<script>
(function(){
  function submitForm(formId, values){
    var form=document.getElementById(formId);
    if(!form)return;
    Object.keys(values).forEach(function(key){
      var input=form.querySelector('[name="'+key+'"]');
      if(input)input.value=values[key]||'';
    });
    form.submit();
  }
  function setupGoogle(){
    var clientId=window.GOOGLE_CLIENT_ID;
    var container=document.getElementById('google-signin-btn');
    if(!clientId||!container)return;
    function configure(){
      if(typeof google==='undefined'||!google.accounts)return false;
      google.accounts.id.initialize({
        client_id:clientId,
        callback:function(response){submitForm('oauth-google-form',{id_token:response.credential});},
        auto_select:false,
        cancel_on_tap_outside:true
      });
      // Render Google's official button inside the styled container, matching the web app.
      var width=container.offsetWidth||300;
      container.innerHTML='';
      container.style.padding='0';
      container.style.border='none';
      container.style.background='transparent';
      container.style.minHeight='44px';
      google.accounts.id.renderButton(container,{type:'standard',theme:'outline',size:'large',text:'continue_with',shape:'rectangular',width:width});
      return true;
    }
    if(configure())return;
    var attempts=0;
    var timer=setInterval(function(){
      attempts++;
      if(configure()||attempts>=50)clearInterval(timer);
    },100);
  }
  function setupApple(){
    var clientId=window.APPLE_CLIENT_ID;
    var button=document.getElementById('apple-signin-btn');
    if(!clientId||!button)return;
    function configure(){
      if(typeof AppleID==='undefined'||!AppleID.auth)return false;
      AppleID.auth.init({clientId:clientId,scope:'name email',redirectURI:window.APPLE_REDIRECT_URI||window.location.origin,usePopup:true});
      return true;
    }
    configure();
    button.addEventListener('click',function(){
      if(typeof AppleID==='undefined'||!AppleID.auth)return;
      AppleID.auth.signIn().then(function(data){
        var token=data&&data.authorization&&data.authorization.id_token;
        if(!token)return;
        var fullName='';
        if(data.user&&data.user.name)fullName=[data.user.name.firstName,data.user.name.lastName].filter(Boolean).join(' ');
        submitForm('oauth-apple-form',{identity_token:token,full_name:fullName});
      }).catch(function(error){
        if(error&&(error.error==='popup_closed_by_user'||error.error==='user_cancelled_authorize'))return;
      });
    });
    var attempts=0;
    var timer=setInterval(function(){
      attempts++;
      if(configure()||attempts>=50)clearInterval(timer);
    },100);
  }
  function setupPasswordToggle(){
    var toggle=document.getElementById('oauth-password-toggle');
    var input=document.getElementById('oauth-password');
    if(!toggle||!input)return;
    var open=toggle.querySelector('.eye-open');
    var closed=toggle.querySelector('.eye-closed');
    toggle.addEventListener('click',function(){
      var show=input.type==='password';
      input.type=show?'text':'password';
      toggle.setAttribute('aria-label',show?'Hide password':'Show password');
      toggle.title=show?'Hide password':'Show password';
      if(open)open.style.display=show?'none':'';
      if(closed)closed.style.display=show?'':'none';
    });
  }
  setupGoogle();
  setupApple();
  setupPasswordToggle();
})();
</script>
</body></html>
{{define "params"}}
<input type="hidden" name="response_type" value="{{.ResponseType}}">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
<input type="hidden" name="resource" value="{{.Resource}}">
{{end}}`))
