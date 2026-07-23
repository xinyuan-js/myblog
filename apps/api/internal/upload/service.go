package upload

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/myblog/apps/api/internal/config"
	"github.com/example/myblog/apps/api/internal/storage"
	_ "golang.org/x/image/webp"
)

const (
	MaxSize       = 10 << 20
	maxImageEdge  = 12000
	maxImagePixel = 40_000_000
)

var (
	ErrTooLarge        = errors.New("upload is too large")
	ErrUnsupportedType = errors.New("unsupported media type")
	ErrNotFound        = errors.New("upload not found")
	ErrInUse           = errors.New("upload is in use")
	ErrInvalidState    = errors.New("invalid upload state")
)

type Reference struct {
	ResourceType string `json:"resourceType"`
	ResourceID   *int64 `json:"resourceId"`
	Field        string `json:"field"`
	Label        string `json:"label"`
}

type Item struct {
	ID          int64       `json:"id"`
	URL         string      `json:"url"`
	Filename    string      `json:"filename"`
	ContentType string      `json:"contentType"`
	Size        int64       `json:"size"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Status      string      `json:"status"`
	UsageCount  int         `json:"usageCount"`
	References  []Reference `json:"references"`
	CreatedAt   time.Time   `json:"createdAt"`
	TrashedAt   *time.Time  `json:"trashedAt"`
	StorageKey  string      `json:"-"`
}

type Page struct {
	Items      []Item     `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type ListQuery struct {
	Page, PageSize        int
	Status, Usage, Search string
}

type Input struct {
	Filename            string
	DeclaredContentType string
	Body                []byte
}

type Service struct {
	db        *sql.DB
	objects   storage.ObjectStore
	publicURL string
	now       func() time.Time
}

func NewService(db *sql.DB, objects storage.ObjectStore, cfg config.Config) *Service {
	return &Service{db: db, objects: objects, publicURL: strings.TrimSuffix(cfg.MediaPublicURL, "/"), now: time.Now}
}

func (s *Service) Create(ctx context.Context, input Input, createdBy uint64) (Item, error) {
	filename := filepath.Base(strings.TrimSpace(input.Filename))
	if filename == "." || filename == "" || utf8.RuneCountInString(filename) > 255 {
		return Item{}, ErrUnsupportedType
	}
	body := input.Body
	if len(body) == 0 {
		return Item{}, ErrUnsupportedType
	}
	if len(body) > MaxSize {
		return Item{}, ErrTooLarge
	}

	detected := http.DetectContentType(body[:min(512, len(body))])
	format, extension, contentType, ok := acceptedImage(detected, filepath.Ext(filename))
	if !ok {
		return Item{}, ErrUnsupportedType
	}
	declared := strings.ToLower(strings.TrimSpace(strings.Split(input.DeclaredContentType, ";")[0]))
	if declared != "" && declared != "application/octet-stream" && declared != contentType && !(contentType == "image/jpeg" && declared == "image/jpg") {
		return Item{}, ErrUnsupportedType
	}
	configuration, decodedFormat, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || decodedFormat != format || !validImageDimensions(configuration.Width, configuration.Height) {
		return Item{}, ErrUnsupportedType
	}

	randomName := make([]byte, 16)
	if _, err := rand.Read(randomName); err != nil {
		return Item{}, fmt.Errorf("generate object key: %w", err)
	}
	now := s.now().UTC()
	key := fmt.Sprintf("%04d/%02d/%s%s", now.Year(), now.Month(), hex.EncodeToString(randomName), extension)
	checksum := sha256.Sum256(body)
	publicURL := s.publicURL + "/" + key
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO uploads (storage_key, public_url, original_filename, content_type, size, width, height, sha256, status, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'uploading', ?)
	`, key, publicURL, filename, contentType, len(body), configuration.Width, configuration.Height, checksum[:], createdBy)
	if err != nil {
		return Item{}, fmt.Errorf("record upload: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Item{}, fmt.Errorf("read pending upload id: %w", err)
	}
	if err := s.objects.Put(ctx, key, bytes.NewReader(body), int64(len(body)), contentType); err != nil {
		s.discardPendingUpload(id, key)
		return Item{}, err
	}
	update, err := s.db.ExecContext(ctx, `UPDATE uploads SET status='active' WHERE id=? AND status='uploading'`, id)
	if err == nil {
		if affected, rowsErr := update.RowsAffected(); rowsErr != nil {
			err = rowsErr
		} else if affected != 1 {
			err = errors.New("pending upload state changed unexpectedly")
		}
	}
	if err != nil {
		s.discardPendingUpload(id, key)
		return Item{}, fmt.Errorf("activate upload: %w", err)
	}
	return Item{ID: id, URL: publicURL, Filename: filename, ContentType: contentType, Size: int64(len(body)), Width: configuration.Width,
		Height: configuration.Height, Status: "active", UsageCount: 0, References: []Reference{}, CreatedAt: now, StorageKey: key}, nil
}

func (s *Service) discardPendingUpload(id int64, key string) {
	cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.objects.Delete(cleanupContext, key); err != nil {
		return
	}
	_, _ = s.db.ExecContext(cleanupContext, `DELETE FROM uploads WHERE id=? AND status='uploading'`, id)
}

func validImageDimensions(width, height int) bool {
	if width < 1 || height < 1 || width > maxImageEdge || height > maxImageEdge {
		return false
	}
	return int64(width)*int64(height) <= maxImagePixel
}

func (s *Service) List(ctx context.Context, query ListQuery) (Page, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.Status == "trashed" {
		conditions = append(conditions, "u.status IN ('trashed','deleting')")
	} else {
		conditions = append(conditions, "u.status = ?")
		args = append(args, query.Status)
	}
	if query.Search != "" {
		conditions = append(conditions, "u.original_filename LIKE ?")
		args = append(args, "%"+escapeLike(query.Search)+"%")
	}
	if query.Usage == "used" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM upload_references ur WHERE ur.upload_id=u.id)")
	}
	if query.Usage == "unused" {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM upload_references ur WHERE ur.upload_id=u.id)")
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM uploads u WHERE `+where, args...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count uploads: %w", err)
	}
	selectArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id,u.public_url,u.original_filename,u.content_type,u.size,u.width,u.height,u.status,u.created_at,u.trashed_at,u.storage_key,
		       (SELECT COUNT(*) FROM upload_references ur WHERE ur.upload_id=u.id)
		FROM uploads u WHERE `+where+` ORDER BY u.created_at DESC,u.id DESC LIMIT ? OFFSET ?
	`, selectArgs...)
	if err != nil {
		return Page{}, fmt.Errorf("list uploads: %w", err)
	}
	items, err := scanItems(rows)
	if err != nil {
		return Page{}, err
	}
	if err := s.loadReferences(ctx, items); err != nil {
		return Page{}, err
	}
	totalPages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	return Page{Items: items, Pagination: Pagination{Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages}}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id,u.public_url,u.original_filename,u.content_type,u.size,u.width,u.height,u.status,u.created_at,u.trashed_at,u.storage_key,
		       (SELECT COUNT(*) FROM upload_references ur WHERE ur.upload_id=u.id)
		FROM uploads u WHERE u.id=?
	`, id)
	if err != nil {
		return Item{}, err
	}
	items, err := scanItems(rows)
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrNotFound
	}
	if err := s.loadReferences(ctx, items); err != nil {
		return Item{}, err
	}
	return items[0], nil
}

func (s *Service) Trash(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE uploads SET status='trashed',trashed_at=UTC_TIMESTAMP(6)
		WHERE id=? AND status='active' AND NOT EXISTS (SELECT 1 FROM upload_references ur WHERE ur.upload_id=uploads.id)
	`, id)
	if err != nil {
		return fmt.Errorf("trash upload: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 1 {
		return nil
	}
	item, getErr := s.Get(ctx, id)
	if errors.Is(getErr, ErrNotFound) {
		return ErrNotFound
	}
	if getErr != nil {
		return getErr
	}
	if item.UsageCount > 0 {
		return ErrInUse
	}
	return ErrInvalidState
}

func (s *Service) Restore(ctx context.Context, id int64) (Item, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE uploads SET status='active',trashed_at=NULL WHERE id=? AND status='trashed'`, id)
	if err != nil {
		return Item{}, fmt.Errorf("restore upload: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		if _, e := s.Get(ctx, id); errors.Is(e, ErrNotFound) {
			return Item{}, ErrNotFound
		}
		return Item{}, ErrInvalidState
	}
	return s.Get(ctx, id)
}

func (s *Service) DeletePermanent(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var key, status string
	var refs int
	err = tx.QueryRowContext(ctx, `SELECT storage_key,status,(SELECT COUNT(*) FROM upload_references WHERE upload_id=uploads.id) FROM uploads WHERE id=? FOR UPDATE`, id).Scan(&key, &status, &refs)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if refs > 0 || (status != "trashed" && status != "deleting") {
		return ErrInvalidState
	}
	if status == "trashed" {
		if _, err = tx.ExecContext(ctx, `UPDATE uploads SET status='deleting' WHERE id=?`, id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err = s.objects.Delete(ctx, key); err != nil {
		compensationContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(compensationContext, `UPDATE uploads SET status='trashed' WHERE id=? AND status='deleting'`, id)
		return err
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM uploads WHERE id=? AND status='deleting'`, id); err != nil {
		return fmt.Errorf("delete upload record: %w", err)
	}
	return nil
}

func (s *Service) CleanupTrashed(ctx context.Context, olderThan time.Time, limit int) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM uploads WHERE status IN ('trashed','deleting') AND trashed_at < ? ORDER BY trashed_at ASC LIMIT ?`, olderThan.UTC(), limit)
	if err != nil {
		return fmt.Errorf("list expired trashed uploads: %w", err)
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var cleanupErr error
	for _, id := range ids {
		if err := s.DeletePermanent(ctx, id); err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrInvalidState) {
			// One broken MinIO object must not permanently starve every newer
			// item behind it. Keep the failed row retryable and process the rest
			// of the batch before reporting the aggregate failure.
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete expired upload %d: %w", id, err))
		}
	}
	return cleanupErr
}

func (s *Service) CleanupPending(ctx context.Context, olderThan time.Time, limit int) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,storage_key FROM uploads
		WHERE status='uploading' AND created_at < ?
		ORDER BY created_at ASC LIMIT ?
	`, olderThan.UTC(), limit)
	if err != nil {
		return fmt.Errorf("list interrupted uploads: %w", err)
	}
	type pendingUpload struct {
		id  int64
		key string
	}
	items := make([]pendingUpload, 0)
	for rows.Next() {
		var item pendingUpload
		if err := rows.Scan(&item.id, &item.key); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var cleanupErr error
	for _, item := range items {
		result, err := s.db.ExecContext(ctx, `
			UPDATE uploads SET status='deleting',trashed_at=UTC_TIMESTAMP(6)
			WHERE id=? AND status='uploading'
		`, item.id)
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("claim interrupted upload %d: %w", item.id, err))
			continue
		}
		affected, err := result.RowsAffected()
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("inspect interrupted upload %d claim: %w", item.id, err))
			continue
		}
		if affected == 0 {
			continue
		}
		if err := s.objects.Delete(ctx, item.key); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete interrupted upload %d object: %w", item.id, err))
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM uploads WHERE id=? AND status='deleting'`, item.id); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete interrupted upload %d record: %w", item.id, err))
		}
	}
	return cleanupErr
}

func scanItems(rows *sql.Rows) ([]Item, error) {
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		var trashed sql.NullTime
		if err := rows.Scan(&item.ID, &item.URL, &item.Filename, &item.ContentType, &item.Size, &item.Width, &item.Height, &item.Status, &item.CreatedAt, &trashed, &item.StorageKey, &item.UsageCount); err != nil {
			return nil, err
		}
		if trashed.Valid {
			value := trashed.Time
			item.TrashedAt = &value
		}
		item.References = []Reference{}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadReferences(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	marks := make([]string, len(items))
	args := make([]any, len(items))
	positions := map[int64]int{}
	for i := range items {
		marks[i] = "?"
		args[i] = items[i].ID
		positions[items[i].ID] = i
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ur.upload_id,ur.resource_type,ur.resource_id,ur.field_name,CASE WHEN ur.resource_type LIKE 'site_%' THEN '站点设置' ELSE COALESCE(p.title,'已删除文章') END FROM upload_references ur LEFT JOIN posts p ON p.id=ur.resource_id WHERE ur.upload_id IN (`+strings.Join(marks, ",")+`) ORDER BY ur.id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var uploadID int64
		var ref Reference
		var resourceID sql.NullInt64
		if err := rows.Scan(&uploadID, &ref.ResourceType, &resourceID, &ref.Field, &ref.Label); err != nil {
			return err
		}
		if resourceID.Valid {
			v := resourceID.Int64
			ref.ResourceID = &v
		}
		if i, ok := positions[uploadID]; ok {
			items[i].References = append(items[i].References, ref)
		}
	}
	return rows.Err()
}

func acceptedImage(detected, extension string) (format, canonicalExtension, contentType string, ok bool) {
	extension = strings.ToLower(extension)
	switch detected {
	case "image/jpeg":
		if extension != ".jpg" && extension != ".jpeg" {
			return
		}
		return "jpeg", ".jpg", "image/jpeg", true
	case "image/png":
		if extension != ".png" {
			return
		}
		return "png", ".png", "image/png", true
	case "image/gif":
		if extension != ".gif" {
			return
		}
		return "gif", ".gif", "image/gif", true
	case "image/webp":
		if extension != ".webp" {
			return
		}
		return "webp", ".webp", "image/webp", true
	}
	return
}
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
