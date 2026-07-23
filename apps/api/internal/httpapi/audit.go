package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/myblog/apps/api/internal/audit"
	"github.com/gin-gonic/gin"
)

type auditHandler struct {
	service *audit.Service
	logger  *slog.Logger
}

func (h auditHandler) list(c *gin.Context) {
	page, ok := positiveQueryInt(c, "page", 1, 1_000)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInt(c, "pageSize", 30, 100)
	if !ok {
		return
	}
	outcome := strings.TrimSpace(c.DefaultQuery("outcome", "all"))
	search := strings.TrimSpace(c.Query("q"))
	if outcome != "all" && outcome != "success" && outcome != "failure" {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "审计结果筛选参数不正确")
		return
	}
	if utf8.RuneCountInString(search) > 100 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "审计搜索关键词不能超过 100 个字符")
		return
	}
	result, err := h.service.List(c.Request.Context(), audit.Query{
		Page: page, PageSize: pageSize, Outcome: outcome, Search: search,
	})
	if err != nil {
		h.logger.Error("list administrator audit events", "requestId", requestIDFromContext(c), "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func auditAdminMutations(service *audit.Service, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if service == nil || !isAuditMethod(c.Request.Method) {
			return
		}
		session, ok := adminSession(c)
		if !ok {
			return
		}
		recordContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := service.Record(recordContext, audit.Record{
			ActorGitHubID:    session.User.GitHubID,
			ActorLogin:       auditValue(session.User.Login, 100),
			Method:           c.Request.Method,
			RequestPath:      auditValue(c.Request.URL.Path, 500),
			ResponseStatus:   c.Writer.Status(),
			RequestID:        auditValue(requestIDFromContext(c), 64),
			ClientIP:         auditValue(c.ClientIP(), 45),
			ResourceLocation: auditValue(c.Writer.Header().Get("Location"), 500),
		})
		if err != nil {
			logger.Error("record administrator audit event",
				"requestId", requestIDFromContext(c),
				"actorGithubId", session.User.GitHubID,
				"error", err,
			)
		}
	}
}

func isAuditMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch || method == http.MethodDelete
}

func auditValue(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}
