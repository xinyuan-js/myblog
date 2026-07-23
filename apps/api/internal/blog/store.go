package blog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

var (
	ErrSlugConflict    = errors.New("slug conflict")
	ErrSlugLocked      = errors.New("published post slug is locked")
	ErrTaxonomyInUse   = errors.New("taxonomy is in use")
	ErrInvalidTaxonomy = errors.New("referenced taxonomy does not exist")
	ErrInvalidMedia    = errors.New("referenced media does not exist or is unavailable")
)

type Store struct {
	db             *sql.DB
	mediaPublicURL string
}

func NewStore(db *sql.DB, mediaPublicURL ...string) *Store {
	prefix := "/uploads"
	if len(mediaPublicURL) > 0 && strings.TrimSpace(mediaPublicURL[0]) != "" {
		prefix = strings.TrimSuffix(mediaPublicURL[0], "/")
	}
	return &Store{db: db, mediaPublicURL: prefix}
}

func (s *Store) SiteProfile(ctx context.Context) (SiteProfile, error) {
	var profile SiteProfile
	var avatar, banner, icp sql.NullString
	var socialJSON []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT title, subtitle, description, avatar_url, banner_url,
		       author_name, author_bio, about_markdown, social_links, icp_number
		FROM site_settings WHERE id = 1
	`).Scan(&profile.Title, &profile.Subtitle, &profile.Description, &avatar, &banner,
		&profile.AuthorName, &profile.AuthorBio, &profile.AboutMarkdown, &socialJSON, &icp)
	if errors.Is(err, sql.ErrNoRows) {
		return SiteProfile{}, ErrNotFound
	}
	if err != nil {
		return SiteProfile{}, fmt.Errorf("query site profile: %w", err)
	}
	profile.AvatarURL = nullableString(avatar)
	profile.BannerURL = nullableString(banner)
	profile.ICPNumber = nullableString(icp)
	if err := json.Unmarshal(socialJSON, &profile.SocialLinks); err != nil {
		return SiteProfile{}, fmt.Errorf("decode social links: %w", err)
	}
	if profile.SocialLinks == nil {
		profile.SocialLinks = []SocialLink{}
	}
	return profile, nil
}

func (s *Store) PublicTags(ctx context.Context) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug,
		       COUNT(DISTINCT CASE WHEN p.deleted_at IS NULL AND p.status = 'published'
		         AND p.published_at <= UTC_TIMESTAMP(6) THEN p.id END) AS post_count
		FROM tags t
		LEFT JOIN post_tags pt ON pt.tag_id = t.id
		LEFT JOIN posts p ON p.id = pt.post_id
		GROUP BY t.id, t.name, t.slug
		ORDER BY post_count DESC, t.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query public tags: %w", err)
	}
	defer rows.Close()
	result := make([]Tag, 0)
	for rows.Next() {
		var item Tag
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.PostCount); err != nil {
			return nil, fmt.Errorf("scan public tag: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) PublicCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.slug, c.description,
		       COUNT(DISTINCT CASE WHEN p.deleted_at IS NULL AND p.status = 'published'
		         AND p.published_at <= UTC_TIMESTAMP(6) THEN p.id END) AS post_count
		FROM categories c
		LEFT JOIN posts p ON p.category_id = c.id
		GROUP BY c.id, c.name, c.slug, c.description
		ORDER BY c.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query public categories: %w", err)
	}
	defer rows.Close()
	result := make([]Category, 0)
	for rows.Next() {
		var item Category
		var description sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &description, &item.PostCount); err != nil {
			return nil, fmt.Errorf("scan public category: %w", err)
		}
		item.Description = nullableString(description)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListPublicPosts(ctx context.Context, query PublicPostQuery) (PostPage, error) {
	conditions := []string{
		"p.deleted_at IS NULL",
		"p.status = 'published'",
		"p.published_at <= UTC_TIMESTAMP(6)",
	}
	args := make([]any, 0, 4)
	if query.TagSlug != "" {
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM post_tags fpt JOIN tags ft ON ft.id = fpt.tag_id
			WHERE fpt.post_id = p.id AND ft.slug = ?
		)`)
		args = append(args, query.TagSlug)
	}
	if query.CategorySlug != "" {
		conditions = append(conditions, "c.slug = ?")
		args = append(args, query.CategorySlug)
	}
	if query.Search != "" {
		conditions = append(conditions,
			"MATCH(p.title, p.excerpt, p.content_markdown) AGAINST (?)")
		args = append(args, query.Search)
	}
	where := strings.Join(conditions, " AND ")

	var total int64
	countSQL := `SELECT COUNT(*) FROM posts p LEFT JOIN categories c ON c.id = p.category_id WHERE ` + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return PostPage{}, fmt.Errorf("count public posts: %w", err)
	}

	orderBy := "p.published_at DESC, p.id DESC"
	selectArgs := append([]any{}, args...)
	if query.Search != "" {
		orderBy = "MATCH(p.title, p.excerpt, p.content_markdown) AGAINST (?) DESC, p.published_at DESC, p.id DESC"
		selectArgs = append(selectArgs, query.Search)
	}
	selectArgs = append(selectArgs, query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.title, p.slug, p.excerpt, p.cover_url, p.status,
		       p.published_at, p.updated_at, p.word_count,
		       c.id, c.name, c.slug, c.description
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, selectArgs...)
	if err != nil {
		return PostPage{}, fmt.Errorf("query public posts: %w", err)
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
	return PostPage{Items: items, Pagination: Pagination{
		Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages,
	}}, nil
}

func (s *Store) PublicPost(ctx context.Context, slug string) (PostDetail, error) {
	item, err := s.queryPublicPost(ctx, slug)
	if err != nil {
		return PostDetail{}, err
	}
	items := []PostSummary{item.PostSummary}
	if err := s.loadTags(ctx, items); err != nil {
		return PostDetail{}, err
	}
	item.PostSummary = items[0]
	item.PreviousPost, err = s.publicNeighbor(ctx, *item.PublishedAt, item.ID, true)
	if err != nil {
		return PostDetail{}, err
	}
	item.NextPost, err = s.publicNeighbor(ctx, *item.PublishedAt, item.ID, false)
	if err != nil {
		return PostDetail{}, err
	}
	return item, nil
}

func (s *Store) queryPublicPost(ctx context.Context, slug string) (PostDetail, error) {
	var item PostDetail
	var cover sql.NullString
	var published time.Time
	var categoryID sql.NullInt64
	var categoryName, categorySlug, categoryDescription sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.title, p.slug, p.excerpt, p.cover_url, p.status,
		       p.published_at, p.updated_at, p.word_count, p.content_markdown,
		       c.id, c.name, c.slug, c.description
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.slug = ? AND p.deleted_at IS NULL AND p.status = 'published'
		  AND p.published_at <= UTC_TIMESTAMP(6)
	`, slug).Scan(&item.ID, &item.Title, &item.Slug, &item.Excerpt, &cover, &item.Status,
		&published, &item.UpdatedAt, &item.WordCount, &item.ContentMarkdown,
		&categoryID, &categoryName, &categorySlug, &categoryDescription)
	if errors.Is(err, sql.ErrNoRows) {
		return PostDetail{}, ErrNotFound
	}
	if err != nil {
		return PostDetail{}, fmt.Errorf("query public post: %w", err)
	}
	item.CoverURL = nullableString(cover)
	item.PublishedAt = &published
	item.Status = "published"
	item.Tags = []Tag{}
	item.ReadingTimeMinutes = readingTime(item.WordCount)
	item.Category = categoryFromNull(categoryID, categoryName, categorySlug, categoryDescription)
	return item, nil
}

func (s *Store) publicNeighbor(ctx context.Context, publishedAt time.Time, id int64, newer bool) (*PostLink, error) {
	operator, order := "<", "DESC"
	if newer {
		operator, order = ">", "ASC"
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT title, slug FROM posts
		WHERE deleted_at IS NULL AND status = 'published' AND published_at <= UTC_TIMESTAMP(6)
		  AND (published_at `+operator+` ? OR (published_at = ? AND id `+operator+` ?))
		ORDER BY published_at `+order+`, id `+order+` LIMIT 1
	`, publishedAt, publishedAt, id)
	var link PostLink
	if err := row.Scan(&link.Title, &link.Slug); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("query public post neighbor: %w", err)
	}
	return &link, nil
}

func scanPostSummaries(rows *sql.Rows) ([]PostSummary, error) {
	defer rows.Close()
	items := make([]PostSummary, 0)
	for rows.Next() {
		var item PostSummary
		var cover sql.NullString
		var published sql.NullTime
		var categoryID sql.NullInt64
		var categoryName, categorySlug, categoryDescription sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.Slug, &item.Excerpt, &cover, &item.Status,
			&published, &item.UpdatedAt, &item.WordCount,
			&categoryID, &categoryName, &categorySlug, &categoryDescription); err != nil {
			return nil, fmt.Errorf("scan post summary: %w", err)
		}
		item.CoverURL = nullableString(cover)
		if published.Valid {
			value := published.Time
			item.PublishedAt = &value
		}
		item.Status = effectiveStatus(item.Status, item.PublishedAt)
		item.Tags = []Tag{}
		item.ReadingTimeMinutes = readingTime(item.WordCount)
		item.Category = categoryFromNull(categoryID, categoryName, categorySlug, categoryDescription)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post summaries: %w", err)
	}
	return items, nil
}

func (s *Store) loadTags(ctx context.Context, posts []PostSummary) error {
	if len(posts) == 0 {
		return nil
	}
	placeholders := make([]string, len(posts))
	args := make([]any, len(posts))
	index := make(map[int64]int, len(posts))
	for i := range posts {
		placeholders[i] = "?"
		args[i] = posts[i].ID
		index[posts[i].ID] = i
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pt.post_id, t.id, t.name, t.slug
		FROM post_tags pt JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY t.name ASC
	`, args...)
	if err != nil {
		return fmt.Errorf("query post tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var postID int64
		var tag Tag
		if err := rows.Scan(&postID, &tag.ID, &tag.Name, &tag.Slug); err != nil {
			return fmt.Errorf("scan post tag: %w", err)
		}
		if position, ok := index[postID]; ok {
			posts[position].Tags = append(posts[position].Tags, tag)
		}
	}
	return rows.Err()
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func categoryFromNull(id sql.NullInt64, name, slug, description sql.NullString) *Category {
	if !id.Valid {
		return nil
	}
	return &Category{ID: id.Int64, Name: name.String, Slug: slug.String, Description: nullableString(description)}
}

func effectiveStatus(status string, publishedAt *time.Time) string {
	if status == "published" && publishedAt != nil && publishedAt.After(time.Now().UTC()) {
		return "scheduled"
	}
	return status
}

func readingTime(wordCount uint) uint {
	minutes := (wordCount + 299) / 300
	if minutes < 1 {
		return 1
	}
	return minutes
}
