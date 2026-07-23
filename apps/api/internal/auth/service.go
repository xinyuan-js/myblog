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
	"sync"
	"time"

	"github.com/example/myblog/apps/api/internal/config"
)

var (
	ErrNotConfigured   = errors.New("github oauth is not configured")
	ErrInvalidState    = errors.New("invalid oauth state")
	ErrUnauthenticated = errors.New("session is missing or invalid")
)

const oauthStateCookie = "blog_oauth_state"

type User struct {
	GitHubID  uint64 `json:"githubId"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"-"`
	AvatarURL string `json:"avatarUrl"`
	IsAdmin   bool   `json:"isAdmin"`
	IsOwner   bool   `json:"isOwner"`
}

type Session struct {
	User      User
	CSRFToken string
	TokenHash [32]byte
	rawToken  string
}

type Service struct {
	db       *sql.DB
	artalkDB *sql.DB
	cfg      config.Config
	client   *http.Client
	now      func() time.Time
	roleMu   sync.Mutex
}

func NewService(db *sql.DB, cfg config.Config) *Service {
	return &Service{db: db, cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}, now: time.Now}
}

func (s *Service) SetArtalkDatabase(db *sql.DB) { s.artalkDB = db }

func (s *Service) Configured() bool {
	return usableOAuthCredential(s.cfg.GitHubClientID) && usableOAuthCredential(s.cfg.GitHubClientSecret) &&
		s.cfg.GitHubAdminID != 0 && len(s.cfg.OAuthStateSecret) >= 32
}

func usableOAuthCredential(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value != "" && !strings.Contains(value, "placeholder") &&
		!strings.HasPrefix(value, "replace-") && !strings.HasPrefix(value, "change-me")
}

func (s *Service) SessionCookieName() string { return s.cfg.SessionCookieName }

func (s *Service) ArtalkInternalURL() string { return s.cfg.ArtalkInternalURL }

func (s *Service) AppURL(path string) string {
	return strings.TrimSuffix(s.cfg.AppOrigin, "/") + path
}

func (s *Service) BeginLogin(ctx context.Context, returnTo string) (authorizeURL string, cookie *http.Cookie, err error) {
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
	if _, err := s.db.ExecContext(ctx, `
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
		"scope":        {"read:user user:email"},
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
	user.IsAdmin, user.IsOwner, err = s.authorization(ctx, user.GitHubID)
	if err != nil {
		return Session{}, "", err
	}
	if !user.IsAdmin && strings.HasPrefix(state.ReturnTo, "/admin") {
		state.ReturnTo = "/"
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
		Name: s.cfg.SessionCookieName, Value: value, Path: "/", MaxAge: int(s.cfg.SessionTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.SessionSecure, SameSite: http.SameSiteLaxMode,
	}, nil
}

func (s *Service) ClearSessionCookie() *http.Cookie {
	return &http.Cookie{Name: s.cfg.SessionCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SessionSecure, SameSite: http.SameSiteLaxMode}
}

func (s *Service) ClearLegacySessionCookie() *http.Cookie {
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
	var refreshLastSeen bool
	err := s.db.QueryRowContext(ctx, `
		SELECT session.github_id, session_user.github_login, session_user.display_name,
		       session_user.email, session_user.avatar_url, session.csrf_token_hash,
		       session.last_seen_at <= UTC_TIMESTAMP(6) - INTERVAL 5 MINUTE AS refresh_last_seen
		FROM user_sessions AS session
		INNER JOIN github_users AS session_user ON session_user.github_id = session.github_id
		WHERE session.token_hash = ? AND session.expires_at > UTC_TIMESTAMP(6)
	`, tokenHash[:]).Scan(
		&githubID, &session.User.Login, &session.User.Name, &session.User.Email, &session.User.AvatarURL,
		&storedCSRFHash, &refreshLastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("query user session: %w", err)
	}
	if len(storedCSRFHash) != len(csrfHash) || subtle.ConstantTimeCompare(storedCSRFHash, csrfHash[:]) != 1 {
		return Session{}, ErrUnauthenticated
	}
	if githubID == 0 || session.User.Email == "" {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE token_hash = ?`, tokenHash[:])
		return Session{}, ErrUnauthenticated
	}
	session.User.GitHubID = githubID
	// Resolve authorization on every request so grants and revocations take
	// effect immediately without forcing users to sign in again.
	session.User.IsAdmin, session.User.IsOwner, err = s.authorization(ctx, githubID)
	if err != nil {
		return Session{}, err
	}
	session.CSRFToken = csrf
	session.TokenHash = tokenHash
	session.rawToken = token
	if refreshLastSeen {
		// Avoid turning every authenticated read (especially Artalk polling)
		// into a database write. The predicate also makes concurrent refreshes
		// idempotent after the first request advances the timestamp.
		_, _ = s.db.ExecContext(ctx, `
			UPDATE user_sessions
			SET last_seen_at = UTC_TIMESTAMP(6)
			WHERE token_hash = ? AND last_seen_at <= UTC_TIMESTAMP(6) - INTERVAL 5 MINUTE
		`, tokenHash[:])
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, session Session) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE token_hash = ?`, session.TokenHash[:]); err != nil {
		return fmt.Errorf("delete user session: %w", err)
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

func (s *Service) fetchGitHubUser(ctx context.Context, accessToken string) (User, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return User{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := s.client.Do(request)
	if err != nil {
		return User{}, fmt.Errorf("get github user: %w", err)
	}
	defer response.Body.Close()
	var payload struct {
		ID        uint64 `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return User{}, fmt.Errorf("decode github user: %w", err)
	}
	if response.StatusCode != http.StatusOK || payload.ID == 0 || payload.Login == "" {
		return User{}, fmt.Errorf("github user request failed with status %d", response.StatusCode)
	}
	if strings.TrimSpace(payload.Name) == "" {
		payload.Name = payload.Login
	}
	email, err := s.fetchVerifiedGitHubEmail(ctx, accessToken)
	if err != nil {
		return User{}, err
	}
	return User{
		GitHubID: payload.ID, Login: payload.Login, Name: payload.Name,
		Email: email, AvatarURL: payload.AvatarURL,
	}, nil
}

func (s *Service) fetchVerifiedGitHubEmail(ctx context.Context, accessToken string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("get github emails: %w", err)
	}
	defer response.Body.Close()
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode github emails: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails request failed with status %d", response.StatusCode)
	}
	for _, candidate := range emails {
		if candidate.Primary && candidate.Verified && strings.TrimSpace(candidate.Email) != "" {
			return strings.ToLower(strings.TrimSpace(candidate.Email)), nil
		}
	}
	for _, candidate := range emails {
		if candidate.Verified && strings.TrimSpace(candidate.Email) != "" {
			return strings.ToLower(strings.TrimSpace(candidate.Email)), nil
		}
	}
	return "", errors.New("github account has no verified email")
}

func (s *Service) createSession(ctx context.Context, user User) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	csrf := s.csrfToken(token)
	tokenHash, csrfHash := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(csrf))
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, fmt.Errorf("begin user session: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO github_users (github_id, github_login, display_name, email, avatar_url)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			github_login = VALUES(github_login),
			display_name = VALUES(display_name),
			email = VALUES(email),
			avatar_url = VALUES(avatar_url)
	`, user.GitHubID, user.Login, user.Name, user.Email, user.AvatarURL); err != nil {
		return Session{}, fmt.Errorf("remember github user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_sessions
		(token_hash, csrf_token_hash, github_id, github_login, display_name, email, avatar_url, expires_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tokenHash[:], csrfHash[:], user.GitHubID, user.Login, user.Name, user.Email, user.AvatarURL, now.Add(s.cfg.SessionTTL), now); err != nil {
		return Session{}, fmt.Errorf("create user session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Session{}, fmt.Errorf("commit user session: %w", err)
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
	if value == "" || strings.ContainsAny(value, "\r\n") || strings.HasPrefix(value, "//") {
		return "/"
	}
	target, err := url.ParseRequestURI(value)
	if err != nil || target.IsAbs() || target.Host != "" || !strings.HasPrefix(target.Path, "/") ||
		target.Path == "/login" || strings.HasPrefix(target.Path, "/admin/login") {
		return "/"
	}
	return value
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
