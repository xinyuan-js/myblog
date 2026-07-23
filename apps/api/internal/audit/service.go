package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Event struct {
	ID               int64     `json:"id"`
	ActorGitHubID    uint64    `json:"actorGithubId"`
	ActorLogin       string    `json:"actorLogin"`
	Method           string    `json:"method"`
	RequestPath      string    `json:"requestPath"`
	ResponseStatus   int       `json:"responseStatus"`
	RequestID        string    `json:"requestId"`
	ClientIP         string    `json:"clientIp"`
	ResourceLocation string    `json:"resourceLocation"`
	OccurredAt       time.Time `json:"occurredAt"`
}

type Record struct {
	ActorGitHubID    uint64
	ActorLogin       string
	Method           string
	RequestPath      string
	ResponseStatus   int
	RequestID        string
	ClientIP         string
	ResourceLocation string
}

type Query struct {
	Page, PageSize int
	Outcome        string
	Search         string
}

type Page struct {
	Items      []Event    `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(ctx context.Context, record Record) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_audit_events (
			actor_github_id, actor_login, method, request_path, response_status,
			request_id, client_ip, resource_location, occurred_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))
	`, record.ActorGitHubID, record.ActorLogin, record.Method, record.RequestPath,
		record.ResponseStatus, record.RequestID, record.ClientIP, record.ResourceLocation)
	if err != nil {
		return fmt.Errorf("record administrator audit event: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, query Query) (Page, error) {
	conditions := []string{"1=1"}
	args := make([]any, 0, 4)
	switch query.Outcome {
	case "success":
		conditions = append(conditions, "response_status BETWEEN 200 AND 399")
	case "failure":
		conditions = append(conditions, "response_status >= 400")
	}
	if query.Search != "" {
		pattern := "%" + escapeLike(query.Search) + "%"
		conditions = append(conditions, "(actor_login LIKE ? OR request_path LIKE ? OR request_id LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_audit_events WHERE `+where,
		args...,
	).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count administrator audit events: %w", err)
	}
	selectArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor_github_id, actor_login, method, request_path, response_status,
		       request_id, client_ip, resource_location, occurred_at
		FROM admin_audit_events
		WHERE `+where+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, selectArgs...)
	if err != nil {
		return Page{}, fmt.Errorf("list administrator audit events: %w", err)
	}
	defer rows.Close()
	items := make([]Event, 0)
	for rows.Next() {
		var item Event
		if err := rows.Scan(
			&item.ID, &item.ActorGitHubID, &item.ActorLogin, &item.Method, &item.RequestPath,
			&item.ResponseStatus, &item.RequestID, &item.ClientIP, &item.ResourceLocation, &item.OccurredAt,
		); err != nil {
			return Page{}, fmt.Errorf("scan administrator audit event: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate administrator audit events: %w", err)
	}
	totalPages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	return Page{
		Items: items,
		Pagination: Pagination{
			Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages,
		},
	}, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
