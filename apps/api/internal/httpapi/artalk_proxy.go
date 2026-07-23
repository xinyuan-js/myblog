package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

type artalkProxyHandler struct {
	service *auth.Service
	logger  *slog.Logger
	target  *url.URL
}

var legacyArtalkVotePath = regexp.MustCompile(`^/api/v2/votes/(comment|page)_(up|down)/([0-9]+)$`)

func newArtalkProxyHandler(service *auth.Service, logger *slog.Logger) (artalkProxyHandler, error) {
	target, err := url.Parse(service.ArtalkInternalURL())
	if err != nil {
		return artalkProxyHandler{}, err
	}
	return artalkProxyHandler{service: service, logger: logger, target: target}, nil
}

func (h artalkProxyHandler) proxy(c *gin.Context) {
	upstreamPath := c.Param("path")
	upstreamPath = legacyArtalkVotePath.ReplaceAllString(upstreamPath, `/api/v2/votes/$1/$3/$2`)
	if upstreamPath == "" || !strings.HasPrefix(upstreamPath, "/") || strings.Contains(upstreamPath, "..") {
		writeError(c, http.StatusBadRequest, "INVALID_ARTALK_PATH", "评论接口路径不正确")
		return
	}

	var reservation auth.CommentReservation
	unsafeMethod := isUnsafeArtalkMethod(c.Request.Method)
	artalkToken := artalkRequestToken(c.Request)
	authenticatedRead := !unsafeMethod && artalkToken != ""
	if unsafeMethod || authenticatedRead {
		session, ok, err := h.session(c)
		switch {
		case err != nil:
			h.logger.Error("authenticate comment author", "requestId", requestIDFromContext(c), "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
			return
		case !ok:
			writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录后再评论")
			return
		case artalkToken == "":
			writeError(c, http.StatusUnauthorized, "ARTALK_SESSION_REQUIRED", "评论身份已失效，请刷新页面后重试")
			return
		case unsafeMethod && !h.service.ValidOrigin(c.Request):
			writeError(c, http.StatusForbidden, "ORIGIN_INVALID", "评论请求来源不正确")
			return
		}
		if err := h.service.ValidateArtalkSession(c.Request.Context(), artalkToken, session); err != nil {
			switch {
			case errors.Is(err, auth.ErrArtalkSessionInvalid), errors.Is(err, auth.ErrArtalkUserMismatch):
				writeError(c, http.StatusUnauthorized, "ARTALK_SESSION_INVALID", "评论身份已失效，请刷新页面后重试")
			default:
				h.logger.Error("validate Artalk session", "requestId", requestIDFromContext(c), "error", err)
				writeError(c, http.StatusBadGateway, "COMMENTS_UNAVAILABLE", "评论服务暂时不可用")
			}
			return
		}
		// Artalk sends its token in the Authorization header for private
		// notification and moderation reads. Requiring the live blog session
		// above makes logout invalidate those reads immediately. Anonymous
		// public reads do not carry this header and remain available.
		if unsafeMethod {
			if c.Request.Method == http.MethodPost && upstreamPath == "/api/v2/comments" {
				reservation, err = h.service.ReserveComment(c.Request.Context(), session)
			} else {
				err = h.service.CheckCommentAccess(c.Request.Context(), session)
			}
			if err != nil {
				if writeCommentPolicyError(c, err) {
					return
				}
				h.logger.Error("reserve comment quota", "requestId", requestIDFromContext(c), "error", err)
				writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
				return
			}
		}
	}

	request := c.Request
	request.URL.Path = upstreamPath
	request.URL.RawPath = ""
	request.Host = h.target.Host
	// The blog session is scoped to the whole origin so this gateway can
	// enforce comment policy. Never forward that cookie to Artalk itself.
	request.Header.Del("Cookie")

	proxy := httputil.NewSingleHostReverseProxy(h.target)
	defaultDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		defaultDirector(request)
		request.URL.Path = upstreamPath
		request.URL.RawPath = ""
		request.Host = h.target.Host
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			h.releaseReservation(reservation)
		}
		return nil
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		h.releaseReservation(reservation)
		h.logger.Error("proxy Artalk request", "requestId", requestIDFromContext(c), "error", err)
		writeError(c, http.StatusBadGateway, "COMMENTS_UNAVAILABLE", "评论服务暂时不可用")
	}
	proxy.ServeHTTP(c.Writer, request)
}

func isUnsafeArtalkMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete
}

func artalkRequestToken(request *http.Request) string {
	if token := strings.TrimSpace(request.URL.Query().Get("token")); token != "" {
		return token
	}
	const bearerPrefix = "Bearer "
	authorization := request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authorization, bearerPrefix))
}

func (h artalkProxyHandler) session(c *gin.Context) (auth.Session, bool, error) {
	cookie, err := c.Request.Cookie(h.service.SessionCookieName())
	if err != nil {
		return auth.Session{}, false, nil
	}
	session, err := h.service.Authenticate(c.Request.Context(), cookie.Value)
	if errors.Is(err, auth.ErrUnauthenticated) {
		http.SetCookie(c.Writer, h.service.ClearSessionCookie())
		http.SetCookie(c.Writer, h.service.ClearLegacySessionCookie())
		return auth.Session{}, false, nil
	}
	return session, err == nil, err
}

func (h artalkProxyHandler) releaseReservation(reservation auth.CommentReservation) {
	if !reservation.Reserved {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	h.service.ReleaseCommentReservation(ctx, reservation)
}

func writeCommentPolicyError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, auth.ErrCommentsBlocked):
		writeError(c, http.StatusForbidden, "COMMENTS_BLOCKED", "该账号已被禁止发表评论")
	case errors.Is(err, auth.ErrCommentDailyLimit):
		writeError(c, http.StatusTooManyRequests, "COMMENT_DAILY_LIMIT", "今天的评论额度已用完，请明天再试")
	case errors.Is(err, auth.ErrCommentUserMissing):
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "请重新登录后再评论")
	case errors.Is(err, auth.ErrArtalkIdentityConflict):
		writeError(c, http.StatusConflict, "COMMENT_IDENTITY_CONFLICT", "评论身份邮箱已被其他账号占用，请联系站点管理员")
	default:
		return false
	}
	return true
}
