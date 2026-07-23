package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

type commentUserHandler struct {
	service *auth.Service
	logger  *slog.Logger
}

func (h commentUserHandler) list(c *gin.Context) {
	page, ok := positiveQueryInt(c, "page", 1, 1_000)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInt(c, "pageSize", 20, 100)
	if !ok {
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(query) > 100 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "搜索关键词不能超过 100 个字符")
		return
	}
	result, err := h.service.ListCommentUsers(c.Request.Context(), query, page, pageSize)
	if err != nil {
		h.internalError(c, "list comment users", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h commentUserHandler) update(c *gin.Context) {
	githubID, err := strconv.ParseUint(c.Param("githubId"), 10, 64)
	if err != nil || githubID == 0 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "GitHub 用户 ID 不正确")
		return
	}
	var input struct {
		CommentsBlocked    bool   `json:"commentsBlocked"`
		CommentBlockReason string `json:"commentBlockReason"`
		DailyLimit         *int   `json:"dailyLimit"`
	}
	if !decodeJSONBody(c, &input, 4096) {
		return
	}
	input.CommentBlockReason = strings.TrimSpace(input.CommentBlockReason)
	fields := map[string]string{}
	if input.CommentsBlocked && input.CommentBlockReason == "" {
		fields["commentBlockReason"] = "封禁账号时请填写原因"
	}
	if utf8.RuneCountInString(input.CommentBlockReason) > 500 {
		fields["commentBlockReason"] = "封禁原因不能超过 500 个字符"
	}
	if input.DailyLimit != nil && (*input.DailyLimit < 1 || *input.DailyLimit > 1000) {
		fields["dailyLimit"] = "每日额度必须在 1 到 1000 之间"
	}
	if len(fields) > 0 {
		writeValidationError(c, fields)
		return
	}
	item, err := h.service.UpdateCommentPolicy(
		c.Request.Context(), githubID, input.CommentsBlocked, input.CommentBlockReason, input.DailyLimit,
	)
	switch {
	case errors.Is(err, auth.ErrCommentUserMissing):
		writeError(c, http.StatusNotFound, "COMMENT_USER_NOT_FOUND", "用户不存在")
	case errors.Is(err, auth.ErrCommentUserProtected):
		writeError(c, http.StatusConflict, "COMMENT_USER_PROTECTED", "管理员账号不受评论封禁和额度限制")
	case err != nil:
		h.internalError(c, "update comment user", err)
	default:
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"data": item})
	}
}

func (h commentUserHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}
