// [INPUT]: Saved URLs, temporary login redirects, and origin-matching examples.
// [OUTPUT]: URL validation, exact origins, and cross-site redirects that grant no permissions.
// [POS]: URL/navigation boundary tests; permission combinations are covered by permissions_test.go.
// [PROTOCOL]: Update this header, internal/site/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package site_test

import (
	"testing"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
	website "github.com/ohxxx/PageDock/internal/site"
)

func TestTargetURL(t *testing.T) {
	tests := []struct {
		raw       string
		allowHTTP bool
		want      string
	}{
		{" https://EXAMPLE.com:443 ", false, "https://example.com/"},
		{"https://example.com/inbox?view=unread#latest", false, "https://example.com/inbox?view=unread#latest"},
		{"https://example.com/#/inbox?view=all", false, "https://example.com/#/inbox?view=all"},
		{"http://localhost:8080/app", true, "http://localhost:8080/app"},
		{"http://[::1]:80", true, "http://[::1]/"},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			u, err := website.TargetURL(tc.raw, tc.allowHTTP)
			if err != nil {
				t.Fatal(err)
			}
			if u.String() != tc.want {
				t.Fatalf("got %q, want %q", u.String(), tc.want)
			}
		})
	}
}

func TestTargetRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"", "example.com", "/settings", "file:///etc/passwd", "javascript:alert(1)",
		"data:text/html,test", "mygo://localhost/", "http://mygo.localhost/",
		"http://example.com", "https://user:password@example.com/", "https://example.com:99999",
		"https://example.com:abc", "https://example.com:0", "https://[wrong]/", "https://a..com",
		"https://example.com/\nnext", "https://example.com\\@evil.test", "https://例子.com",
		"https://example.com/?token=sensitive", "https://example.com/?ACCESS_TOKEN=sensitive",
		"https://example.com/#access_token=sensitive", "https://example.com/#/login?api-key=sensitive",
		"https://example.com/?password=sensitive", "https://example.com/?code=sensitive",
		"https://example.com/?token=sensitive?next=true", "https://example.com/#access_token=sensitive?next=true",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := website.TargetURL(raw, false); err == nil {
				t.Fatal("unsafe URL was accepted")
			}
		})
	}
}

func TestOriginBoundary(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://example.com/messages", true},
		{"https://EXAMPLE.com:443/path#next", true},
		{"https://example.com:444/", false},
		{"http://example.com/", false},
		{"https://sub.example.com/", false},
		{"https://example.com.evil.test/", false},
		{"https://example.com@evil.test/", false},
		{"file:///tmp/index.html", false},
		{"about:blank", false},
	} {
		if got := website.SameOrigin(tc.url, "https://example.com"); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestNavigationAllowsLoginRedirectsWithoutGrantingPermissions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		url       string
		allowHTTP bool
		want      bool
	}{
		{"login site", "https://login.example.net/authorize?client_id=pagedock", false, true},
		{"login callback", "https://example.com/callback?code=test-code&state=test-state", false, true},
		{"implicit callback", "https://example.com/#access_token=test-token", false, true},
		{"HTTP enabled", "http://localhost:8080/login", true, true},
		{"HTTP disabled", "http://example.net/login", false, false},
		{"local file", "file:///tmp/settings.html", true, false},
		{"internal scheme", "mygo://localhost/", true, false},
		{"reserved origin", "http://mygo.localhost/", true, false},
		{"embedded password", "https://user:password@example.net/", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := website.NavigationURL(tc.url, tc.allowHTTP)
			if (err == nil) != tc.want {
				t.Fatalf("navigation allowed=%v, want %v: %v", err == nil, tc.want, err)
			}
		})
	}
	if prefs.AllowsPermissions("https://login.example.net", "https://example.com", prefs.Config{AllowNotifications: true}, []mygo.Permission{mygo.PermissionNotifications}) {
		t.Fatal("login site inherited notification permission")
	}
	if _, err := website.TargetURL("https://example.com/callback?code=test-code", false); err == nil {
		t.Fatal("temporary login URL was accepted as a persisted home URL")
	}
}
