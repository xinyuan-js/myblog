package auth

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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xinyuan-js/myblog/apps/api/internal/config"
)

var (
	ErrNotConfigured   = errors.New("github oauth is not configured")
	ErrInvalidState    = errors.New("invalid oauth state")
	ErrForbidden       = errors.New("github user is not the configured administrator")
	ErrUnauthenticated = errors.New("session is missing or invalid")
)

const oauthStateCookie = "blog_oauth_state"

type AdminUser struct {
	GitHubID  uint64 `json:"githubId"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type Session struct {
	User      AdminUser
	CSRFToken string
	TokenHash [32]byte
	rawToken  string
}

type Service struct {
	db     *sql.DB
	cfg    config.Config
	client *http.Client
	now    func() time.Time
}

func NewService(db *sql.DB, cfg config.Config) *Service {
	return &Service{db: db, cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}, now: time.Now}
}

func (s *Service) Configured() bool {
	return s.cfg.GitHubClientID != "" && s.cfg.GitHubClientSecret != "" &&
		s.cfg.GitHubAdminID != 0 && len(s.cfg.OAuthStateSecret) >= 32
}

func (s *Service) SessionCookieName() string { return s.cfg.SessionCookieName }

func (s *Service) AppURL(path string) string {
	return strings.TrimSuffix(s.cfg.AppOrigin, "/") + path
}

func (s *Service) BeginLogin(returnTo string) (authorizeURL string, cookie *http.Cookie, err error) {
	if !s.Configured() {
		return "", nil, ErrNotConfigured
	}
	nonce, err := randomToken(32)
	if err != nil {
		return "", nil, err
	}
	state := oauthState{
		Nonce: nonce, ReturnTo: sanitizeReturnTo(returnTo), ExpiresAt: s.now().UTC().Add(10 * time.Minute).Unix(),
	}
	nonceHash := sha256.Sum256([]byte(nonce))
	if _, err := s.db.Exec(`
		INSERT INTO oauth_states (nonce_hash, expires_at) VALUES (?, ?)
	`, nonceHash[:], time.Unix(state.ExpiresAt, 0).UTC()); err != nil {
		return "", nil, fmt.Errorf("store oauth state: %w", err)
	}
	signed, err := s.signState(state)
	if err != nil {
		return "", nil, err
	}
	query := url.Values{
		"client_id":    {s.cfg.GitHubClientID},
		"redirect_uri": {s.cfg.GitHubCallbackURL},
		"scope":        {"read:user"},
		"state":        {nonce},
	}
	return "https://github.com/login/oauth/authorize?" + query.Encode(), &http.Cookie{
		Name: oauthStateCookie, Value: signed, Path: "/api/auth/github/callback",
		MaxAge: 600, HttpOnly: true, Secure: s.cfg.SessionSecure, SameSite: http.SameSiteLaxMode,
	}, nil
}

func (s *Service) CompleteLogin(ctx context.Context, code, queryState, signedState string) (Session, string, error) {
	if !s.Configured() {
		return Session{}, "", ErrNotConfigured
	}
	state, err := s.verifyState(signedState)
	if err != nil || queryState == "" || subtle.ConstantTimeCompare([]byte(queryState), []byte(state.Nonce)) != 1 ||
		state.ExpiresAt < s.now().UTC().Unix() {
		return Session{}, "", ErrInvalidState
	}
	nonceHash := sha256.Sum256([]byte(state.Nonce))
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM oauth_states WHERE nonce_hash = ? AND expires_at >= UTC_TIMESTAMP(6)
	`, nonceHash[:])
	if err != nil {
		return Session{}, "", fmt.Errorf("consume oauth state: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return Session{}, "", ErrInvalidState
	}
	accessToken, err := s.exchangeCode(ctx, code)
	if err != nil {
		return Session{}, "", err
	}
	user, err := s.fetchGitHubUser(ctx, accessToken)
	if err != nil {
		return Session{}, "", err
	}
	if user.GitHubID != s.cfg.GitHubAdminID {
		return Session{}, "", ErrForbidden
	}
	session, err := s.createSession(ctx, user)
	return session, state.ReturnTo, err
}

func (s *Service) SessionCookie(session Session) (*http.Cookie, error) {
	// The raw tokens only exist in the HttpOnly cookie. The database stores hashes.
	value := session.rawToken
	if value == "" {
		return nil, errors.New("session tokens are missing")
	}
	return &http.Cookie{
		Name: s.cfg.SessionCookieName, Value: value, Path: "/api", MaxAge: int(s.cfg.SessionTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.SessionSecure, SameSite: http.SameSiteLaxMode,
	}, nil
}

func (s *Service) ClearSessionCookie() *http.Cookie {
	return &http.Cookie{Name: s.cfg.SessionCookieName, Value: "", Path: "/api", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SessionSecure, SameSite: http.SameSiteLaxMode}
}

func (s *Service) ClearStateCookie() *http.Cookie {
	return &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/api/auth/github/callback", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SessionSecure, SameSite: http.SameSiteLaxMode}
}

func (s *Service) Authenticate(ctx context.Context, cookieValue string) (Session, error) {
	token, ok := parseSessionCookie(cookieValue)
	if !ok {
		return Session{}, ErrUnauthenticated
	}
	tokenHash := sha256.Sum256([]byte(token))
	csrf := s.csrfToken(token)
	csrfHash := sha256.Sum256([]byte(csrf))
	var session Session
	var githubID uint64
	var storedCSRFHash []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT github_id, github_login, display_name, avatar_url, csrf_token_hash
		FROM admin_sessions
		WHERE token_hash = ? AND expires_at > UTC_TIMESTAMP(6)
	`, tokenHash[:]).Scan(&githubID, &session.User.Login, &session.User.Name, &session.User.AvatarURL, &storedCSRFHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("query admin session: %w", err)
	}
	if len(storedCSRFHash) != len(csrfHash) || subtle.ConstantTimeCompare(storedCSRFHash, csrfHash[:]) != 1 {
		return Session{}, ErrUnauthenticated
	}
	// Re-check the immutable administrator ID on every authenticated request.
	// This immediately revokes old sessions if ADMIN_GITHUB_ID is changed and
	// prevents a valid-looking database row for another GitHub user from being
	// treated as an administrator.
	if githubID == 0 || githubID != s.cfg.GitHubAdminID {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = ?`, tokenHash[:])
		return Session{}, ErrUnauthenticated
	}
	session.User.GitHubID = githubID
	session.CSRFToken = csrf
	session.TokenHash = tokenHash
	_, _ = s.db.ExecContext(ctx, `UPDATE admin_sessions SET last_seen_at = UTC_TIMESTAMP(6) WHERE token_hash = ?`, tokenHash[:])
	return session, nil
}

func (s *Service) Logout(ctx context.Context, session Session) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = ?`, session.TokenHash[:]); err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}

func (s *Service) ValidOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return s.cfg.Environment != config.Production
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSuffix(origin, "/")), []byte(strings.TrimSuffix(s.cfg.AppOrigin, "/"))) == 1
}

func (s *Service) ValidCSRF(session Session, provided string) bool {
	return provided != "" && subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(provided)) == 1
}

type oauthState struct {
	Nonce     string `json:"nonce"`
	ReturnTo  string `json:"returnTo"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (s *Service) signState(state oauthState) (string, error) {
	body, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode oauth state: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(s.cfg.OAuthStateSecret))
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) verifyState(value string) (oauthState, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return oauthState{}, ErrInvalidState
	}
	provided, err := hex.DecodeString(parts[1])
	if err != nil {
		return oauthState{}, ErrInvalidState
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.OAuthStateSecret))
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return oauthState{}, ErrInvalidState
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthState{}, ErrInvalidState
	}
	var state oauthState
	if err := json.Unmarshal(body, &state); err != nil {
		return oauthState{}, ErrInvalidState
	}
	return state, nil
}

func (s *Service) exchangeCode(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", errors.New("github oauth code is empty")
	}
	form := url.Values{
		"client_id": {s.cfg.GitHubClientID}, "client_secret": {s.cfg.GitHubClientSecret},
		"code": {code}, "redirect_uri": {s.cfg.GitHubCallbackURL},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange github oauth code: %w", err)
	}
	defer response.Body.Close()
	var payload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode github oauth response: %w", err)
	}
	if response.StatusCode != http.StatusOK || payload.AccessToken == "" {
		return "", fmt.Errorf("github oauth exchange failed: status %d, code %s", response.StatusCode, payload.Error)
	}
	return payload.AccessToken, nil
}

func (s *Service) fetchGitHubUser(ctx context.Context, accessToken string) (AdminUser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return AdminUser{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := s.client.Do(request)
	if err != nil {
		return AdminUser{}, fmt.Errorf("get github user: %w", err)
	}
	defer response.Body.Close()
	var payload struct {
		ID        uint64 `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return AdminUser{}, fmt.Errorf("decode github user: %w", err)
	}
	if response.StatusCode != http.StatusOK || payload.ID == 0 || payload.Login == "" {
		return AdminUser{}, fmt.Errorf("github user request failed with status %d", response.StatusCode)
	}
	if strings.TrimSpace(payload.Name) == "" {
		payload.Name = payload.Login
	}
	return AdminUser{GitHubID: payload.ID, Login: payload.Login, Name: payload.Name, AvatarURL: payload.AvatarURL}, nil
}

func (s *Service) createSession(ctx context.Context, user AdminUser) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	csrf := s.csrfToken(token)
	tokenHash, csrfHash := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(csrf))
	now := s.now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_sessions
		(token_hash, csrf_token_hash, github_id, github_login, display_name, avatar_url, expires_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, tokenHash[:], csrfHash[:], user.GitHubID, user.Login, user.Name, user.AvatarURL, now.Add(s.cfg.SessionTTL), now); err != nil {
		return Session{}, fmt.Errorf("create admin session: %w", err)
	}
	return Session{User: user, CSRFToken: csrf, TokenHash: tokenHash, rawToken: token}, nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func sanitizeReturnTo(value string) string {
	if value == "/admin" || strings.HasPrefix(value, "/admin/") || strings.HasPrefix(value, "/admin?") {
		if !strings.HasPrefix(value, "/admin/login") && !strings.ContainsAny(value, "\r\n") {
			return value
		}
	}
	return "/admin"
}

func parseSessionCookie(value string) (string, bool) {
	if len(value) < 32 || strings.ContainsAny(value, ".=/+ ") {
		return "", false
	}
	return value, true
}

func (s *Service) csrfToken(sessionToken string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.OAuthStateSecret))
	_, _ = mac.Write([]byte("csrf:" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
