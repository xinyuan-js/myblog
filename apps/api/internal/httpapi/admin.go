package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/example/myblog/apps/api/internal/blog"
)

var markdownSyntax = regexp.MustCompile(`(?m)^[#>*+\-]+\s*|[` + "`" + `*_~\[\]()]`)

type adminStore interface {
	ListAdminPosts(context.Context, blog.AdminPostQuery) (blog.PostPage, error)
	AdminPost(context.Context, int64) (blog.PostDetail, error)
	CreatePost(context.Context, blog.PostMutation, uint) (blog.PostDetail, error)
	UpdatePost(context.Context, int64, blog.PostMutation, uint) (blog.PostDetail, error)
	DeletePost(context.Context, int64) error
	AdminTags(context.Context) ([]blog.Tag, error)
	AdminCategories(context.Context) ([]blog.Category, error)
	CreateTag(context.Context, blog.TaxonomyMutation) (blog.Tag, error)
	UpdateTag(context.Context, int64, blog.TaxonomyMutation) (blog.Tag, error)
	DeleteTag(context.Context, int64) error
	CreateCategory(context.Context, blog.TaxonomyMutation) (blog.Category, error)
	UpdateCategory(context.Context, int64, blog.TaxonomyMutation) (blog.Category, error)
	DeleteCategory(context.Context, int64) error
	UpdateSiteAppearance(context.Context, blog.SiteAppearanceMutation) (blog.SiteProfile, error)
}

type adminHandler struct {
	store  adminStore
	logger *slog.Logger
}

func (h adminHandler) posts(c *gin.Context) {
	page, ok := positiveQueryInt(c, "page", 1, 1_000_000)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInt(c, "pageSize", 20, 100)
	if !ok {
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && status != "draft" && status != "published" && status != "scheduled" {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "status 参数不正确")
		return
	}
	pageResult, err := h.store.ListAdminPosts(c.Request.Context(), blog.AdminPostQuery{Page: page, PageSize: pageSize, Status: status})
	if err != nil {
		h.internalError(c, "list admin posts", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": pageResult})
}

func (h adminHandler) post(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.store.AdminPost(c.Request.Context(), id)
	if errors.Is(err, blog.ErrNotFound) {
		writeError(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}
	if err != nil {
		h.internalError(c, "get admin post", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h adminHandler) createPost(c *gin.Context) {
	mutation, fields, ok := decodePostMutation(c)
	if !ok {
		return
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.store.CreatePost(c.Request.Context(), mutation, countWords(mutation.ContentMarkdown))
	if h.writePostStoreError(c, err) {
		return
	}
	c.Header("Location", "/api/admin/posts/"+strconv.FormatInt(item.ID, 10))
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h adminHandler) updatePost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	mutation, fields, ok := decodePostMutation(c)
	if !ok {
		return
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.store.UpdatePost(c.Request.Context(), id, mutation, countWords(mutation.ContentMarkdown))
	if h.writePostStoreError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h adminHandler) deletePost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	err := h.store.DeletePost(c.Request.Context(), id)
	if errors.Is(err, blog.ErrNotFound) {
		writeError(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
		return
	}
	if err != nil {
		h.internalError(c, "delete post", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandler) tags(c *gin.Context) {
	items, err := h.store.AdminTags(c.Request.Context())
	if err != nil {
		h.internalError(c, "list admin tags", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h adminHandler) categories(c *gin.Context) {
	items, err := h.store.AdminCategories(c.Request.Context())
	if err != nil {
		h.internalError(c, "list admin categories", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h adminHandler) createTag(c *gin.Context) {
	mutation, fields, ok := decodeTaxonomy(c, false)
	if !ok {
		return
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.store.CreateTag(c.Request.Context(), mutation)
	if h.writeTaxonomyError(c, err, "TAG_NOT_FOUND") {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h adminHandler) updateTag(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	mutation, fields, ok := decodeTaxonomy(c, false)
	if !ok {
		return
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.store.UpdateTag(c.Request.Context(), id, mutation)
	if h.writeTaxonomyError(c, err, "TAG_NOT_FOUND") {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h adminHandler) deleteTag(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if h.writeTaxonomyError(c, h.store.DeleteTag(c.Request.Context(), id), "TAG_NOT_FOUND") {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandler) createCategory(c *gin.Context) {
	mutation, fields, ok := decodeTaxonomy(c, true)
	if !ok {
		return
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.store.CreateCategory(c.Request.Context(), mutation)
	if h.writeTaxonomyError(c, err, "CATEGORY_NOT_FOUND") {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h adminHandler) updateCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	mutation, fields, ok := decodeTaxonomy(c, true)
	if !ok {
		return
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.store.UpdateCategory(c.Request.Context(), id, mutation)
	if h.writeTaxonomyError(c, err, "CATEGORY_NOT_FOUND") {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h adminHandler) deleteCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if h.writeTaxonomyError(c, h.store.DeleteCategory(c.Request.Context(), id), "CATEGORY_NOT_FOUND") {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h adminHandler) updateSiteAppearance(c *gin.Context) {
	var mutation blog.SiteAppearanceMutation
	if !decodeJSONBody(c, &mutation, 1<<20) {
		return
	}
	mutation.Title = strings.TrimSpace(mutation.Title)
	mutation.Subtitle = strings.TrimSpace(mutation.Subtitle)
	mutation.Description = strings.TrimSpace(mutation.Description)
	mutation.AuthorName = strings.TrimSpace(mutation.AuthorName)
	mutation.AuthorBio = strings.TrimSpace(mutation.AuthorBio)
	fields := map[string]string{}
	if length := utf8.RuneCountInString(mutation.Title); length < 1 || length > 120 {
		fields["title"] = "站点标题长度必须为 1～120 个字符"
	}
	if utf8.RuneCountInString(mutation.Subtitle) > 200 {
		fields["subtitle"] = "副标题不能超过 200 个字符"
	}
	if utf8.RuneCountInString(mutation.Description) > 500 {
		fields["description"] = "站点描述不能超过 500 个字符"
	}
	if length := utf8.RuneCountInString(mutation.AuthorName); length < 1 || length > 120 {
		fields["authorName"] = "作者名称长度必须为 1～120 个字符"
	}
	if utf8.RuneCountInString(mutation.AuthorBio) > 500 {
		fields["authorBio"] = "作者简介不能超过 500 个字符"
	}
	if strings.TrimSpace(mutation.AboutMarkdown) == "" || len(mutation.AboutMarkdown) > 1<<20 {
		fields["aboutMarkdown"] = "About 内容不能为空且不能超过 1 MiB"
	}
	if mutation.AvatarURL != nil && !validMediaURL(*mutation.AvatarURL) {
		fields["avatarUrl"] = "头像必须使用本站媒体库 URL"
	}
	if mutation.BannerURL != nil && !validMediaURL(*mutation.BannerURL) {
		fields["bannerUrl"] = "Banner 必须使用本站媒体库 URL"
	}
	if len(mutation.SocialLinks) > 10 {
		fields["socialLinks"] = "社交链接最多设置 10 个"
	}
	seenLinks := map[string]bool{}
	for index := range mutation.SocialLinks {
		link := &mutation.SocialLinks[index]
		link.Label, link.URL, link.Icon = strings.TrimSpace(link.Label), strings.TrimSpace(link.URL), strings.TrimSpace(link.Icon)
		if utf8.RuneCountInString(link.Label) < 1 || utf8.RuneCountInString(link.Label) > 50 ||
			len(link.URL) > 2048 || !validSocialURL(link.URL) ||
			(link.Icon != "github" && link.Icon != "mail" && link.Icon != "rss" && link.Icon != "link") ||
			seenLinks[link.URL] {
			fields["socialLinks"] = "社交链接的名称、URL 或图标不正确，且 URL 不能重复"
			break
		}
		seenLinks[link.URL] = true
	}
	if mutation.ICPNumber != nil {
		value := strings.TrimSpace(*mutation.ICPNumber)
		if value == "" {
			mutation.ICPNumber = nil
		} else if utf8.RuneCountInString(value) > 100 {
			fields["icpNumber"] = "备案号不能超过 100 个字符"
		} else {
			mutation.ICPNumber = &value
		}
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	profile, err := h.store.UpdateSiteAppearance(c.Request.Context(), mutation)
	if errors.Is(err, blog.ErrInvalidMedia) {
		writeValidationError(c, map[string]string{"avatarUrl": "媒体不存在、已删除或不属于本站"})
		return
	}
	if err != nil {
		h.internalError(c, "update site appearance", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": profile})
}

func validSocialURL(value string) bool {
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return false
	}
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return true
	}
	if parsed.Scheme != "mailto" || parsed.Opaque == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	address, err := mail.ParseAddress(parsed.Opaque)
	return err == nil && address.Address == parsed.Opaque
}

func (h adminHandler) writePostStoreError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, blog.ErrNotFound):
		writeError(c, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
	case errors.Is(err, blog.ErrSlugConflict):
		writeErrorFields(c, http.StatusConflict, "SLUG_CONFLICT", "Slug 已被使用", map[string]string{"slug": "Slug 已被使用"})
	case errors.Is(err, blog.ErrSlugLocked):
		writeError(c, http.StatusConflict, "POST_SLUG_LOCKED", "文章发布后不能修改 Slug")
	case errors.Is(err, blog.ErrInvalidTaxonomy):
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", "分类或标签不存在")
	case errors.Is(err, blog.ErrInvalidMedia):
		writeValidationError(c, map[string]string{"coverUrl": "媒体不存在、已删除或不属于本站"})
	default:
		h.internalError(c, "save post", err)
	}
	return true
}

func (h adminHandler) writeTaxonomyError(c *gin.Context, err error, notFoundCode string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, blog.ErrNotFound):
		writeError(c, http.StatusNotFound, notFoundCode, "资源不存在")
	case errors.Is(err, blog.ErrSlugConflict):
		writeError(c, http.StatusConflict, "SLUG_CONFLICT", "名称或 Slug 已被使用")
	case errors.Is(err, blog.ErrTaxonomyInUse):
		writeError(c, http.StatusConflict, "TAXONOMY_IN_USE", "该分类或标签仍被文章使用")
	default:
		h.internalError(c, "mutate taxonomy", err)
	}
	return true
}

func (h adminHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}

func decodePostMutation(c *gin.Context) (blog.PostMutation, map[string]string, bool) {
	var value blog.PostMutation
	if !decodeJSONBody(c, &value, 2<<20) {
		return value, nil, false
	}
	value.Title, value.Slug, value.Excerpt = strings.TrimSpace(value.Title), strings.TrimSpace(value.Slug), strings.TrimSpace(value.Excerpt)
	fields := map[string]string{}
	if utf8.RuneCountInString(value.Title) < 1 || utf8.RuneCountInString(value.Title) > 200 {
		fields["title"] = "标题长度必须为 1～200 个字符"
	}
	if len(value.Slug) < 1 || len(value.Slug) > 160 || !slugPattern.MatchString(value.Slug) {
		fields["slug"] = "Slug 只能包含小写字母、数字和连字符"
	}
	if utf8.RuneCountInString(value.Excerpt) > 500 {
		fields["excerpt"] = "摘要不能超过 500 个字符"
	}
	if strings.TrimSpace(value.ContentMarkdown) == "" || len(value.ContentMarkdown) > 2<<20 {
		fields["contentMarkdown"] = "正文不能为空且不能超过 2 MiB"
	}
	if value.Status != "draft" && value.Status != "published" && value.Status != "scheduled" {
		fields["status"] = "文章状态不正确"
	}
	if value.Status == "scheduled" && (value.PublishedAt == nil || !value.PublishedAt.After(time.Now().UTC())) {
		fields["publishedAt"] = "定时发布时间必须晚于当前时间"
	}
	if value.CategoryID != nil && *value.CategoryID < 1 {
		fields["categoryId"] = "分类 ID 不正确"
	}
	if len(value.TagIDs) > 20 {
		fields["tagIds"] = "一篇文章最多使用 20 个标签"
	}
	seen := map[int64]bool{}
	for _, id := range value.TagIDs {
		if id < 1 || seen[id] {
			fields["tagIds"] = "标签 ID 必须为不重复的正整数"
		}
		seen[id] = true
	}
	if value.CoverURL != nil && !validMediaURL(*value.CoverURL) {
		fields["coverUrl"] = "封面必须使用本站媒体库 URL"
	}
	return value, fields, true
}

func decodeTaxonomy(c *gin.Context, category bool) (blog.TaxonomyMutation, map[string]string, bool) {
	var value blog.TaxonomyMutation
	if !decodeJSONBody(c, &value, 4096) {
		return value, nil, false
	}
	value.Name, value.Slug = strings.TrimSpace(value.Name), strings.TrimSpace(value.Slug)
	fields := map[string]string{}
	if utf8.RuneCountInString(value.Name) < 1 || utf8.RuneCountInString(value.Name) > 80 {
		fields["name"] = "名称长度必须为 1～80 个字符"
	}
	if len(value.Slug) < 1 || len(value.Slug) > 80 || !slugPattern.MatchString(value.Slug) {
		fields["slug"] = "Slug 格式不正确"
	}
	if category && value.Description != nil {
		trimmed := strings.TrimSpace(*value.Description)
		value.Description = &trimmed
		if utf8.RuneCountInString(trimmed) > 300 {
			fields["description"] = "描述不能超过 300 个字符"
		}
	}
	return value, fields, true
}

func decodeJSONBody(c *gin.Context, destination any, maximum int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximum)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "JSON 请求体格式不正确")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "请求体只能包含一个 JSON 对象")
		return false
	}
	return true
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "资源 ID 不正确")
		return 0, false
	}
	return id, true
}

func countWords(markdown string) uint {
	plain := markdownSyntax.ReplaceAllString(markdown, " ")
	var count uint
	for _, r := range plain {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

func validMediaURL(value string) bool {
	return value != "" && len(value) <= 2048 && !strings.ContainsAny(value, "\r\n") && !strings.Contains(value, "..")
}

func writeValidationError(c *gin.Context, fields map[string]string) {
	writeErrorFields(c, http.StatusBadRequest, "VALIDATION_FAILED", "提交内容不符合要求", fields)
}
