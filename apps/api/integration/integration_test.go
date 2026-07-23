package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/example/myblog/apps/api/internal/blog"
	"github.com/example/myblog/apps/api/internal/config"
	"github.com/example/myblog/apps/api/internal/database"
	"github.com/example/myblog/apps/api/internal/upload"
)

type memoryObjects struct{ values map[string][]byte }

func (m *memoryObjects) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	body, err := io.ReadAll(reader)
	if err == nil {
		m.values[key] = body
	}
	return err
}
func (m *memoryObjects) Delete(_ context.Context, key string) error {
	delete(m.values, key)
	return nil
}

func TestMySQLBlogAndMediaLifecycle(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN is not set")
	}
	cfg := config.Config{DatabaseDSN: dsn, DatabaseMaxOpen: 5, DatabaseMaxIdle: 2, DatabaseMaxLife: time.Minute, MediaPublicURL: "/uploads"}
	db, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("second migration must be idempotent: %v", err)
	}
	resetDatabase(t, db)

	store := blog.NewStore(db, "/uploads")
	category, err := store.CreateCategory(context.Background(), blog.TaxonomyMutation{Name: "技术", Slug: "tech"})
	if err != nil {
		t.Fatal(err)
	}
	tag, err := store.CreateTag(context.Background(), blog.TaxonomyMutation{Name: "Go", Slug: "go"})
	if err != nil {
		t.Fatal(err)
	}
	objects := &memoryObjects{values: map[string][]byte{}}
	media := upload.NewService(db, objects, cfg)
	header := pngFileHeader(t)
	asset, err := media.Create(context.Background(), header, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects.values) != 1 || asset.Width != 2 || asset.Height != 2 {
		t.Fatalf("unexpected uploaded asset: %+v", asset)
	}

	now := time.Now().UTC().Add(-time.Minute)
	post, err := store.CreatePost(context.Background(), blog.PostMutation{Title: "第一篇", Slug: "first-post", Excerpt: "摘要", ContentMarkdown: "# 正文\n\n![图](" + asset.URL + ")", CoverURL: &asset.URL, Status: "published", PublishedAt: &now, CategoryID: &category.ID, TagIDs: []int64{tag.ID}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.ListPublicPosts(context.Background(), blog.PublicPostQuery{Page: 1, PageSize: 10})
	if err != nil || page.Pagination.Total != 1 {
		t.Fatalf("public posts = %+v, %v", page, err)
	}
	searchPage, err := store.ListPublicPosts(context.Background(), blog.PublicPostQuery{Page: 1, PageSize: 10, Search: "第一篇"})
	if err != nil || searchPage.Pagination.Total != 1 || len(searchPage.Items) != 1 || searchPage.Items[0].ID != post.ID {
		t.Fatalf("fulltext search = %+v, %v", searchPage, err)
	}
	detail, err := store.PublicPost(context.Background(), post.Slug)
	if err != nil || len(detail.Tags) != 1 {
		t.Fatalf("public post = %+v, %v", detail, err)
	}
	asset, err = media.Get(context.Background(), asset.ID)
	if err != nil || asset.UsageCount != 2 {
		t.Fatalf("media references = %+v, %v", asset, err)
	}
	if err := media.Trash(context.Background(), asset.ID); !errors.Is(err, upload.ErrInUse) {
		t.Fatalf("trash referenced media error = %v", err)
	}

	future := time.Now().UTC().Add(time.Hour)
	_, err = store.CreatePost(context.Background(), blog.PostMutation{Title: "定时", Slug: "future-post", Excerpt: "", ContentMarkdown: "future", Status: "scheduled", PublishedAt: &future, TagIDs: []int64{}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	page, err = store.ListPublicPosts(context.Background(), blog.PublicPostQuery{Page: 1, PageSize: 10})
	if err != nil || page.Pagination.Total != 1 {
		t.Fatalf("future post leaked: %+v, %v", page, err)
	}
	locked := blog.PostMutation{Title: post.Title, Slug: "changed", Excerpt: post.Excerpt, ContentMarkdown: post.ContentMarkdown, CoverURL: post.CoverURL, Status: "published", PublishedAt: post.PublishedAt, CategoryID: &category.ID, TagIDs: []int64{tag.ID}}
	if _, err := store.UpdatePost(context.Background(), post.ID, locked, 100); !errors.Is(err, blog.ErrSlugLocked) {
		t.Fatalf("slug lock error = %v", err)
	}

	if err := store.DeletePost(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTag(context.Background(), tag.ID); err != nil {
		t.Fatalf("tag should be releasable after its last post is deleted: %v", err)
	}
	if err := store.DeleteCategory(context.Background(), category.ID); err != nil {
		t.Fatalf("category should be releasable after its last post is deleted: %v", err)
	}
	if err := media.Trash(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := media.Restore(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
	currentProfile, err := store.SiteProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	siteMutation := blog.SiteAppearanceMutation{
		Title: currentProfile.Title, Subtitle: currentProfile.Subtitle, Description: currentProfile.Description,
		AvatarURL: &asset.URL, BannerURL: currentProfile.BannerURL,
		AuthorName: currentProfile.AuthorName, AuthorBio: currentProfile.AuthorBio,
		AboutMarkdown: currentProfile.AboutMarkdown, SocialLinks: currentProfile.SocialLinks, ICPNumber: currentProfile.ICPNumber,
	}
	profile, err := store.UpdateSiteAppearance(context.Background(), siteMutation)
	if err != nil || profile.AvatarURL == nil {
		t.Fatalf("site appearance = %+v, %v", profile, err)
	}
	if err := media.Trash(context.Background(), asset.ID); !errors.Is(err, upload.ErrInUse) {
		t.Fatalf("site reference not protected: %v", err)
	}
	siteMutation.AvatarURL = nil
	if _, err := store.UpdateSiteAppearance(context.Background(), siteMutation); err != nil {
		t.Fatal(err)
	}
	if err := media.Trash(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
	if err := media.DeletePermanent(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
	if len(objects.values) != 0 {
		t.Fatal("object was not deleted")
	}
}

func resetDatabase(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`SET FOREIGN_KEY_CHECKS=0;TRUNCATE TABLE upload_references;TRUNCATE TABLE uploads;TRUNCATE TABLE post_tags;TRUNCATE TABLE posts;TRUNCATE TABLE tags;TRUNCATE TABLE categories;TRUNCATE TABLE user_sessions;TRUNCATE TABLE oauth_states;SET FOREIGN_KEY_CHECKS=1;UPDATE site_settings SET avatar_url=NULL,banner_url=NULL WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
}

func pngFileHeader(t *testing.T) *multipart.FileHeader {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var imageBody bytes.Buffer
	if err := png.Encode(&imageBody, picture); err != nil {
		t.Fatal(err)
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", "pixel.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imageBody.Bytes()); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	request := httptest.NewRequest("POST", "/", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	_, header, err := request.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}
	return header
}
