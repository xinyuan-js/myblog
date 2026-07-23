package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrInvalidArtalkToken     = errors.New("invalid artalk identity token")
	ErrArtalkUserMismatch     = errors.New("artalk token does not belong to the current blog user")
	ErrArtalkSessionInvalid   = errors.New("artalk session is missing or invalid")
	ErrArtalkIdentityConflict = errors.New("artalk identity conflicts with another github user")
)

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

func (s *Service) ValidateArtalkSession(ctx context.Context, token string, session Session) error {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 4096 || strings.ContainsAny(token, " \t\r\n") || session.User.Email == "" {
		return ErrArtalkSessionInvalid
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, s.cfg.ArtalkInternalURL+"/api/v2/user", nil,
	)
	if err != nil {
		return fmt.Errorf("create Artalk session validation request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("validate Artalk session: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		if response.StatusCode >= 400 && response.StatusCode < 500 {
			return ErrArtalkSessionInvalid
		}
		return fmt.Errorf("validate Artalk session: status %d", response.StatusCode)
	}
	var payload struct {
		User *struct {
			Email string `json:"email"`
		} `json:"user"`
		IsLogin bool `json:"is_login"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return fmt.Errorf("decode Artalk session validation: %w", err)
	}
	if !payload.IsLogin || payload.User == nil || strings.TrimSpace(payload.User.Email) == "" {
		return ErrArtalkSessionInvalid
	}
	if !strings.EqualFold(strings.TrimSpace(payload.User.Email), strings.TrimSpace(session.User.Email)) {
		return ErrArtalkUserMismatch
	}
	return nil
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
	s.roleMu.Lock()
	defer s.roleMu.Unlock()
	if s.artalkDB == nil {
		return ArtalkSession{}, errors.New("artalk database is not configured")
	}
	isAdmin, isOwner, err := s.authorization(ctx, session.User.GitHubID)
	if err != nil {
		return ArtalkSession{}, err
	}
	session.User.IsAdmin, session.User.IsOwner = isAdmin, isOwner
	if err := s.CheckCommentAccess(ctx, session); err != nil {
		return ArtalkSession{}, err
	}
	// Validate the complete identity before mutating the mapped Artalk user.
	identityToken, err := s.IssueArtalkToken(session)
	if err != nil {
		return ArtalkSession{}, err
	}
	mappedArtalkUserID, err := s.prepareArtalkIdentity(ctx, session.User.GitHubID, session.User.Email)
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
	if mappedArtalkUserID != 0 && mappedArtalkUserID != artalkUser.ID {
		return ArtalkSession{}, ErrArtalkIdentityConflict
	}
	if err := s.bindArtalkIdentity(ctx, session.User.GitHubID, artalkUser.ID); err != nil {
		return ArtalkSession{}, err
	}
	update, err := s.artalkDB.ExecContext(ctx, `
		UPDATE users
		SET is_admin = ?, updated_at = UTC_TIMESTAMP(3)
		WHERE id = ? AND deleted_at IS NULL
	`, session.User.IsAdmin, artalkUser.ID)
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("synchronize artalk role: %w", err)
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return ArtalkSession{}, fmt.Errorf("read synchronized Artalk role count: %w", err)
	}
	if affected == 0 {
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

func (s *Service) prepareArtalkIdentity(ctx context.Context, githubID uint64, email string) (uint64, error) {
	var mappedID sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `
		SELECT artalk_user_id
		FROM github_users
		WHERE github_id = ?
	`, githubID).Scan(&mappedID); err != nil {
		return 0, fmt.Errorf("read Artalk identity mapping: %w", err)
	}
	if !mappedID.Valid || mappedID.Int64 <= 0 {
		return 0, nil
	}
	artalkUserID := uint64(mappedID.Int64)
	email = strings.ToLower(strings.TrimSpace(email))

	var conflict bool
	if err := s.artalkDB.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE LOWER(email) = ? AND id <> ? AND deleted_at IS NULL
		)
	`, email, artalkUserID).Scan(&conflict); err != nil {
		return 0, fmt.Errorf("check Artalk email ownership: %w", err)
	}
	if conflict {
		return 0, ErrArtalkIdentityConflict
	}
	result, err := s.artalkDB.ExecContext(ctx, `
		UPDATE users
		SET email = ?, updated_at = UTC_TIMESTAMP(3)
		WHERE id = ? AND deleted_at IS NULL
	`, email, artalkUserID)
	if err != nil {
		return 0, fmt.Errorf("update stable Artalk identity: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read updated Artalk identity count: %w", err)
	}
	if affected == 0 {
		var exists bool
		if err := s.artalkDB.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM users WHERE id = ? AND deleted_at IS NULL)
		`, artalkUserID).Scan(&exists); err != nil {
			return 0, fmt.Errorf("verify stable Artalk identity: %w", err)
		}
		if !exists {
			return 0, errors.New("mapped Artalk user no longer exists")
		}
	}
	return artalkUserID, nil
}

func (s *Service) bindArtalkIdentity(ctx context.Context, githubID, artalkUserID uint64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE github_users
		SET artalk_user_id = ?
		WHERE github_id = ? AND (artalk_user_id IS NULL OR artalk_user_id = ?)
	`, artalkUserID, githubID, artalkUserID)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrArtalkIdentityConflict
		}
		return fmt.Errorf("bind Artalk identity: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read bound Artalk identity count: %w", err)
	}
	if affected > 0 {
		return nil
	}
	var existing sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `
		SELECT artalk_user_id FROM github_users WHERE github_id = ?
	`, githubID).Scan(&existing); err != nil {
		return fmt.Errorf("verify Artalk identity mapping: %w", err)
	}
	if !existing.Valid || existing.Int64 <= 0 || uint64(existing.Int64) != artalkUserID {
		return ErrArtalkIdentityConflict
	}
	return nil
}

func (s *Service) signArtalkToken(encoded string) []byte {
	mac := hmac.New(sha256.New, []byte(s.cfg.OAuthStateSecret))
	_, _ = mac.Write([]byte("artalk-sso:" + encoded))
	return mac.Sum(nil)
}
