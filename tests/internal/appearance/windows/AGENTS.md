# Windows appearance tests

Every file retains a `windows` build tag. `apply_test.go` covers EXEs, links, and running-image replacement. `name_test.go` covers visible system names, registry matching, and recovery. `taskbar_test.go` checks real Win32 window properties and link migration; `rename_test.go` covers pinned-item renames, conflicts, and rollback.

`fixture_test.go` provides temporary EXE/multilingual fixtures for this independent package and requires Go; keep it synchronized with shared PE fixtures. `image_test.go` only creates images and `testmain_test.go` isolates locale. Each case creates/destroys Win32 windows on a locked thread without a MyGo event loop. Application desktop tests still cover actual app-window creation.

Modify temporary programs, links, and registry keys only, never installed applications. Record native Windows tests separately from portable resource tests; correct properties do not prove visible Explorer behavior. Update the product windows map, parent test map, and validation records.
