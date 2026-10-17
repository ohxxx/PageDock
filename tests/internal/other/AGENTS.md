# Other-platform tests

`appearance_test.go` retains `!darwin && !windows`. Verify that saved icon config does not block ordinary saves, failures retain old config, and defaults can be restored. packages.json keeps the tests in pagedock, reusing parent image fixtures. Do not pretend Linux or other platforms support icon application. Update the parent map and platform notes.
