# Native view tests

`settings_test.go` and `language_test.go` cover input, drafts, saves, disabled state, keyboard access, scaling, and both languages. `browser_test.go` covers caption avoidance, loading feedback, and page bounds. `color_test.go` and `loading_test.go` retain desktop constraints and use real local WebViews.

`windows/menu_test.go` checks saved branding and menu input on every host. `windows/caption_test.go` requires `desktop && windows` and reads actual system-button pixels. packages.json maps the windows group back to view without exposing Browser/Settings private state. `desktop_test.go` and `testmain_test.go` are mutually exclusive; `image_test.go` only generates input images.

The working directory remains `internal/view/`; `make test-desktop` runs packages serially. Do not open physical devices, contact remote services, or write source-matching tests. Update headers and product/test maps.
