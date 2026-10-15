package proc_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"devkit-deps/internal/plan"
	"devkit-deps/internal/proc"
)

func TestMain(m *testing.M) {
	if os.Getenv("DEVKIT_DEPS_TTY_CHILD") == "1" {
		os.Exit(ttyChild())
	}
	os.Exit(m.Run())
}

func TestSignaledPipeExitIs141(t *testing.T) {
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", "-c", "kill -PIPE $$"},
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, nil, &bytes.Buffer{}, &bytes.Buffer{}, 5*time.Second)
	if d != nil || code != 128+int(syscall.SIGPIPE) {
		t.Fatalf("code %d %+v", code, d)
	}
}

func TestTerminalReadStaysForeground(t *testing.T) {
	master, slavePath := openPty(t)
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "DEVKIT_DEPS_TTY_CHILD=1", "DEVKIT_DEPS_TTY_SLAVE="+slavePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if _, err := master.Write([]byte("ok\n")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	_ = master.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, readErr := master.Read(buf)
	waitErr := <-done
	code := 0
	if waitErr != nil {
		var exit *exec.ExitError
		if !errors.As(waitErr, &exit) {
			t.Fatal(waitErr)
		}
		code = exit.ExitCode()
	}
	if readErr != nil || string(buf[:n]) != "got:ok" || code != 0 {
		t.Fatalf("code %d read %q err %v\n%s", code, buf[:n], readErr, stderr.String())
	}
}

func ttyChild() int {
	slave, err := os.OpenFile(os.Getenv("DEVKIT_DEPS_TTY_SLAVE"), os.O_RDWR, 0)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return 1
	}
	defer slave.Close()
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", "-c", "IFS= read -r line; printf 'got:%s' \"$line\""},
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, slave, slave, slave, 2*time.Second)
	if d != nil {
		os.Stderr.WriteString(d.Message)
		return d.Exit
	}
	return code
}

func openPty(t *testing.T) (*os.File, string) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })
	fd := int(master.Fd())
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	var name [128]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		t.Fatalf("name: %v", errno)
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		t.Fatal("pty name is not terminated")
	}
	slavePath := string(name[:n])
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	termios, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatalf("get termios: %v", err)
	}
	termios.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(int(slave.Fd()), unix.TIOCSETA, termios); err != nil {
		t.Fatalf("set termios: %v", err)
	}
	return master, slavePath
}

func TestLaunchPreservesBinaryStdoutAndStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", "-c", "printf 'a\\000b'"},
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, bytes.NewReader([]byte{'a', 0, 'b'}), &stdout, &stderr, 0)
	if d != nil || code != 0 {
		t.Fatalf("code %d %+v", code, d)
	}
	if !bytes.Equal(stdout.Bytes(), []byte{'a', 0, 'b'}) {
		t.Fatalf("%#v", stdout.Bytes())
	}

	stdout.Reset()
	doc.Execution.Argv = []string{"/bin/sh", "-c", "cat"}
	code, d = proc.Launch(context.Background(), doc, bytes.NewReader([]byte{'a', 0, 'b'}), &stdout, &stderr, 0)
	if d != nil || code != 0 || !bytes.Equal(stdout.Bytes(), []byte{'a', 0, 'b'}) {
		t.Fatalf("code %d %+v %#v", code, d, stdout.Bytes())
	}
}

func TestLaunchForwardsArguments(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "args.sh")
	body := "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\0' \"$arg\"; done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", script, "a b", "", "ü", "--flag"},
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, nil, &stdout, ioDiscard(), 0)
	if d != nil || code != 0 {
		t.Fatalf("%d %+v", code, d)
	}
	want := append([]byte("a b"), 0)
	want = append(want, 0)
	want = append(want, []byte("ü")...)
	want = append(want, 0)
	want = append(want, []byte("--flag")...)
	want = append(want, 0)
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("%#v", stdout.Bytes())
	}
}

func TestLaunchExitCodesAndCallerCwd(t *testing.T) {
	for _, code := range []int{0, 7, 42} {
		doc := plan.Document{Execution: plan.ExecDoc{
			Argv:        []string{"/bin/sh", "-c", "exit " + itoa(code)},
			Environment: map[string]string{},
		}}
		got, d := proc.Launch(context.Background(), doc, nil, ioDiscard(), ioDiscard(), 0)
		if d != nil || got != code {
			t.Fatalf("exit %d -> %d %+v", code, got, d)
		}
	}
	dir := t.TempDir()
	var stdout bytes.Buffer
	doc := plan.Document{Execution: plan.ExecDoc{
		Argv:        []string{"/bin/sh", "-c", "pwd"},
		Cwd:         dir,
		Environment: map[string]string{},
	}}
	code, d := proc.Launch(context.Background(), doc, nil, &stdout, ioDiscard(), 0)
	if d != nil || code != 0 {
		t.Fatal(d)
	}
	got := strings.TrimSpace(stdout.String())
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want && got != dir {
		t.Fatalf("pwd %s want %s", got, want)
	}
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
