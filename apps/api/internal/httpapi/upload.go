package httpapi

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/example/myblog/apps/api/internal/upload"
	"github.com/gin-gonic/gin"
)

type uploadHandler struct {
	service *upload.Service
	logger  *slog.Logger
}

const maxConcurrentUploads = 2

var errInvalidUploadForm = errors.New("invalid upload multipart form")

func (h uploadHandler) create(c *gin.Context) {
	session, ok := adminSession(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录管理端")
		return
	}
	input, err := decodeUploadInput(c, upload.MaxSize)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) || errors.Is(err, upload.ErrTooLarge) {
			writeError(c, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE", "图片不能超过 10 MiB")
		} else {
			writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "上传请求必须且只能包含一个 file 文件")
		}
		return
	}
	item, err := h.service.Create(c.Request.Context(), input, session.User.GitHubID)
	if h.writeUploadError(c, err) {
		return
	}
	c.Header("Location", "/api/admin/uploads/"+strconv.FormatInt(item.ID, 10))
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func decodeUploadInput(c *gin.Context, maximum int64) (upload.Input, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximum+(1<<20))
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return upload.Input{}, errInvalidUploadForm
	}
	part, err := reader.NextPart()
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return upload.Input{}, err
		}
		return upload.Input{}, errInvalidUploadForm
	}
	if part.FormName() != "file" || part.FileName() == "" {
		_ = part.Close()
		return upload.Input{}, errInvalidUploadForm
	}
	filename := part.FileName()
	declaredContentType := part.Header.Get("Content-Type")

	var body bytes.Buffer
	if expected := c.Request.ContentLength; expected > 0 {
		if expected > maximum+1 {
			expected = maximum + 1
		}
		body.Grow(int(expected))
	}
	_, readError := io.Copy(&body, io.LimitReader(part, maximum+1))
	closeError := part.Close()
	if readError != nil {
		return upload.Input{}, readError
	}
	if closeError != nil {
		return upload.Input{}, closeError
	}
	if int64(body.Len()) > maximum {
		return upload.Input{}, upload.ErrTooLarge
	}

	extra, err := reader.NextPart()
	if err == nil {
		_ = extra.Close()
		return upload.Input{}, errInvalidUploadForm
	}
	if !errors.Is(err, io.EOF) {
		return upload.Input{}, err
	}
	return upload.Input{
		Filename:            filename,
		DeclaredContentType: declaredContentType,
		Body:                body.Bytes(),
	}, nil
}

func (h uploadHandler) list(c *gin.Context) {
	page, ok := positiveQueryInt(c, "page", 1, 1_000)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInt(c, "pageSize", 20, 100)
	if !ok {
		return
	}
	status := strings.TrimSpace(c.DefaultQuery("status", "active"))
	usage := strings.TrimSpace(c.DefaultQuery("usage", "all"))
	search := strings.TrimSpace(c.Query("q"))
	if (status != "active" && status != "trashed") || (usage != "all" && usage != "used" && usage != "unused") || utf8.RuneCountInString(search) > 100 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "媒体筛选参数不正确")
		return
	}
	result, err := h.service.List(c.Request.Context(), upload.ListQuery{Page: page, PageSize: pageSize, Status: status, Usage: usage, Search: search})
	if err != nil {
		h.internalError(c, "list uploads", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h uploadHandler) get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if h.writeUploadError(c, err) {
		return
	}
	if item.Status == "uploading" {
		writeError(c, http.StatusNotFound, "UPLOAD_NOT_FOUND", "媒体不存在")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": item})
}
func (h uploadHandler) trash(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if h.writeUploadError(c, h.service.Trash(c.Request.Context(), id)) {
		return
	}
	c.Status(http.StatusNoContent)
}
func (h uploadHandler) restore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.service.Restore(c.Request.Context(), id)
	if h.writeUploadError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}
func (h uploadHandler) permanent(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if h.writeUploadError(c, h.service.DeletePermanent(c.Request.Context(), id)) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h uploadHandler) writeUploadError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, upload.ErrTooLarge):
		writeError(c, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE", "图片不能超过 10 MiB")
	case errors.Is(err, upload.ErrUnsupportedType):
		writeError(c, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "只允许有效的 JPEG、PNG、WebP 或 GIF 图片")
	case errors.Is(err, upload.ErrNotFound):
		writeError(c, http.StatusNotFound, "UPLOAD_NOT_FOUND", "媒体不存在")
	case errors.Is(err, upload.ErrInUse):
		writeError(c, http.StatusConflict, "UPLOAD_IN_USE", "媒体正在被文章或站点设置使用")
	case errors.Is(err, upload.ErrInvalidState):
		writeError(c, http.StatusConflict, "UPLOAD_STATE_INVALID", "媒体当前状态不允许此操作")
	default:
		h.internalError(c, "manage upload", err)
	}
	return true
}

func (h uploadHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}
