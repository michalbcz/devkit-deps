package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devkit-deps/internal/cache"
	"devkit-deps/internal/diag"
	"devkit-deps/internal/lease"
	"devkit-deps/internal/plan"
	"devkit-deps/internal/proc"
)

func Prepare(ctx context.Context, doc plan.Document, held *lease.Lease, rebuild, noPrepare bool, timeout time.Duration, stderr io.Writer) (plan.Document, *diag.Diagnostic) {
	if len(doc.Setup) == 0 {
		return doc, nil
	}
	if len(doc.MissingTools) > 0 {
		return doc, &diag.Diagnostic{
			Code: "tool.missing", Message: "missing " + strings.Join(doc.MissingTools, ", "),
			Language: doc.Language, Phase: diag.PhaseDiscover,
			MissingTools: append([]string{}, doc.MissingTools...), Exit: 3,
		}
	}
	if doc.EnvironmentDirectory == "" {
		return doc, &diag.Diagnostic{Code: "cache.directory", Message: "environment directory is not set", Phase: diag.PhasePrepare, Exit: 4}
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if doc.Execution.Environment == nil {
		doc.Execution.Environment = map[string]string{}
	}
	owned := false
	if held == nil {
		var err error
		held, err = lease.AcquireUse(ctx, doc.EnvironmentDirectory)
		if err != nil {
			return doc, asDiag(err)
		}
		owned = true
	}
	if owned {
		defer held.Release()
	}
	prep, err := lease.AcquirePrep(ctx, doc.EnvironmentDirectory)
	if err != nil {
		return doc, asDiag(err)
	}
	defer prep.Release()

	if !rebuild {
		ready, err := cache.ReadReady(doc.EnvironmentDirectory)
		if err == nil {
			for k, v := range ready.Captured {
				doc.Execution.Environment[k] = v
			}
			doc.Readiness = "ready"
			fmt.Fprintf(stderr, "Using cached environment: %s\n", doc.EnvironmentDirectory)
			return doc, nil
		}
		if noPrepare {
			return doc, &diag.Diagnostic{Code: "cache.not_ready", Message: "environment is not ready", Phase: diag.PhasePrepare, Exit: 4}
		}
	}
	if err := cache.RemoveTree(doc.EnvironmentDirectory); err != nil {
		return doc, asDiag(err)
	}
	if err := os.MkdirAll(doc.EnvironmentDirectory, 0o755); err != nil {
		return doc, &diag.Diagnostic{Code: "cache.write", Message: err.Error(), Phase: diag.PhasePrepare, Exit: 4}
	}
	if d := writeFiles(doc.EnvironmentDirectory, doc.Files); d != nil {
		return doc, d
	}
	result := proc.RunSetup(ctx, doc.Setup, timeout, stderr)
	if result.Err != nil {
		return doc, result.Err
	}
	captured := result.Captured
	if captured == nil {
		captured = map[string]string{}
	}
	if err := cache.WriteReady(doc.EnvironmentDirectory, cache.Ready{Schema: 1, Captured: captured, RequiredFiles: doc.RequiredFiles}); err != nil {
		return doc, &diag.Diagnostic{Code: "cache.write", Message: err.Error(), Phase: diag.PhasePrepare, Exit: 4}
	}
	for k, v := range captured {
		doc.Execution.Environment[k] = v
	}
	for _, key := range doc.Execution.DeferredEnvironment {
		if v, ok := captured[key]; ok {
			doc.Execution.Environment[key] = v
		}
	}
	doc.Readiness = "ready"
	return doc, nil
}

func writeFiles(env string, files []plan.FileDoc) *diag.Diagnostic {
	for _, file := range files {
		if strings.HasPrefix(file.Path, "<per-run-") {
			continue
		}
		if file.Path == "" || strings.HasPrefix(file.Path, "<") || filepath.IsAbs(file.Path) || strings.Contains(file.Path, "..") {
			return &diag.Diagnostic{Code: "cache.boundary", Message: "refusing to write " + file.Path, Phase: diag.PhasePrepare, Exit: 4}
		}
		dest := filepath.Join(env, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return &diag.Diagnostic{Code: "cache.write", Message: err.Error(), Phase: diag.PhasePrepare, Exit: 4}
		}
		if err := os.WriteFile(dest, []byte(file.Body), 0o644); err != nil {
			return &diag.Diagnostic{Code: "cache.write", Message: err.Error(), Phase: diag.PhasePrepare, Exit: 4}
		}
	}
	return nil
}

func asDiag(err error) *diag.Diagnostic {
	var d *diag.Diagnostic
	if errors.As(err, &d) {
		return d
	}
	return &diag.Diagnostic{Code: "cache.write", Message: err.Error(), Phase: diag.PhasePrepare, Exit: 4}
}
