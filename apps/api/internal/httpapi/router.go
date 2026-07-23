package httpapi

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/myblog/apps/api/internal/audit"
	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/example/myblog/apps/api/internal/config"
	"github.com/example/myblog/apps/api/internal/upload"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	DB          *sql.DB
	PublicStore publicStore
	AdminStore  adminStore
	Auth        *auth.Service
	Audit       *audit.Service
	Uploads     *upload.Service
	MinIOReady  func(context.Context) error
	ArtalkReady func(context.Context) error
}

func NewRouter(cfg config.Config, logger *slog.Logger, dependency ...Dependencies) (*gin.Engine, error) {
	if cfg.Environment == config.Production {
		gin.SetMode(gin.ReleaseMode)
	} else if cfg.Environment == config.Test {
		gin.SetMode(gin.TestMode)
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, err
	}
	router.Use(requestIDMiddleware(), securityHeadersMiddleware(), accessLogMiddleware(logger), recoveryMiddleware(logger))
	var deps Dependencies
	if len(dependency) > 0 {
		deps = dependency[0]
	}

	api := router.Group("/api")
	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok"}})
	})
	api.GET("/readyz", func(c *gin.Context) {
		if deps.DB != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
			defer cancel()
			if err := deps.DB.PingContext(ctx); err != nil {
				writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "服务依赖尚未就绪")
				return
			}
		}
		if deps.MinIOReady != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
			defer cancel()
			if err := deps.MinIOReady(ctx); err != nil {
				writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "服务依赖尚未就绪")
				return
			}
		}
		if deps.ArtalkReady != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
			defer cancel()
			if err := deps.ArtalkReady(ctx); err != nil {
				writeError(c, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "服务依赖尚未就绪")
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok"}})
	})
	if deps.PublicStore != nil {
		public := publicHandler{store: deps.PublicStore, logger: logger}
		api.GET("/site", public.site)
		api.GET("/posts", public.posts)
		api.GET("/posts/:slug", public.post)
		api.GET("/tags", public.tags)
		api.GET("/categories", public.categories)
	}
	if deps.Auth != nil {
		authHTTP := authHandler{service: deps.Auth, logger: logger}
		beginLoginRateLimit := newRateLimiter(20, time.Minute)
		artalkSessionRateLimit := newRateLimiter(60, time.Minute)
		api.GET("/auth/github", beginLoginRateLimit, authHTTP.beginGitHub)
		// A callback can reach GitHub only with a signed, unexpired, one-time
		// state created by the rate-limited begin endpoint. An additional IP
		// bucket here lets unrelated traffic invalidate a legitimate return
		// from GitHub, especially behind shared NAT.
		api.GET("/auth/github/callback", authHTTP.githubCallback)
		api.GET("/auth/me", authHTTP.me)
		api.POST("/auth/logout", authHTTP.logout)
		api.POST("/auth/artalk/session", artalkSessionRateLimit, authHTTP.artalkSession)
		router.GET("/internal/artalk-oidc/userinfo", authHTTP.artalkUserInfo)
		artalkProxy, err := newArtalkProxyHandler(deps.Auth, logger)
		if err != nil {
			return nil, err
		}
		for _, method := range []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		} {
			router.Handle(method, "/internal/artalk/*path", artalkProxy.proxy)
		}
	}
	if deps.Auth != nil && deps.AdminStore != nil {
		adminHTTP := adminHandler{store: deps.AdminStore, logger: logger}
		administratorHTTP := administratorHandler{service: deps.Auth, logger: logger}
		commentUserHTTP := commentUserHandler{service: deps.Auth, logger: logger}
		auditHTTP := auditHandler{service: deps.Audit, logger: logger}
		admin := api.Group("/admin", requireAdmin(deps.Auth, logger), auditAdminMutations(deps.Audit, logger))
		admin.GET("/posts", adminHTTP.posts)
		admin.GET("/posts/trash", adminHTTP.trashedPosts)
		admin.GET("/posts/:id", adminHTTP.post)
		admin.POST("/posts", requireCSRF(deps.Auth), adminHTTP.createPost)
		admin.PUT("/posts/:id", requireCSRF(deps.Auth), adminHTTP.updatePost)
		admin.DELETE("/posts/:id", requireCSRF(deps.Auth), adminHTTP.deletePost)
		admin.POST("/posts/:id/restore", requireCSRF(deps.Auth), adminHTTP.restorePost)
		admin.DELETE("/posts/:id/permanent", requireCSRF(deps.Auth), adminHTTP.deletePostPermanent)
		admin.GET("/tags", adminHTTP.tags)
		admin.POST("/tags", requireCSRF(deps.Auth), adminHTTP.createTag)
		admin.PUT("/tags/:id", requireCSRF(deps.Auth), adminHTTP.updateTag)
		admin.DELETE("/tags/:id", requireCSRF(deps.Auth), adminHTTP.deleteTag)
		admin.GET("/categories", adminHTTP.categories)
		admin.POST("/categories", requireCSRF(deps.Auth), adminHTTP.createCategory)
		admin.PUT("/categories/:id", requireCSRF(deps.Auth), adminHTTP.updateCategory)
		admin.DELETE("/categories/:id", requireCSRF(deps.Auth), adminHTTP.deleteCategory)
		admin.PUT("/site/appearance", requireCSRF(deps.Auth), adminHTTP.updateSiteAppearance)
		admin.GET("/administrators", requireOwner(), administratorHTTP.list)
		admin.POST("/administrators", requireOwner(), requireCSRF(deps.Auth), administratorHTTP.add)
		admin.DELETE("/administrators/:githubId", requireOwner(), requireCSRF(deps.Auth), administratorHTTP.remove)
		admin.GET("/users", commentUserHTTP.list)
		admin.PUT("/users/:githubId/comment-policy", requireCSRF(deps.Auth), commentUserHTTP.update)
		if deps.Audit != nil {
			admin.GET("/audit-events", requireOwner(), auditHTTP.list)
		}
	}
	if deps.Auth != nil && deps.Uploads != nil {
		uploadHTTP := uploadHandler{service: deps.Uploads, logger: logger}
		uploadConcurrencyLimit := newConcurrencyLimiter(maxConcurrentUploads)
		media := api.Group(
			"/admin/uploads",
			requireAdmin(deps.Auth, logger),
			auditAdminMutations(deps.Audit, logger),
		)
		media.GET("", uploadHTTP.list)
		media.GET("/:id", uploadHTTP.get)
		media.POST("", newRateLimiter(30, time.Minute), requireCSRF(deps.Auth), uploadConcurrencyLimit, uploadHTTP.create)
		media.DELETE("/:id", requireCSRF(deps.Auth), uploadHTTP.trash)
		media.POST("/:id/restore", requireCSRF(deps.Auth), uploadHTTP.restore)
		media.DELETE("/:id/permanent", requireCSRF(deps.Auth), uploadHTTP.permanent)
	}

	router.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "接口不存在")
	})
	router.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "请求方法不受支持")
	})
	return router, nil
}
