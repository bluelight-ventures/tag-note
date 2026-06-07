package mcpoauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/runminglu/tag-note/internal/model"
	"github.com/runminglu/tag-note/internal/repo"
	"github.com/runminglu/tag-note/internal/service"
)

func TestOAuthE2EAuthorizationCodeAndRefresh(t *testing.T) {
	t.Setenv("TAGNOTE_ALLOW_DEV_SECRET", "1")

	ctx := context.Background()
	sqliteRepo, err := repo.NewSQLiteRepo(t.TempDir() + "/tagnote.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	defer sqliteRepo.Close()

	authSvc, err := service.NewAuth(sqliteRepo, service.NewEmailService(), t.TempDir())
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	if _, err := authSvc.Register(ctx, model.RegisterRequest{
		Email:       "mcp-user@example.com",
		Password:    "password123",
		DisplayName: "MCP User",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	mux := http.NewServeMux()
	oauthServer, err := NewServer(Config{
		Issuer:              "http://mcp.example.test",
		Resource:            "http://mcp.example.test/mcp",
		ResourceMetadataURL: "http://mcp.example.test/.well-known/oauth-protected-resource/mcp",
	}, NewStore(sqliteRepo.DB()), authSvc)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	oauthServer.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	clientID := registerClientForTest(t, server.URL)
	verifier := "test-code-verifier-abcdefghijklmnopqrstuvwxyz"
	challenge := pkceChallenge(verifier)
	redirectURI := "http://127.0.0.1/callback"

	authorizeValues := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {ScopeRead + " " + ScopeWrite},
		"state":                 {"state-123"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"resource":              {"http://mcp.example.test/mcp"},
	}

	noRedirectClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	loginForm := cloneValues(authorizeValues)
	loginForm.Set("email", "mcp-user@example.com")
	loginForm.Set("password", "password123")
	loginResp, err := noRedirectClient.PostForm(server.URL+"/oauth/login", loginForm)
	if err != nil {
		t.Fatalf("POST /oauth/login error = %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", loginResp.StatusCode)
	}
	cookies := loginResp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	approveForm := cloneValues(authorizeValues)
	approveForm.Set("decision", "approve")
	approveReq, err := http.NewRequest(http.MethodPost, server.URL+"/oauth/approve", strings.NewReader(approveForm.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	approveReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		approveReq.AddCookie(cookie)
	}
	approveResp, err := noRedirectClient.Do(approveReq)
	if err != nil {
		t.Fatalf("POST /oauth/approve error = %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusFound {
		t.Fatalf("approve status = %d, want 302", approveResp.StatusCode)
	}
	location, err := url.Parse(approveResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse approval redirect: %v", err)
	}
	code := location.Query().Get("code")
	if code == "" {
		t.Fatalf("approval redirect missing code: %s", location.String())
	}
	if got := location.Query().Get("state"); got != "state-123" {
		t.Fatalf("state = %q, want state-123", got)
	}

	tokenResp := exchangeCodeForTest(t, server.URL, clientID, redirectURI, code, verifier)
	tokenInfo, err := oauthServer.VerifyBearerToken(ctx, tokenResp.AccessToken, nil)
	if err != nil {
		t.Fatalf("VerifyBearerToken(access) error = %v", err)
	}
	if tokenInfo.UserID == "" {
		t.Fatal("verified token user ID is empty")
	}
	if strings.Join(tokenInfo.Scopes, " ") != ScopeRead+" "+ScopeWrite {
		t.Fatalf("scopes = %#v", tokenInfo.Scopes)
	}

	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {tokenResp.RefreshToken},
	}
	refreshHTTPResp, err := http.PostForm(server.URL+"/oauth/token", refreshForm)
	if err != nil {
		t.Fatalf("refresh token POST error = %v", err)
	}
	defer refreshHTTPResp.Body.Close()
	if refreshHTTPResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200", refreshHTTPResp.StatusCode)
	}
	var refreshed tokenResponse
	if err := json.NewDecoder(refreshHTTPResp.Body).Decode(&refreshed); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.AccessToken == tokenResp.AccessToken {
		t.Fatalf("invalid refreshed tokens: %#v", refreshed)
	}
}

func TestAuthorizeLoginPageRendersTagNoteSocialAuth(t *testing.T) {
	t.Setenv("TAGNOTE_ALLOW_DEV_SECRET", "1")
	t.Setenv("GOOGLE_CLIENT_ID", "web-client.apps.googleusercontent.com")
	t.Setenv("APPLE_CLIENT_ID", "com.example.tagnote.web")

	ctx := context.Background()
	sqliteRepo, err := repo.NewSQLiteRepo(t.TempDir() + "/tagnote.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	defer sqliteRepo.Close()
	authSvc, err := service.NewAuth(sqliteRepo, service.NewEmailService(), t.TempDir())
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	if err := sqliteRepo.CreateUser(ctx, "mcp-social-user", "mcp-social@example.com", "hash", "MCP Social", time.Now().UTC()); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	mux := http.NewServeMux()
	oauthServer, err := NewServer(Config{
		Issuer:              "http://mcp.example.test",
		Resource:            "http://mcp.example.test/mcp",
		ResourceMetadataURL: "http://mcp.example.test/.well-known/oauth-protected-resource/mcp",
		GoogleClientID:      "web-client.apps.googleusercontent.com",
		AppleClientID:       "com.example.tagnote.web",
		AppleRedirectURI:    "https://tag-note.com/app",
	}, NewStore(sqliteRepo.DB()), authSvc)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	oauthServer.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	clientID := registerClientForTest(t, server.URL)
	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {"http://127.0.0.1/callback"},
		"scope":                 {ScopeRead + " " + ScopeWrite},
		"state":                 {"state-123"},
		"code_challenge":        {pkceChallenge("test-code-verifier-abcdefghijklmnopqrstuvwxyz")},
		"code_challenge_method": {"S256"},
		"resource":              {"http://mcp.example.test/mcp"},
	}

	resp, err := http.Get(server.URL + "/oauth/authorize?" + values.Encode())
	if err != nil {
		t.Fatalf("GET /oauth/authorize error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorize status = %d, want 200", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read authorize body: %v", err)
	}
	body := string(bodyBytes)
	for _, want := range []string{
		"TagNote",
		"Tag your thinking. Find it instantly.",
		"Connect TagNote MCP",
		`class="auth-features"`,
		"Markdown-powered notes",
		`id="oauth-password-toggle"`,
		`id="google-signin-btn"`,
		"Continue with Google",
		`id="apple-signin-btn"`,
		"Continue with Apple",
		`/oauth/login/google`,
		`/oauth/login/apple`,
		"https://accounts.google.com/gsi/client",
		"https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("authorize body missing %q", want)
		}
	}
}

func registerClientForTest(t *testing.T, baseURL string) string {
	t.Helper()
	req := oauthex.ClientRegistrationMetadata{
		RedirectURIs:            []string{"http://127.0.0.1/callback"},
		TokenEndpointAuthMethod: "none",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		ClientName:              "MCP Test Client",
		Scope:                   ScopeRead + " " + ScopeWrite,
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/oauth/register", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST /oauth/register error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", resp.StatusCode)
	}
	var registered oauthex.ClientRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&registered); err != nil {
		t.Fatalf("decode registration: %v", err)
	}
	if registered.ClientID == "" {
		t.Fatal("registered client_id is empty")
	}
	return registered.ClientID
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func exchangeCodeForTest(t *testing.T, baseURL, clientID, redirectURI, code, verifier string) tokenResponse {
	t.Helper()
	resp, err := http.PostForm(baseURL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code":          {code},
		"code_verifier": {verifier},
	})
	if err != nil {
		t.Fatalf("POST /oauth/token error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d, want 200", resp.StatusCode)
	}
	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.TokenType != "Bearer" {
		t.Fatalf("invalid token response: %#v", token)
	}
	return token
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func cloneValues(values url.Values) url.Values {
	clone := url.Values{}
	for key, vals := range values {
		clone[key] = append([]string(nil), vals...)
	}
	return clone
}
