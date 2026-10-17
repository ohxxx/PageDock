// [INPUT]: Setting combinations, exact/cross-site origins, and browser permission requests.
// [OUTPUT]: Actual grant, default-deny, mixed-permission, and platform-limit policy results.
// [POS]: Permission behavior tests without source or declaration assertions.
// [PROTOCOL]: Update this header, internal/config/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package config

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestPermissionPolicy(t *testing.T) {
	all := Config{AllowNotifications: true, AllowCamera: true, AllowMicrophone: true, AllowGeolocation: true}
	for _, tc := range []struct {
		name        string
		origin      string
		config      Config
		permissions []mygo.Permission
		want        bool
	}{
		{"notifications", "https://example.com", Config{AllowNotifications: true}, []mygo.Permission{mygo.PermissionNotifications}, true},
		{"camera", "https://example.com", Config{AllowCamera: true}, []mygo.Permission{mygo.PermissionCamera}, true},
		{"microphone", "https://example.com", Config{AllowMicrophone: true}, []mygo.Permission{mygo.PermissionMicrophone}, true},
		{"geolocation", "https://example.com", all, []mygo.Permission{mygo.PermissionGeolocation}, GeolocationSupported},
		{"video call", "https://example.com", all, []mygo.Permission{mygo.PermissionCamera, mygo.PermissionMicrophone}, true},
		{"microphone disabled", "https://example.com", Config{AllowCamera: true}, []mygo.Permission{mygo.PermissionCamera, mygo.PermissionMicrophone}, false},
		{"camera disabled", "https://example.com", Config{AllowNotifications: true}, []mygo.Permission{mygo.PermissionCamera}, false},
		{"other site", "https://elsewhere.test", all, []mygo.Permission{mygo.PermissionCamera}, false},
		{"other port", "https://example.com:8443", all, []mygo.Permission{mygo.PermissionMicrophone}, false},
		{"other scheme", "http://example.com", all, []mygo.Permission{mygo.PermissionNotifications}, false},
		{"unknown", "https://example.com", all, []mygo.Permission{"filesystem"}, false},
		{"empty", "https://example.com", all, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AllowsPermissions(tc.origin, "https://example.com", tc.config, tc.permissions); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	for _, permission := range []mygo.Permission{mygo.PermissionNotifications, mygo.PermissionCamera, mygo.PermissionMicrophone, mygo.PermissionGeolocation} {
		if AllowsPermissions("https://example.com", "https://example.com", Config{}, []mygo.Permission{permission}) {
			t.Fatalf("default config granted %s", permission)
		}
	}
}
