// [INPUT]: The saved website, configured permissions, and actual WebView permission requests.
// [OUTPUT]: Grant only enabled, platform-controllable permissions to the exact origin; reject unknown or partially unauthorized requests.
// [POS]: Settings-to-browser permission boundary; MyGo on macOS does not yet expose geolocation callbacks.
// [PROTOCOL]: Update this header and internal/config/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package config

import (
	"runtime"

	"github.com/egoist/mygo"
	website "github.com/ohxxx/PageDock/internal/site"
)

const GeolocationSupported = runtime.GOOS == "linux" || runtime.GOOS == "windows"

func AllowsPermissions(raw, expected string, config Config, permissions []mygo.Permission) bool {
	if !website.SameOrigin(raw, expected) || len(permissions) == 0 {
		return false
	}
	for _, permission := range permissions {
		var allowed bool
		switch permission {
		case mygo.PermissionNotifications:
			allowed = config.AllowNotifications
		case mygo.PermissionCamera:
			allowed = config.AllowCamera
		case mygo.PermissionMicrophone:
			allowed = config.AllowMicrophone
		case mygo.PermissionGeolocation:
			allowed = GeolocationSupported && config.AllowGeolocation
		}
		if !allowed {
			return false
		}
	}
	return true
}
