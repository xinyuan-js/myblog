package blog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func (s *Store) AdminTags(ctx context.Context) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug, COUNT(DISTINCT p.id)
		FROM tags t LEFT JOIN post_tags pt ON pt.tag_id = t.id LEFT JOIN posts p ON p.id = pt.post_id
		GROUP BY t.id, t.name, t.slug ORDER BY t.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query admin tags: %w", err)
	}
	defer rows.Close()
	items := make([]Tag, 0)
	for rows.Next() {
		var item Tag
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.PostCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.slug, c.description, COUNT(DISTINCT p.id)
		FROM categories c LEFT JOIN posts p ON p.category_id = c.id
		GROUP BY c.id, c.name, c.slug, c.description ORDER BY c.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query admin categories: %w", err)
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var item Category
		var description sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &description, &item.PostCount); err != nil {
			return nil, err
		}
		item.Description = nullableString(description)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAdminPosts(ctx context.Context, query AdminPostQuery) (PostPage, error) {
	condition := "p.deleted_at IS NULL"
	if query.Trashed {
		condition = "p.deleted_at IS NOT NULL"
	} else {
		switch query.Status {
		case "draft":
			condition += " AND p.status = 'draft'"
		case "published":
			condition += " AND p.status = 'published' AND p.published_at <= UTC_TIMESTAMP(6)"
		case "scheduled":
			condition += " AND p.status = 'published' AND p.published_at > UTC_TIMESTAMP(6)"
		}
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts p WHERE "+condition).Scan(&total); err != nil {
		return PostPage{}, fmt.Errorf("count admin posts: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.title, p.slug, p.excerpt, p.cover_url, p.status,
		       p.published_at, p.updated_at, p.word_count,
		       c.id, c.name, c.slug, c.description
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id
		WHERE `+condition+` ORDER BY p.updated_at DESC, p.id DESC LIMIT ? OFFSET ?
	`, query.PageSize, (query.Page-1)*query.PageSize)
	if err != nil {
		return PostPage{}, fmt.Errorf("query admin posts: %w", err)
	}
	items, err := scanPostSummaries(rows)
	if err != nil {
		return PostPage{}, err
	}
	if err := s.loadTags(ctx, items); err != nil {
		return PostPage{}, err
	}
	totalPages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	return PostPage{Items: items, Pagination: Pagination{Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages}}, nil
}

func (s *Store) AdminPost(ctx context.Context, id int64) (PostDetail, error) {
	var item PostDetail
	var cover sql.NullString
	var published sql.NullTime
	var categoryID sql.NullInt64
	var categoryName, categorySlug, categoryDescription sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.title, p.slug, p.excerpt, p.cover_url, p.status, p.published_at,
		       p.updated_at, p.word_count, p.content_markdown, c.id, c.name, c.slug, c.description
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = ? AND p.deleted_at IS NULL
	`, id).Scan(&item.ID, &item.Title, &item.Slug, &item.Excerpt, &cover, &item.Status, &published,
		&item.UpdatedAt, &item.WordCount, &item.ContentMarkdown, &categoryID, &categoryName, &categorySlug, &categoryDescription)
	if errors.Is(err, sql.ErrNoRows) {
		return PostDetail{}, ErrNotFound
	}
	if err != nil {
		return PostDetail{}, fmt.Errorf("query admin post: %w", err)
	}
	item.CoverURL = nullableString(cover)
	if published.Valid {
		value := published.Time
		item.PublishedAt = &value
	}
	item.Status = effectiveStatus(item.Status, item.PublishedAt)
	item.Category = categoryFromNull(categoryID, categoryName, categorySlug, categoryDescription)
	item.Tags = []Tag{}
	item.ReadingTimeMinutes = readingTime(item.WordCount)
	items := []PostSummary{item.PostSummary}
	if err := s.loadTags(ctx, items); err != nil {
		return PostDetail{}, err
	}
	item.PostSummary = items[0]
	return item, nil
}

func (s *Store) CreatePost(ctx context.Context, mutation PostMutation, wordCount uint) (PostDetail, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return PostDetail{}, fmt.Errorf("begin create post: %w", err)
	}
	defer tx.Rollback()
	if err := validateTaxonomies(ctx, tx, mutation.CategoryID, mutation.TagIDs); err != nil {
		return PostDetail{}, err
	}
	status, publishedAt := persistentPublication(mutation.Status, mutation.PublishedAt, time.Now().UTC())
	result, err := tx.ExecContext(ctx, `
		INSERT INTO posts (title, slug, excerpt, content_markdown, cover_url, category_id, status, published_at, word_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, mutation.Title, mutation.Slug, mutation.Excerpt, mutation.ContentMarkdown, mutation.CoverURL,
		mutation.CategoryID, status, publishedAt, wordCount)
	if isDuplicate(err) {
		return PostDetail{}, ErrSlugConflict
	}
	if err != nil {
		return PostDetail{}, fmt.Errorf("insert post: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return PostDetail{}, fmt.Errorf("read new post id: %w", err)
	}
	if err := replacePostTags(ctx, tx, id, mutation.TagIDs); err != nil {
		return PostDetail{}, err
	}
	if err := s.syncPostMedia(ctx, tx, id, mutation); err != nil {
		return PostDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return PostDetail{}, fmt.Errorf("commit create post: %w", err)
	}
	return s.AdminPost(ctx, id)
}

func (s *Store) UpdatePost(ctx context.Context, id int64, mutation PostMutation, wordCount uint) (PostDetail, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return PostDetail{}, fmt.Errorf("begin update post: %w", err)
	}
	defer tx.Rollback()
	var oldSlug, oldStatus string
	var oldPublished sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT slug, status, published_at FROM posts WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, id).
		Scan(&oldSlug, &oldStatus, &oldPublished)
	if errors.Is(err, sql.ErrNoRows) {
		return PostDetail{}, ErrNotFound
	}
	if err != nil {
		return PostDetail{}, fmt.Errorf("lock post: %w", err)
	}
	if oldSlug != mutation.Slug && oldStatus == "published" && oldPublished.Valid && !oldPublished.Time.After(time.Now().UTC()) {
		return PostDetail{}, ErrSlugLocked
	}
	if err := validateTaxonomies(ctx, tx, mutation.CategoryID, mutation.TagIDs); err != nil {
		return PostDetail{}, err
	}
	status, publishedAt := persistentPublication(mutation.Status, mutation.PublishedAt, time.Now().UTC())
	_, err = tx.ExecContext(ctx, `
		UPDATE posts SET title=?, slug=?, excerpt=?, content_markdown=?, cover_url=?, category_id=?,
		status=?, published_at=?, word_count=? WHERE id=?
	`, mutation.Title, mutation.Slug, mutation.Excerpt, mutation.ContentMarkdown, mutation.CoverURL,
		mutation.CategoryID, status, publishedAt, wordCount, id)
	if isDuplicate(err) {
		return PostDetail{}, ErrSlugConflict
	}
	if err != nil {
		return PostDetail{}, fmt.Errorf("update post: %w", err)
	}
	if err := replacePostTags(ctx, tx, id, mutation.TagIDs); err != nil {
		return PostDetail{}, err
	}
	if err := s.syncPostMedia(ctx, tx, id, mutation); err != nil {
		return PostDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return PostDetail{}, fmt.Errorf("commit update post: %w", err)
	}
	return s.AdminPost(ctx, id)
}

func (s *Store) DeletePost(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE posts SET deleted_at=UTC_TIMESTAMP(6) WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete post: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RestorePost(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE posts SET deleted_at=NULL WHERE id=? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("restore post: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePostPermanent(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin permanent post delete: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_references WHERE resource_type IN ('post_cover','post_content') AND resource_id=?`, id); err != nil {
		return fmt.Errorf("release post media references: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM posts WHERE id=? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("permanent delete post: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit permanent post delete: %w", err)
	}
	return nil
}

func (s *Store) UpdateSiteAppearance(ctx context.Context, mutation SiteAppearanceMutation) (SiteProfile, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return SiteProfile{}, err
	}
	defer tx.Rollback()
	var locked int
	if err := tx.QueryRowContext(ctx, `SELECT id FROM site_settings WHERE id=1 FOR UPDATE`).Scan(&locked); err != nil {
		return SiteProfile{}, err
	}
	if err := s.syncSiteMedia(ctx, tx, mutation); err != nil {
		return SiteProfile{}, err
	}
	socialJSON, err := json.Marshal(mutation.SocialLinks)
	if err != nil {
		return SiteProfile{}, fmt.Errorf("encode social links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE site_settings
		SET title=?,subtitle=?,description=?,avatar_url=?,banner_url=?,
		    author_name=?,author_bio=?,about_markdown=?,social_links=?,icp_number=?,public_security_record_number=?
		WHERE id=1
	`, mutation.Title, mutation.Subtitle, mutation.Description, mutation.AvatarURL, mutation.BannerURL,
		mutation.AuthorName, mutation.AuthorBio, mutation.AboutMarkdown, socialJSON, mutation.ICPNumber,
		mutation.PublicSecurityRecordNumber); err != nil {
		return SiteProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return SiteProfile{}, err
	}
	return s.SiteProfile(ctx)
}

func (s *Store) CreateTag(ctx context.Context, mutation TaxonomyMutation) (Tag, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO tags (name, slug) VALUES (?, ?)`, mutation.Name, mutation.Slug)
	if isDuplicate(err) {
		return Tag{}, ErrSlugConflict
	}
	if err != nil {
		return Tag{}, fmt.Errorf("create tag: %w", err)
	}
	id, _ := result.LastInsertId()
	return Tag{ID: id, Name: mutation.Name, Slug: mutation.Slug, PostCount: 0}, nil
}

func (s *Store) UpdateTag(ctx context.Context, id int64, mutation TaxonomyMutation) (Tag, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE tags SET name=?, slug=? WHERE id=?`, mutation.Name, mutation.Slug, id)
	if isDuplicate(err) {
		return Tag{}, ErrSlugConflict
	}
	if err != nil {
		return Tag{}, fmt.Errorf("update tag: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int
		if s.db.QueryRowContext(ctx, `SELECT 1 FROM tags WHERE id=?`, id).Scan(&exists) != nil {
			return Tag{}, ErrNotFound
		}
	}
	items, err := s.AdminTags(ctx)
	if err != nil {
		return Tag{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return Tag{}, ErrNotFound
}

func (s *Store) DeleteTag(ctx context.Context, id int64) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_tags WHERE tag_id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrTaxonomyInUse
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM tags WHERE id=?`, id)
	if isForeignKeyInUse(err) {
		return ErrTaxonomyInUse
	}
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateCategory(ctx context.Context, mutation TaxonomyMutation) (Category, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO categories (name, slug, description) VALUES (?, ?, ?)`, mutation.Name, mutation.Slug, mutation.Description)
	if isDuplicate(err) {
		return Category{}, ErrSlugConflict
	}
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}
	id, _ := result.LastInsertId()
	return Category{ID: id, Name: mutation.Name, Slug: mutation.Slug, Description: mutation.Description, PostCount: 0}, nil
}

func (s *Store) UpdateCategory(ctx context.Context, id int64, mutation TaxonomyMutation) (Category, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE categories SET name=?, slug=?, description=? WHERE id=?`, mutation.Name, mutation.Slug, mutation.Description, id)
	if isDuplicate(err) {
		return Category{}, ErrSlugConflict
	}
	if err != nil {
		return Category{}, fmt.Errorf("update category: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int
		if s.db.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id=?`, id).Scan(&exists) != nil {
			return Category{}, ErrNotFound
		}
	}
	items, err := s.AdminCategories(ctx)
	if err != nil {
		return Category{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return Category{}, ErrNotFound
}

func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts WHERE category_id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrTaxonomyInUse
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id=?`, id)
	if isForeignKeyInUse(err) {
		return ErrTaxonomyInUse
	}
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func validateTaxonomies(ctx context.Context, tx *sql.Tx, categoryID *int64, tagIDs []int64) error {
	if categoryID != nil {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id=?`, *categoryID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidTaxonomy
		} else if err != nil {
			return err
		}
	}
	if len(tagIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(tagIDs)), ",")
	args := make([]any, len(tagIDs))
	for i := range tagIDs {
		args[i] = tagIDs[i]
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tags WHERE id IN (`+placeholders+`)`, args...).Scan(&count); err != nil {
		return err
	}
	if count != len(tagIDs) {
		return ErrInvalidTaxonomy
	}
	return nil
}

func replacePostTags(ctx context.Context, tx *sql.Tx, postID int64, tagIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM post_tags WHERE post_id=?`, postID); err != nil {
		return fmt.Errorf("clear post tags: %w", err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO post_tags (post_id, tag_id) VALUES (?, ?)`, postID, tagID); err != nil {
			return fmt.Errorf("insert post tag: %w", err)
		}
	}
	return nil
}

func persistentPublication(status string, publishedAt *time.Time, now time.Time) (string, *time.Time) {
	if status == "draft" {
		return "draft", publishedAt
	}
	if publishedAt == nil {
		value := now
		return "published", &value
	}
	value := publishedAt.UTC()
	return "published", &value
}

func isDuplicate(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func isForeignKeyInUse(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1451
}

func (s *Store) syncPostMedia(ctx context.Context, tx *sql.Tx, postID int64, mutation PostMutation) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_references WHERE resource_type IN ('post_cover','post_content') AND resource_id=?`, postID); err != nil {
		return err
	}
	if mutation.CoverURL != nil {
		if err := s.insertMediaReference(ctx, tx, *mutation.CoverURL, "post_cover", &postID, "coverUrl", true); err != nil {
			return err
		}
	}
	for _, url := range markdownImageURLs(mutation.ContentMarkdown) {
		if err := s.insertMediaReference(ctx, tx, url, "post_content", &postID, "contentMarkdown", false); err != nil {
			return err
		}
	}
	return nil
}

func markdownImageURLs(markdown string) []string {
	source := []byte(markdown)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	urls := make([]string, 0)
	seen := make(map[string]struct{})
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		image, ok := node.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}
		url := string(util.UnescapePunctuations(image.Destination))
		if url == "" {
			return ast.WalkContinue, nil
		}
		if _, exists := seen[url]; exists {
			return ast.WalkContinue, nil
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
		return ast.WalkContinue, nil
	})
	return urls
}

func (s *Store) syncSiteMedia(ctx context.Context, tx *sql.Tx, mutation SiteAppearanceMutation) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_references WHERE resource_type IN ('site_avatar','site_banner','site_about')`); err != nil {
		return err
	}
	if mutation.AvatarURL != nil {
		if err := s.insertMediaReference(ctx, tx, *mutation.AvatarURL, "site_avatar", nil, "avatarUrl", true); err != nil {
			return err
		}
	}
	if mutation.BannerURL != nil {
		if err := s.insertMediaReference(ctx, tx, *mutation.BannerURL, "site_banner", nil, "bannerUrl", true); err != nil {
			return err
		}
	}
	for _, imageURL := range markdownImageURLs(mutation.AboutMarkdown) {
		if err := s.insertMediaReference(ctx, tx, imageURL, "site_about", nil, "aboutMarkdown", false); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) insertMediaReference(ctx context.Context, tx *sql.Tx, url, resourceType string, resourceID *int64, field string, required bool) error {
	local := strings.HasPrefix(url, s.mediaPublicURL+"/")
	if !local && !required {
		return nil
	}
	if !local {
		return ErrInvalidMedia
	}
	var uploadID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM uploads WHERE public_url=? AND status='active'`, url).Scan(&uploadID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidMedia
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO upload_references(upload_id,resource_type,resource_id,field_name) VALUES(?,?,?,?)`, uploadID, resourceType, resourceID, field)
	return err
}
