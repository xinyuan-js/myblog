package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/example/myblog/apps/api/internal/config"
)

func TestArtalkIdentityTokenIsShortLivedAndTamperProof(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service := &Service{
		cfg: config.Config{OAuthStateSecret: strings.Repeat("s", 32)},
		now: func() time.Time { return now },
	}
	session := Session{User: User{
		GitHubID: 100001, Login: "octocat", Name: "Example User",
		Email: "OWNER@EXAMPLE.COM", IsAdmin: true,
	}}
	token, err := service.IssueArtalkToken(session)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.VerifyArtalkToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Subject != "100001" || user.Email != "owner@example.com" || !user.EmailVerified {
		t.Fatalf("userinfo = %+v", user)
	}

	tampered := "x" + token[1:]
	if _, err := service.VerifyArtalkToken(tampered); !errors.Is(err, ErrInvalidArtalkToken) {
		t.Fatalf("tampered token error = %v", err)
	}
	service.now = func() time.Time { return now.Add(artalkTokenTTL + time.Second) }
	if _, err := service.VerifyArtalkToken(token); !errors.Is(err, ErrInvalidArtalkToken) {
		t.Fatalf("expired token error = %v", err)
	}
}
