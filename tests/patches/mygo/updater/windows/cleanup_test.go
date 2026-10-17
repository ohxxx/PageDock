//go:build windows

// [INPUT]: Temporary test EXE copies, running old processes, and real Windows image locks.
// [OUTPUT]: Retain files on timeout while locked; remove the old EXE in the same background cleanup after process exit.
// [POS]: Native Windows behavior tests that launch and rename only temporary test programs.
// [PROTOCOL]: Update patches/mygo/AGENTS.md and validation records.
package mygo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWindowsUpdateCleanupWaitsForExit(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "app.exe")
	if err := os.WriteFile(executable, data, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestWindowsUpdateCleanupHelper$")
	cmd.Env = append(os.Environ(), "PAGEDOCK_CLEANUP_HELPER=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close(); _ = cmd.Wait() }()
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("helper not ready: %q, %v", line, err)
	}
	old := filepath.Join(filepath.Dir(executable), ".app.exe.old-previous")
	if err := os.Rename(executable, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, data, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err = cleanUpdateExecutables(ctx, executable, os.Remove)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("running executable should remain locked: %v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	attempted := make(chan struct{})
	done := make(chan error, 1)
	var once sync.Once
	go func() {
		done <- cleanUpdateExecutables(ctx, executable, func(path string) error {
			err := os.Remove(path)
			once.Do(func() { close(attempted) })
			return err
		})
	}()
	select {
	case <-attempted:
	case <-ctx.Done():
		t.Fatal("cleanup did not start")
	}
	_ = input.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old executable remains after process exit: %v", err)
	}
}

func TestWindowsUpdateCleanupHelper(t *testing.T) {
	if os.Getenv("PAGEDOCK_CLEANUP_HELPER") != "1" {
		return
	}
	fmt.Println("ready")
	_, _ = os.Stdin.Read(make([]byte, 1))
}
