package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrOwnerManagedByConfig = errors.New("the site owner is managed by configuration")
	ErrAdministratorMissing = errors.New("administrator does not exist")
)

type Administrator struct {
	GitHubID    uint64     `json:"githubId"`
	Login       string     `json:"login"`
	Name        string     `json:"name"`
	AvatarURL   string     `json:"avatarUrl"`
	HasSignedIn bool       `json:"hasSignedIn"`
	IsOwner     bool       `json:"isOwner"`
	GrantedAt   *time.Time `json:"grantedAt"`
}

func (s *Service) authorization(ctx context.Context, githubID uint64) (isAdmin, isOwner bool, err error) {
	if githubID == s.cfg.GitHubAdminID {
		return true, true, nil
	}
	var delegated bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM delegated_admins WHERE github_id = ?)`,
		githubID,
	).Scan(&delegated); err != nil {
		return false, false, fmt.Errorf("query administrator permission: %w", err)
	}
	return delegated, false, nil
}

func (s *Service) ListAdministrators(ctx context.Context) ([]Administrator, error) {
	owner := Administrator{GitHubID: s.cfg.GitHubAdminID, IsOwner: true}
	err := s.db.QueryRowContext(ctx, `
		SELECT github_login, display_name, avatar_url
		FROM github_users
		WHERE github_id = ?
	`, s.cfg.GitHubAdminID).Scan(&owner.Login, &owner.Name, &owner.AvatarURL)
	switch {
	case err == nil:
		owner.HasSignedIn = true
	case errors.Is(err, sql.ErrNoRows):
	default:
		return nil, fmt.Errorf("read owner identity: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.github_id, d.granted_at,
		       COALESCE(u.github_login, ''), COALESCE(u.display_name, ''), COALESCE(u.avatar_url, ''),
		       (u.github_id IS NOT NULL)
		FROM delegated_admins d
		LEFT JOIN github_users u ON u.github_id = d.github_id
		WHERE d.github_id <> ?
		ORDER BY d.granted_at ASC, d.github_id ASC
	`, s.cfg.GitHubAdminID)
	if err != nil {
		return nil, fmt.Errorf("list administrators: %w", err)
	}
	defer rows.Close()

	items := []Administrator{owner}
	for rows.Next() {
		var item Administrator
		var grantedAt time.Time
		if err := rows.Scan(
			&item.GitHubID, &grantedAt, &item.Login, &item.Name, &item.AvatarURL, &item.HasSignedIn,
		); err != nil {
			return nil, fmt.Errorf("scan administrator: %w", err)
		}
		item.GrantedAt = &grantedAt
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate administrators: %w", err)
	}
	return items, nil
}

func (s *Service) AddAdministrator(ctx context.Context, githubID, grantedBy uint64) (Administrator, error) {
	s.roleMu.Lock()
	defer s.roleMu.Unlock()
	if githubID == 0 {
		return Administrator{}, errors.New("github id is required")
	}
	if githubID == s.cfg.GitHubAdminID {
		return Administrator{}, ErrOwnerManagedByConfig
	}
	var existed bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM delegated_admins WHERE github_id = ?)`,
		githubID,
	).Scan(&existed); err != nil {
		return Administrator{}, fmt.Errorf("query administrator before addition: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO delegated_admins (github_id, granted_by)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE granted_by = VALUES(granted_by)
	`, githubID, grantedBy); err != nil {
		return Administrator{}, fmt.Errorf("add administrator: %w", err)
	}
	if err := s.syncArtalkAdministrator(ctx, githubID, true); err != nil {
		if !existed {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM delegated_admins WHERE github_id = ?`, githubID)
		}
		return Administrator{}, err
	}
	var grantedAt time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT granted_at FROM delegated_admins WHERE github_id = ?`,
		githubID,
	).Scan(&grantedAt); err != nil {
		return Administrator{}, fmt.Errorf("read added administrator: %w", err)
	}
	return Administrator{GitHubID: githubID, GrantedAt: &grantedAt}, nil
}

func (s *Service) RemoveAdministrator(ctx context.Context, githubID uint64) error {
	s.roleMu.Lock()
	defer s.roleMu.Unlock()
	if githubID == s.cfg.GitHubAdminID {
		return ErrOwnerManagedByConfig
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM delegated_admins WHERE github_id = ?)`,
		githubID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("query administrator before removal: %w", err)
	}
	if !exists {
		return ErrAdministratorMissing
	}
	// Revoke Artalk moderation first. If Artalk is unavailable, retain the
	// delegated grant so the two administration surfaces cannot disagree.
	if err := s.syncArtalkAdministrator(ctx, githubID, false); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM delegated_admins WHERE github_id = ?`, githubID)
	if err != nil {
		// The blog grant is still authoritative when its deletion fails. Restore
		// the Artalk role best-effort so the two administration surfaces do not
		// remain split after a transient database error.
		if compensationErr := s.syncArtalkAdministrator(ctx, githubID, true); compensationErr != nil {
			return errors.Join(fmt.Errorf("remove administrator: %w", err), compensationErr)
		}
		return fmt.Errorf("remove administrator: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed administrator count: %w", err)
	}
	if affected == 0 {
		return ErrAdministratorMissing
	}
	return nil
}

func (s *Service) ReconcileArtalkAdministrators(ctx context.Context) error {
	s.roleMu.Lock()
	defer s.roleMu.Unlock()
	if s.artalkDB == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT email, artalk_user_id
		FROM github_users
		WHERE github_id = ? OR github_id IN (SELECT github_id FROM delegated_admins)
	`, s.cfg.GitHubAdminID)
	if err != nil {
		return fmt.Errorf("list authoritative Artalk administrators: %w", err)
	}
	type identity struct {
		email        string
		artalkUserID sql.NullInt64
	}
	identities := make([]identity, 0)
	for rows.Next() {
		var item identity
		if err := rows.Scan(&item.email, &item.artalkUserID); err != nil {
			rows.Close()
			return fmt.Errorf("scan authoritative Artalk administrator: %w", err)
		}
		identities = append(identities, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate authoritative Artalk administrators: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close authoritative administrator rows: %w", err)
	}

	tx, err := s.artalkDB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin Artalk administrator reconciliation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET is_admin = FALSE, updated_at = UTC_TIMESTAMP(3)
		WHERE is_admin = TRUE AND deleted_at IS NULL
	`); err != nil {
		return fmt.Errorf("clear stale Artalk administrators: %w", err)
	}
	for _, identity := range identities {
		var err error
		if identity.artalkUserID.Valid && identity.artalkUserID.Int64 > 0 {
			_, err = tx.ExecContext(ctx, `
				UPDATE users SET is_admin = TRUE, updated_at = UTC_TIMESTAMP(3)
				WHERE id = ? AND deleted_at IS NULL
			`, identity.artalkUserID.Int64)
		} else {
			// Legacy identities are backfilled on their next successful Artalk
			// exchange. Until then, email is the only available association.
			_, err = tx.ExecContext(ctx, `
				UPDATE users SET is_admin = TRUE, updated_at = UTC_TIMESTAMP(3)
				WHERE email = ? AND deleted_at IS NULL
			`, identity.email)
		}
		if err != nil {
			return fmt.Errorf("restore authoritative Artalk administrator: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Artalk administrator reconciliation: %w", err)
	}
	return nil
}

func (s *Service) syncArtalkAdministrator(ctx context.Context, githubID uint64, isAdmin bool) error {
	if s.artalkDB == nil {
		return nil
	}
	var email string
	var artalkUserID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT email, artalk_user_id FROM github_users WHERE github_id = ?`,
		githubID,
	).Scan(&email, &artalkUserID)
	if errors.Is(err, sql.ErrNoRows) {
		// The user has not signed in yet, so no Artalk identity exists to sync.
		return nil
	}
	if err != nil {
		return fmt.Errorf("read administrator identity: %w", err)
	}
	var updateErr error
	if artalkUserID.Valid && artalkUserID.Int64 > 0 {
		_, updateErr = s.artalkDB.ExecContext(ctx, `
			UPDATE users
			SET is_admin = ?, updated_at = UTC_TIMESTAMP(3)
			WHERE id = ? AND deleted_at IS NULL
		`, isAdmin, artalkUserID.Int64)
	} else {
		_, updateErr = s.artalkDB.ExecContext(ctx, `
			UPDATE users
			SET is_admin = ?, updated_at = UTC_TIMESTAMP(3)
			WHERE email = ? AND deleted_at IS NULL
		`, isAdmin, email)
	}
	if updateErr != nil {
		return fmt.Errorf("synchronize Artalk administrator: %w", updateErr)
	}
	return nil
}
