package httpapi

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/example/myblog/apps/api/internal/audit"
	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/example/myblog/apps/api/internal/blog"
	"github.com/example/myblog/apps/api/internal/config"
	"github.com/example/myblog/apps/api/internal/upload"
)

func TestOpenAPIOperationsMatchRegisteredRoutes(t *testing.T) {
	cfg := config.Config{Environment: config.Test, SessionCookieName: "blog_session", MediaPublicURL: "/uploads"}
	store := blog.NewStore(nil, cfg.MediaPublicURL)
	router, err := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		PublicStore: store,
		AdminStore:  store,
		Auth:        auth.NewService(nil, cfg),
		Audit:       audit.NewService(nil),
		Uploads:     upload.NewService(nil, nil, cfg),
	})
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]bool)
	parameter := regexp.MustCompile(`:([^/]+)`)
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/internal/artalk/") {
			continue
		}
		path := strings.TrimPrefix(route.Path, "/api")
		path = parameter.ReplaceAllString(path, `{$1}`)
		actual[route.Method+" "+path] = true
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	openAPIPath := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../docs/openapi.yaml"))
	file, err := os.Open(openAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	documented := make(map[string]bool)
	pathLine := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodLine := regexp.MustCompile(`^    (get|post|put|delete|patch):\s*$`)
	currentPath := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if match := pathLine.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
			continue
		}
		if match := methodLine.FindStringSubmatch(line); match != nil && currentPath != "" {
			documented[strings.ToUpper(match[1])+" "+currentPath] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	if missing, extra := setDifference(documented, actual), setDifference(actual, documented); len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("OpenAPI mismatch; undocumented routes=%v, documented but unregistered=%v", extra, missing)
	}
}

func setDifference(left, right map[string]bool) []string {
	values := make([]string, 0)
	for value := range left {
		if !right[value] {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}
