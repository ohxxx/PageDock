# Windows browsing-data tests

`browser_test.go` retains a windows build tag and checks temporary file-lock release, timeout errors, LOCALAPPDATA discovery, and external-profile override rejection. tests/packages.json maps it back to config, using the shared private deletion algorithm and public platform entry without copying code or expanding product APIs.

Use temporary directories and handles only; run alongside shared config/reset tests. Update the product config/windows map and tests/AGENTS.md.
