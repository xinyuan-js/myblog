package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/example/myblog/apps/api/internal/blog"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type publicStore interface {
	SiteProfile(context.Context) (blog.SiteProfile, error)
	PublicTags(context.Context) ([]blog.Tag, error)
	PublicCategories(context.Context) ([]blog.Category, error)
	ListPublicPosts(context.Context, blog.PublicPostQuery) (blog.PostPage, error)
	PublicPost(context.Context, string) (blog.PostDetail, error)
}

type publicHandler struct {
	store  publicStore
	logger *slog.Logger
}

func (h publicHandler) site(c *gin.Context) {
	profile, err := h.store.SiteProfile(c.Request.Context())
	if err != nil {
		h.internalError(c, "get site profile", err)
		return
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		h.internalError(c, "encode site profile", err)
		return
	}
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(encoded))
	c.Header("Cache-Control", "public, max-age=0, must-revalidate")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": profile})
}

func (h publicHandler) tags(c *gin.Context) {
	items, err := h.store.PublicTags(c.Request.Context())
	if err != nil {
		h.internalError(c, "list public tags", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=0, must-revalidate")
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h publicHandler) categories(c *gin.Context) {
	items, err := h.store.PublicCategories(c.Request.Context())
	if err != nil {
		h.internalError(c, "list public categories", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=0, must-revalidate")
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h publicHandler) posts(c *gin.Context) {
	page, ok := positiveQueryInt(c, "page", 1, 1_000_000)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInt(c, "pageSize", 10, 100)
	if !ok {
		return
	}
	tag := strings.TrimSpace(c.Query("tag"))
	category := strings.TrimSpace(c.Query("category"))
	search := strings.TrimSpace(c.Query("q"))
	if (tag != "" && !slugPattern.MatchString(tag)) || (category != "" && !slugPattern.MatchString(category)) {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "标签或分类 Slug 格式不正确")
		return
	}
	if utf8.RuneCountInString(search) > 100 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "搜索关键词不能超过 100 个字符")
		return
	}
	if search != "" && utf8.RuneCountInString(search) < 2 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "搜索关键词至少需要 2 个字符")
		return
	}
	result, err := h.store.ListPublicPosts(c.Request.Context(), blog.PublicPostQuery{
		Page: page, PageSize: pageSize, TagSlug: tag, CategorySlug: category, Search: search,
	})
	if err != nil {
		h.internalError(c, "list public posts", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h publicHandler) post(c *gin.Context) {
	slug := c.Param("slug")
	if !slugPattern.MatchString(slug) || len(slug) > 160 {
		writeError(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在或尚未发布")
		return
	}
	item, err := h.store.PublicPost(c.Request.Context(), slug)
	if errors.Is(err, blog.ErrNotFound) {
		writeError(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在或尚未发布")
		return
	}
	if err != nil {
		h.internalError(c, "get public post", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h publicHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}

func positiveQueryInt(c *gin.Context, name string, fallback, maximum int) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", name+" 参数不正确")
		return 0, false
	}
	return value, true
}
