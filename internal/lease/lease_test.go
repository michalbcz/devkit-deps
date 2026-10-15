package lease_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/lease"
)

func TestMain(m *testing.M) {
	if os.Getenv("DEVKIT_DEPS_HOLD_LOCK") != "" {
		held, err := lease.AcquireUse(context.Background(), os.Args[1])
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(2)
		}
		defer held.Release()
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestUseLeaseBlocksUntilRelease(t *testing.T) {
	env := filepath.Join(t.TempDir(), "perl-abc")
	first, err := lease.AcquireUse(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(env); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("AcquireUse created the environment directory")
	}
	got := make(chan struct{})
	go func() {
		second, err := lease.AcquireUse(context.Background(), env)
		if err != nil {
			t.Error(err)
			close(got)
			return
		}
		second.Release()
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("second acquire did not wait")
	case <-time.After(200 * time.Millisecond):
	}
	first.Release()
	first.Release()
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("second acquire did not finish")
	}
}

func TestCancelWaiterLeavesEnvironment(t *testing.T) {
	root := t.TempDir()
	env := filepath.Join(root, "perl-abc")
	if err := os.MkdirAll(env, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(env, "live.txt")
	if err := os.WriteFile(marker, []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	holder, err := lease.AcquireUse(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := lease.AcquireUse(ctx, env)
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	err = <-errc
	var d *diag.Diagnostic
	if !errors.As(err, &d) || d.Code != "lease.cancelled" || d.Exit != 130 {
		t.Fatalf("%v", err)
	}
	body, err := os.ReadFile(marker)
	if err != nil || string(body) != "safe" {
		t.Fatalf("marker changed: %q %v", body, err)
	}
	holder.Release()
}

func TestOtherProcessHoldsUseLease(t *testing.T) {
	env := filepath.Join(t.TempDir(), "perl-abc")
	cmd := exec.Command(os.Args[0], env)
	cmd.Env = append(os.Environ(), "DEVKIT_DEPS_HOLD_LOCK=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	lock := env + ".usage.lock"
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(lock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not create the use lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := lease.AcquireUse(ctx, env); err == nil {
		t.Fatal("acquired the use lease while another process holds it")
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := lease.AcquireUse(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	got.Release()
}
