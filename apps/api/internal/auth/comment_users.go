package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrCommentUserMissing   = errors.New("comment user does not exist")
	ErrCommentUserProtected = errors.New("administrators cannot be restricted")
	ErrCommentsBlocked      = errors.New("comments are blocked for this account")
	ErrCommentDailyLimit    = errors.New("daily comment limit reached")
)

type CommentUser struct {
	GitHubID            uint64    `json:"githubId"`
	Login               string    `json:"login"`
	Name                string    `json:"name"`
	AvatarURL           string    `json:"avatarUrl"`
	IsAdmin             bool      `json:"isAdmin"`
	IsOwner             bool      `json:"isOwner"`
	CommentsBlocked     bool      `json:"commentsBlocked"`
	CommentBlockReason  string    `json:"commentBlockReason"`
	DailyLimit          *int      `json:"dailyLimit"`
	EffectiveDailyLimit int       `json:"effectiveDailyLimit"`
	TodayCount          int       `json:"todayCount"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type CommentReservation struct {
	GitHubID uint64
	Date     string
	Reserved bool
}

func (s *Service) ListCommentUsers(ctx context.Context, query string) ([]CommentUser, error) {
	query = strings.TrimSpace(query)
	pattern := "%" + query + "%"
	usageDate := s.commentUsageDate()
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.github_id, u.github_login, u.display_name, u.avatar_url,
		       (u.github_id = ? OR d.github_id IS NOT NULL) AS is_admin,
		       (u.github_id = ?) AS is_owner,
		       u.comments_blocked, u.comment_block_reason, u.comment_daily_limit,
		       COALESCE(du.comment_count, 0), u.created_at, u.updated_at
		FROM github_users u
		LEFT JOIN delegated_admins d ON d.github_id = u.github_id
		LEFT JOIN comment_daily_usage du ON du.github_id = u.github_id AND du.usage_date = ?
		WHERE (? = '' OR u.github_login LIKE ? OR u.display_name LIKE ? OR CAST(u.github_id AS CHAR) LIKE ?)
		ORDER BY u.updated_at DESC, u.github_id DESC
		LIMIT 200
	`, s.cfg.GitHubAdminID, s.cfg.GitHubAdminID, usageDate, query, pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("list comment users: %w", err)
	}
	defer rows.Close()

	items := make([]CommentUser, 0)
	for rows.Next() {
		var item CommentUser
		var dailyLimit sql.NullInt64
		if err := rows.Scan(
			&item.GitHubID, &item.Login, &item.Name, &item.AvatarURL, &item.IsAdmin, &item.IsOwner,
			&item.CommentsBlocked, &item.CommentBlockReason, &dailyLimit, &item.TodayCount,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment user: %w", err)
		}
		item.EffectiveDailyLimit = s.cfg.CommentDailyLimit
		if dailyLimit.Valid {
			value := int(dailyLimit.Int64)
			item.DailyLimit = &value
			item.EffectiveDailyLimit = value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment users: %w", err)
	}
	return items, nil
}

func (s *Service) UpdateCommentPolicy(
	ctx context.Context,
	githubID uint64,
	blocked bool,
	reason string,
	dailyLimit *int,
) (CommentUser, error) {
	if githubID == 0 {
		return CommentUser{}, ErrCommentUserMissing
	}
	isAdmin, _, err := s.authorization(ctx, githubID)
	if err != nil {
		return CommentUser{}, err
	}
	if isAdmin {
		return CommentUser{}, ErrCommentUserProtected
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE github_users
		SET comments_blocked = ?, comment_block_reason = ?, comment_daily_limit = ?
		WHERE github_id = ?
	`, blocked, strings.TrimSpace(reason), dailyLimit, githubID)
	if err != nil {
		return CommentUser{}, fmt.Errorf("update comment policy: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists bool
		if err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM github_users WHERE github_id = ?)`, githubID,
		).Scan(&exists); err != nil {
			return CommentUser{}, fmt.Errorf("verify comment user: %w", err)
		}
		if !exists {
			return CommentUser{}, ErrCommentUserMissing
		}
	}
	items, err := s.ListCommentUsers(ctx, fmt.Sprintf("%d", githubID))
	if err != nil {
		return CommentUser{}, err
	}
	for _, item := range items {
		if item.GitHubID == githubID {
			return item, nil
		}
	}
	return CommentUser{}, ErrCommentUserMissing
}

func (s *Service) CheckCommentAccess(ctx context.Context, session Session) error {
	if session.User.IsAdmin {
		return nil
	}
	blocked, limit, count, err := s.commentPolicy(ctx, session.User.GitHubID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrCommentsBlocked
	}
	if count >= limit {
		return ErrCommentDailyLimit
	}
	return nil
}

func (s *Service) ReserveComment(ctx context.Context, session Session) (CommentReservation, error) {
	if session.User.IsAdmin {
		return CommentReservation{}, nil
	}
	blocked, limit, _, err := s.commentPolicy(ctx, session.User.GitHubID)
	if err != nil {
		return CommentReservation{}, err
	}
	if blocked {
		return CommentReservation{}, ErrCommentsBlocked
	}
	usageDate := s.commentUsageDate()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO comment_daily_usage (github_id, usage_date, comment_count)
		VALUES (?, ?, 1)
		ON DUPLICATE KEY UPDATE
			comment_count = IF(comment_count < ?, comment_count + 1, comment_count)
	`, session.User.GitHubID, usageDate, limit)
	if err != nil {
		return CommentReservation{}, fmt.Errorf("reserve daily comment quota: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return CommentReservation{}, fmt.Errorf("read reserved comment quota: %w", err)
	}
	if affected == 0 {
		return CommentReservation{}, ErrCommentDailyLimit
	}
	return CommentReservation{GitHubID: session.User.GitHubID, Date: usageDate, Reserved: true}, nil
}

func (s *Service) ReleaseCommentReservation(ctx context.Context, reservation CommentReservation) {
	if !reservation.Reserved {
		return
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE comment_daily_usage
		SET comment_count = GREATEST(comment_count - 1, 0)
		WHERE github_id = ? AND usage_date = ?
	`, reservation.GitHubID, reservation.Date)
}

func (s *Service) commentPolicy(ctx context.Context, githubID uint64) (blocked bool, limit, count int, err error) {
	var customLimit sql.NullInt64
	err = s.db.QueryRowContext(ctx, `
		SELECT u.comments_blocked, u.comment_daily_limit, COALESCE(du.comment_count, 0)
		FROM github_users u
		LEFT JOIN comment_daily_usage du ON du.github_id = u.github_id AND du.usage_date = ?
		WHERE u.github_id = ?
	`, s.commentUsageDate(), githubID).Scan(&blocked, &customLimit, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, 0, ErrCommentUserMissing
	}
	if err != nil {
		return false, 0, 0, fmt.Errorf("read comment policy: %w", err)
	}
	limit = s.cfg.CommentDailyLimit
	if customLimit.Valid {
		limit = int(customLimit.Int64)
	}
	return blocked, limit, count, nil
}

func (s *Service) commentUsageDate() string {
	zone := time.FixedZone("comment-day", s.cfg.CommentDayOffset*60*60)
	return s.now().In(zone).Format("2006-01-02")
}
