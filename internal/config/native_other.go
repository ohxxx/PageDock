//go:build !windows

// [INPUT]: Platform name validation and browser-data cleanup operations for the build target.
// [OUTPUT]: Existing platform operations for config/startup, retaining the shared profile deletion algorithm.
// [POS]: Platform selection in config; windows/ owns native implementations and this layer only connects them.
// [PROTOCOL]: Update internal/config/AGENTS.md, platform tests, and validation records.
package config

import "github.com/egoist/mygo"

func ValidatePlatformName(string) error { return nil }

func ClearBrowsingData() error { return mygo.App.ClearBrowsingData() }
