package proc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/plan"
)

func Launch(ctx context.Context, doc plan.Document, stdin io.Reader, stdout, stderr io.Writer, timeout time.Duration) (int, *diag.Diagnostic) {
	if len(doc.Execution.Argv) == 0 {
		return 0, &diag.Diagnostic{Code: "launch.argv", Message: "nothing to execute", Phase: diag.PhaseLaunch, Exit: 4}
	}
	cmd := exec.Command(doc.Execution.Argv[0], doc.Execution.Argv[1:]...)
	if stdin == nil {
		devnull, err := os.Open(os.DevNull)
		if err != nil {
			return 0, launchFailed(err.Error())
		}
		defer devnull.Close()
		stdin = devnull
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if doc.Execution.Cwd != "" {
		cmd.Dir = doc.Execution.Cwd
	}
	cmd.Env = commandEnv(doc.Execution.Environment)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, launchFailed(err.Error())
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}
	select {
	case err := <-done:
		return finishExit(err)
	case <-ctx.Done():
		return stopChild(ctx, cmd, done)
	case <-timer:
		killGroup(cmd)
		<-done
		return 124, &diag.Diagnostic{Code: "launch.timeout", Message: "script deadline exceeded", Phase: diag.PhaseLaunch, Exit: 124}
	}
}

func stopChild(ctx context.Context, cmd *exec.Cmd, done <-chan error) (int, *diag.Diagnostic) {
	sig := syscall.SIGTERM
	if got, ok := SignalOf(ctx); ok && got != 0 {
		sig = got
	}
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, sig)
	}
	select {
	case err := <-done:
		code, d := finishExit(err)
		if d == nil {
			return code, nil
		}
		return signalExit(sig), nil
	case <-time.After(500 * time.Millisecond):
		killGroup(cmd)
		<-done
		return signalExit(sig), nil
	}
}

func signalExit(sig syscall.Signal) int {
	if sig == syscall.SIGINT {
		return 130
	}
	return 143
}

func finishExit(err error) (int, *diag.Diagnostic) {
	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if status, ok := ee.Sys().(syscall.WaitStatus); ok {
			if status.Exited() {
				return status.ExitStatus(), nil
			}
			if status.Signaled() {
				return signalExit(status.Signal()), nil
			}
		}
		if code := ee.ExitCode(); code >= 0 {
			return code, nil
		}
	}
	return 1, launchFailed(err.Error())
}

func launchFailed(message string) *diag.Diagnostic {
	return &diag.Diagnostic{Code: "launch.failed", Message: message, Phase: diag.PhaseLaunch, Exit: 4}
}
