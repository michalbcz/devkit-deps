package proc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/plan"
)

type SetupResult struct {
	ExitCode int
	Tail     []string
	Captured map[string]string
	Err      *diag.Diagnostic
}

func RunSetup(ctx context.Context, steps []plan.StepDoc, timeout time.Duration, stderr io.Writer) SetupResult {
	if stderr == nil {
		stderr = io.Discard
	}
	captured := map[string]string{}
	var tail []string
	for _, step := range steps {
		result := runStep(ctx, step, timeout, stderr)
		tail = result.Tail
		if result.Err != nil {
			result.Captured = captured
			return result
		}
		if len(step.CaptureEnvironment) > 0 {
			value := lastNonBlank(result.Tail)
			if value == "" {
				return SetupResult{
					ExitCode: result.ExitCode, Tail: result.Tail, Captured: captured,
					Err: &diag.Diagnostic{Code: "setup.capture", Message: "setup step returned no module search path", Phase: diag.PhasePrepare, Exit: 4},
				}
			}
			for _, key := range step.CaptureEnvironment {
				captured[key] = value
			}
		}
	}
	if captured == nil {
		captured = map[string]string{}
	}
	return SetupResult{Tail: tail, Captured: captured}
}

func runStep(ctx context.Context, step plan.StepDoc, timeout time.Duration, stderr io.Writer) SetupResult {
	if len(step.Argv) == 0 {
		return setupFailed(0, nil, "setup step has no argv")
	}
	cmd := exec.Command(step.Argv[0], step.Argv[1:]...)
	if step.Cwd != "" {
		cmd.Dir = step.Cwd
	}
	cmd.Env = commandEnv(step.Environment)
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return setupFailed(0, nil, err.Error())
	}
	defer devnull.Close()
	cmd.Stdin = devnull
	pr, pw, err := os.Pipe()
	if err != nil {
		return setupFailed(0, nil, err.Error())
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return setupFailed(0, nil, err.Error())
	}
	_ = pw.Close()

	lines := make([]string, 0, 200)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer pr.Close()
		reader := bufio.NewReader(pr)
		for {
			line, readErr := reader.ReadString('\n')
			if len(line) > 0 {
				line = strings.TrimRight(line, "\r\n")
				fmt.Fprintln(stderr, line)
				lines = append(lines, line)
				if len(lines) > 200 {
					lines = append([]string{}, lines[len(lines)-200:]...)
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	timedOut := make(chan struct{})
	cancelled := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.Time{}
		if timeout > 0 {
			deadline = time.Now().Add(timeout)
		}
		for {
			select {
			case <-readDone:
				return
			case <-ctx.Done():
				close(cancelled)
				killGroup(cmd)
				return
			case <-ticker.C:
				if !deadline.IsZero() && !time.Now().Before(deadline) {
					close(timedOut)
					killGroup(cmd)
					return
				}
			}
		}
	}()

	procErr := <-waitDone
	<-readDone
	tail := append([]string{}, lines...)
	code := exitCode(procErr)
	select {
	case <-timedOut:
		return SetupResult{ExitCode: code, Tail: tail, Err: &diag.Diagnostic{
			Code: "setup.timeout", Message: "dependency preparation timed out", Phase: diag.PhasePrepare, Exit: 124,
		}}
	case <-cancelled:
		exit := 130
		if sig, ok := signalOf(ctx); ok && sig == syscall.SIGTERM {
			exit = 143
		}
		return SetupResult{ExitCode: code, Tail: tail, Err: &diag.Diagnostic{
			Code: "setup.cancelled", Message: "dependency preparation cancelled", Phase: diag.PhasePrepare, Exit: exit,
		}}
	default:
	}
	if code != 0 {
		return setupFailed(code, tail, fmt.Sprintf("setup command failed (exit %d)", code))
	}
	return SetupResult{ExitCode: 0, Tail: tail}
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}

type signalKey struct{}
type signalBoxKey struct{}

type SignalBox struct {
	mu  sync.Mutex
	sig syscall.Signal
	set bool
}

func (b *SignalBox) Store(sig syscall.Signal) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.sig = sig
	b.set = true
	b.mu.Unlock()
}

func WithSignal(ctx context.Context, sig syscall.Signal) context.Context {
	return context.WithValue(ctx, signalKey{}, sig)
}

func WithSignalBox(ctx context.Context, box *SignalBox) context.Context {
	return context.WithValue(ctx, signalBoxKey{}, box)
}

func SignalOf(ctx context.Context) (syscall.Signal, bool) { return signalOf(ctx) }

func signalOf(ctx context.Context) (syscall.Signal, bool) {
	if box, ok := ctx.Value(signalBoxKey{}).(*SignalBox); ok && box != nil {
		box.mu.Lock()
		defer box.mu.Unlock()
		if box.set {
			return box.sig, true
		}
	}
	sig, ok := ctx.Value(signalKey{}).(syscall.Signal)
	return sig, ok
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}

func setupFailed(code int, tail []string, message string) SetupResult {
	return SetupResult{ExitCode: code, Tail: tail, Err: &diag.Diagnostic{
		Code: "setup.failed", Message: message, Phase: diag.PhasePrepare, Exit: 4,
	}}
}

func lastNonBlank(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

func commandEnv(overrides map[string]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(os.Environ())+len(overrides))
	for k, v := range overrides {
		if strings.EqualFold(k, "PATH") && strings.HasSuffix(v, string(os.PathListSeparator)) {
			v += os.Getenv("PATH")
		}
		out = append(out, k+"="+v)
		seen[k] = struct{}{}
	}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := seen[k]; ok {
			continue
		}
		out = append(out, kv)
	}
	return out
}
