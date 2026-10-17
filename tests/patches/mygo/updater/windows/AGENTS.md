# Windows update cleanup tests

`cleanup_test.go` retains the windows constraint, copies/runs temporary test EXEs, and verifies retention while locked followed by cleanup after process exit. packages.json maps it back to the updater product directory to test the shared algorithm directly, without copying cleanup code.

Operate only on EXEs created by the current test. Subprocess entry retains an explicit environment switch. Run `make test TEST_ARGS='-run TestWindowsUpdateCleanup -count=1 -timeout=30s'`; ordinary file tests do not prove Windows image-lock behavior. Update patch and test maps.
