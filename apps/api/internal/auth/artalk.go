package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidArtalkToken = errors.New("invalid artalk identity token")

const (
	artalkTokenPurpose = "artalk-sso"
	artalkTokenTTL     = 5 * time.Minute
)

type artalkClaims struct {
	Purpose   string `json:"purpose"`
	GitHubID  uint64 `json:"githubId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Login     string `json:"login"`
	ExpiresAt int64  `json:"expiresAt"`
}

type ArtalkUserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Nickname      string `json:"nickname"`
}

type ArtalkSession struct {
	Token string          `json:"token"`
	User  json.RawMessage `json:"user"`
}

func (s *Service) IssueArtalkToken(session Session) (string, error) {
	if session.User.GitHubID == 0 || session.User.Email == "" || session.User.Login == "" {
		return "", ErrUnauthenticated
	}
	claims := artalkClaims{
		Purpose: artalkTokenPurpose, GitHubID: session.User.GitHubID,
		Email: session.User.Email, Name: session.User.Name, Login: session.User.Login,
		ExpiresAt: s.now().UTC().Add(artalkTokenTTL).Unix(),
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode artalk identity: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	signature := s.signArtalkToken(encoded)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *Service) VerifyArtalkToken(value string) (ArtalkUserInfo, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return ArtalkUserInfo{}, ErrInvalidArtalkToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, s.signArtalkToken(parts[0])) {
		return ArtalkUserInfo{}, ErrInvalidArtalkToken
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ArtalkUserInfo{}, ErrInvalidArtalkToken
	}
	var claims artalkClaims
	if err := json.Unmarshal(body, &claims); err != nil {
		return ArtalkUserInfo{}, ErrInvalidArtalkToken
	}
	now := s.now().UTC()
	if claims.Purpose != artalkTokenPurpose || claims.GitHubID == 0 || claims.Email == "" || claims.Login == "" ||
		claims.ExpiresAt < now.Unix() || claims.ExpiresAt > now.Add(artalkTokenTTL+time.Minute).Unix() {
		return ArtalkUserInfo{}, ErrInvalidArtalkToken
	}
	name := strings.TrimSpace(claims.Name)
	if name == "" {
		name = claims.Login
	}
	return ArtalkUserInfo{
		Subject: strconv.FormatUint(claims.GitHubID, 10), Email: strings.ToLower(claims.Email),
		EmailVerified: true, Name: name, Nickname: claims.Login,
	}, nil
}

func (s *Service) CreateArtalkSession(ctx context.Context, session Session) (ArtalkSession, error) {
	if s.artalkDB == nil {
		return ArtalkSession{}, errors.New("artalk database is not configured")
	}
	if err := s.CheckCommentAccess(ctx, session); err != nil {
		return ArtalkSession{}, err
	}
	identityToken, err := s.IssueArtalkToken(session)
	if err != nil {
		return ArtalkSession{}, err
	}
	body, err := json.Marshal(map[string]string{"token": identityToken})
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("encode artalk exchange: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, s.cfg.ArtalkInternalURL+"/api/v2/sso/exchange", bytes.NewReader(body),
	)
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("create artalk exchange request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("exchange artalk identity: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ArtalkSession{}, fmt.Errorf("artalk identity exchange failed with status %d", response.StatusCode)
	}
	var result ArtalkSession
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return ArtalkSession{}, fmt.Errorf("decode artalk session: %w", err)
	}
	if result.Token == "" || len(result.User) == 0 {
		return ArtalkSession{}, errors.New("artalk identity exchange returned an incomplete session")
	}
	var artalkUser struct {
		ID uint64 `json:"id"`
	}
	if err := json.Unmarshal(result.User, &artalkUser); err != nil || artalkUser.ID == 0 {
		return ArtalkSession{}, errors.New("artalk identity exchange returned an invalid user")
	}
	update, err := s.artalkDB.ExecContext(ctx, `
		UPDATE users
		SET is_admin = ?, updated_at = UTC_TIMESTAMP(3)
		WHERE id = ? AND deleted_at IS NULL
	`, session.User.IsAdmin, artalkUser.ID)
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("synchronize artalk role: %w", err)
	}
	if affected, _ := update.RowsAffected(); affected == 0 {
		// MySQL reports zero changed rows when the role already has the desired
		// value. Distinguish that normal case from a missing/deleted user.
		var exists bool
		if err := s.artalkDB.QueryRowContext(
			ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = ? AND deleted_at IS NULL)`, artalkUser.ID,
		).Scan(&exists); err != nil {
			return ArtalkSession{}, fmt.Errorf("verify synchronized artalk role: %w", err)
		}
		if !exists {
			return ArtalkSession{}, errors.New("synchronize artalk role: user no longer exists")
		}
	}
	var userPayload map[string]any
	if err := json.Unmarshal(result.User, &userPayload); err != nil {
		return ArtalkSession{}, fmt.Errorf("decode synchronized artalk user: %w", err)
	}
	// The exchange response was created before the database role update. Return
	// the authoritative role immediately so moderation works on the first load.
	userPayload["is_admin"] = session.User.IsAdmin
	result.User, err = json.Marshal(userPayload)
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("encode synchronized artalk user: %w", err)
	}
	return result, nil
}

func (s *Service) signArtalkToken(encoded string) []byte {
	mac := hmac.New(sha256.New, []byte(s.cfg.OAuthStateSecret))
	_, _ = mac.Write([]byte("artalk-sso:" + encoded))
	return mac.Sum(nil)
}
