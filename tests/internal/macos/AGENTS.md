# macOS application integration

`notification_test.go` retains `desktop && darwin` and the explicit notification validation switch. Check actual system logs for each banner; API acceptance is not visibility. packages.json keeps these cases in pagedock, sharing its event loop and Playground fixtures without changing notification identity or user config.

Use `tests/playground/macos/notifications.mjs`, preserving test-bundle registration, resources, environment, and cleanup. Update the parent map and tests/AGENTS.md.
