# Windows configuration

`name.go` validates Windows names. `browser.go` locates the WebView2 profile for the fixed internal name and rejects external overrides. Both declare `//go:build windows`.

Browser cleanup receives the parent's existing `removeBrowsingProfile` operation, retaining the 15-second context, call order, and error propagation. Do not copy the deletion algorithm or import the parent package. Settings and appearance reuse the parent's name-validation entry.

`tests/internal/config/windows/browser_test.go` runs in config through package ownership mapping; shared save/validation cases remain in the parent test directory. Use temporary LOCALAPPDATA and file locks only, never user profiles. Update headers, the parent map, and test notes.
