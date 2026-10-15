package lease

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/proc"
	"golang.org/x/sys/unix"
)

type Lease struct {
	f    *os.File
	mu   *sync.Mutex
	once sync.Once
	err  error
}

func AcquireUse(ctx context.Context, envDir string) (*Lease, error) {
	return acquire(ctx, envDir, ".usage.lock")
}

func AcquirePrep(ctx context.Context, envDir string) (*Lease, error) {
	return acquire(ctx, envDir, ".lock")
}

func (l *Lease) Release() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		if l.f != nil {
			_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
			l.err = l.f.Close()
		}
		if l.mu != nil {
			l.mu.Unlock()
		}
	})
	return l.err
}

func acquire(ctx context.Context, envDir, suffix string) (*Lease, error) {
	parent := filepath.Dir(envDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(parent, filepath.Base(envDir)+suffix)
	mu := mutexFor(path)
	if err := lockMutex(ctx, mu); err != nil {
		return nil, cancelled(ctx)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		mu.Unlock()
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			mu.Unlock()
			return nil, cancelled(ctx)
		}
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return &Lease{f: file, mu: mu}, nil
		}
		if err != syscall.EAGAIN && err != syscall.EWOULDBLOCK {
			_ = file.Close()
			mu.Unlock()
			return nil, err
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			mu.Unlock()
			return nil, cancelled(ctx)
		case <-timer.C:
		}
	}
}

func cancelled(ctx context.Context) *diag.Diagnostic {
	exit := 130
	if sig, ok := proc.SignalOf(ctx); ok && sig == syscall.SIGTERM {
		exit = 143
	}
	return &diag.Diagnostic{
		Code: "lease.cancelled", Message: "cancelled while waiting for a cache lock",
		Phase: diag.PhasePrepare, Exit: exit,
	}
}

var (
	gatesMu sync.Mutex
	gates   = map[string]*sync.Mutex{}
)

func mutexFor(path string) *sync.Mutex {
	gatesMu.Lock()
	defer gatesMu.Unlock()
	mu, ok := gates[path]
	if !ok {
		mu = &sync.Mutex{}
		gates[path] = mu
	}
	return mu
}

func lockMutex(ctx context.Context, mu *sync.Mutex) error {
	acquired := make(chan struct{})
	go func() {
		mu.Lock()
		close(acquired)
	}()
	select {
	case <-acquired:
		return nil
	case <-ctx.Done():
		go func() {
			<-acquired
			mu.Unlock()
		}()
		return ctx.Err()
	}
}
