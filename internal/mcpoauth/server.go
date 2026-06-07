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
		scope = ScopeRead + " " + ScopeWrite
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
	params, err := s.validateAuthorizeRequest(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := s.sessionUserID(r)
	if !ok {
		renderOAuthPage(w, "Connect TagNote MCP", loginPageData{Params: params})
		return
	}
	userName := userID
	renderOAuthPage(w, "Approve TagNote MCP", consentPageData{
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
	params, err := s.validateAuthorizeRequest(r.Form)
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
		renderOAuthPage(w, "Connect TagNote MCP", data)
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
	params, err := s.validateAuthorizeRequest(r.Form)
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

func (s *Server) validateAuthorizeRequest(v url.Values) (authorizeParams, error) {
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
	client, err := s.store.FindClient(context.Background(), p.ClientID)
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

func renderOAuthPage(w http.ResponseWriter, title string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := oauthTemplate
	if err := page.Execute(w, map[string]any{"Title": title, "Data": data}); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

var oauthTemplate = template.Must(template.New("oauth").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;background:#f7f7f4;color:#20201d}
main{max-width:440px;margin:8vh auto;padding:24px}
.panel{background:white;border:1px solid #ddd9cf;border-radius:8px;padding:24px;box-shadow:0 8px 30px rgba(30,27,22,.08)}
h1{font-size:22px;margin:0 0 18px}
label{display:block;font-size:13px;font-weight:600;margin:14px 0 6px}
input{box-sizing:border-box;width:100%;font:inherit;padding:10px 12px;border:1px solid #c9c3b8;border-radius:6px}
button{font:inherit;font-weight:650;padding:10px 14px;border-radius:6px;border:1px solid #2f5b4c;background:#2f5b4c;color:white;cursor:pointer}
.row{display:flex;gap:10px;margin-top:18px}
.secondary{background:white;color:#3c3932;border-color:#c9c3b8}
.error{background:#fff0f0;color:#8c1d18;border:1px solid #f0c7c3;border-radius:6px;padding:10px;margin:0 0 12px}
.meta{color:#5d594f;font-size:14px;line-height:1.45}.scopes{padding-left:18px}
</style>
</head>
<body><main><section class="panel">
{{with .Data}}
{{if eq (printf "%T" .) "mcpoauth.loginPageData"}}
<h1>Connect TagNote MCP</h1>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<form method="post" action="/oauth/login">
{{template "params" .Params}}
<label>Email</label><input name="email" type="email" autocomplete="username" required autofocus>
<label>Password</label><input name="password" type="password" autocomplete="current-password" required>
<div class="row"><button type="submit">Continue</button></div>
</form>
{{else}}
<h1>Approve TagNote MCP</h1>
<p class="meta"><strong>{{.ClientName}}</strong> is requesting access to TagNote as <strong>{{.UserName}}</strong>.</p>
<ul class="meta scopes">{{range .Scopes}}<li>{{.}}</li>{{end}}</ul>
<form method="post" action="/oauth/approve">
{{template "params" .Params}}
<div class="row"><button name="decision" value="approve" type="submit">Approve</button><button class="secondary" name="decision" value="deny" type="submit">Deny</button></div>
</form>
{{end}}
{{end}}
</section></main></body></html>
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
