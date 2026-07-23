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
	GitHubID  uint64     `json:"githubId"`
	IsOwner   bool       `json:"isOwner"`
	GrantedAt *time.Time `json:"grantedAt"`
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT github_id, granted_at
		FROM delegated_admins
		ORDER BY granted_at ASC, github_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list administrators: %w", err)
	}
	defer rows.Close()

	items := []Administrator{{GitHubID: s.cfg.GitHubAdminID, IsOwner: true}}
	for rows.Next() {
		var item Administrator
		var grantedAt time.Time
		if err := rows.Scan(&item.GitHubID, &grantedAt); err != nil {
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

func (s *Service) syncArtalkAdministrator(ctx context.Context, githubID uint64, isAdmin bool) error {
	if s.artalkDB == nil {
		return nil
	}
	var email string
	err := s.db.QueryRowContext(ctx,
		`SELECT email FROM github_users WHERE github_id = ?`,
		githubID,
	).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		// The user has not signed in yet, so no Artalk identity exists to sync.
		return nil
	}
	if err != nil {
		return fmt.Errorf("read administrator identity: %w", err)
	}
	if _, err := s.artalkDB.ExecContext(ctx, `
		UPDATE users
		SET is_admin = ?, updated_at = UTC_TIMESTAMP(3)
		WHERE email = ? AND deleted_at IS NULL
	`, isAdmin, email); err != nil {
		return fmt.Errorf("synchronize Artalk administrator: %w", err)
	}
	return nil
}
