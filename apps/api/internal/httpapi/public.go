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

	"github.com/example/myblog/apps/api/internal/blog"
	"github.com/gin-gonic/gin"
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
	if err := writePublicData(c, profile, "public, max-age=0, must-revalidate"); err != nil {
		h.internalError(c, "encode site profile", err)
	}
}

func (h publicHandler) tags(c *gin.Context) {
	items, err := h.store.PublicTags(c.Request.Context())
	if err != nil {
		h.internalError(c, "list public tags", err)
		return
	}
	if err := writePublicData(c, items, "public, max-age=60"); err != nil {
		h.internalError(c, "encode public tags", err)
	}
}

func (h publicHandler) categories(c *gin.Context) {
	items, err := h.store.PublicCategories(c.Request.Context())
	if err != nil {
		h.internalError(c, "list public categories", err)
		return
	}
	if err := writePublicData(c, items, "public, max-age=60"); err != nil {
		h.internalError(c, "encode public categories", err)
	}
}

func (h publicHandler) posts(c *gin.Context) {
	page, ok := positiveQueryInt(c, "page", 1, 1_000)
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
	if err := writePublicData(c, result, "public, max-age=60"); err != nil {
		h.internalError(c, "encode public posts", err)
	}
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
	if err := writePublicData(c, item, "public, max-age=60"); err != nil {
		h.internalError(c, "encode public post", err)
	}
}

func writePublicData(c *gin.Context, value any, cacheControl string) error {
	encoded, err := json.Marshal(gin.H{"data": value})
	if err != nil {
		return err
	}
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(encoded))
	c.Header("Cache-Control", cacheControl)
	c.Header("ETag", etag)
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return nil
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", encoded)
	return nil
}

func etagMatches(header, current string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == current || strings.TrimPrefix(candidate, "W/") == current {
			return true
		}
	}
	return false
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
