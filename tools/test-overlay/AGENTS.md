# Centralized test assembly

`main.go` mirrors relative paths under tests into product directories by default. `tests/packages.json` declares original package directories only for platform groups that need shared private state. Group names prefix virtual test filenames to prevent collisions. Mapping preserves build tags, package declarations, and working directories.

Reject missing product directories, linked test files, path escapes, existing product-side tests, and multiple sources mapped to one target. An empty test set fails. Write `.cache/tests/overlay.json` without copying product source or modifying the module cache.

`tests/tools/test-overlay/main_test.go` creates temporary modules and actually runs Go tests to verify private access, platform groups, desktop constraints, failure propagation, and path/conflict rejection. Update tests/AGENTS.md, packages.json, and this map together.
