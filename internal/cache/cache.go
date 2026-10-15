package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"devkit-deps/internal/diag"
)

const Schema = "devkit-deps-cache-v1"

const (
	ProfileTcl   = "tk>=8.6,<8.7"
	ProfileMojo  = "mojo==1.1.0;python>=3.11,<3.12"
	ProfileShell = "pixi-shell-v1"
)

type Identity struct {
	Language       string
	Dialect        string
	GOOS, GOARCH   string
	PixiTarget     string
	Profile        string
	RuntimePath    string
	RuntimeVersion string
	ManagerPath    string
	ManagerModUnix int64
	Helpers        []string
	Specs          []string
}

type canonical struct {
	Schema         string   `json:"schema"`
	Language       string   `json:"language"`
	Dialect        string   `json:"dialect"`
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	PixiTarget     string   `json:"pixiTarget"`
	Profile        string   `json:"profile"`
	RuntimePath    string   `json:"runtimePath"`
	RuntimeVersion string   `json:"runtimeVersion"`
	ManagerPath    string   `json:"managerPath"`
	ManagerModUnix int64    `json:"managerModUnix"`
	Helpers        []string `json:"helpers"`
	Specs          []string `json:"specs"`
}

type Ready struct {
	Schema        int               `json:"schema"`
	Captured      map[string]string `json:"captured"`
	RequiredFiles []string          `json:"requiredFiles"`
}

func Hash(id Identity) string {
	helpers, specs := id.Helpers, id.Specs
	if helpers == nil {
		helpers = []string{}
	}
	if specs == nil {
		specs = []string{}
	}
	body, err := json.Marshal(canonical{
		Schema: Schema, Language: id.Language, Dialect: id.Dialect,
		GOOS: id.GOOS, GOARCH: id.GOARCH, PixiTarget: id.PixiTarget, Profile: id.Profile,
		RuntimePath: id.RuntimePath, RuntimeVersion: id.RuntimeVersion,
		ManagerPath: id.ManagerPath, ManagerModUnix: id.ManagerModUnix,
		Helpers: helpers, Specs: specs,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func EnvDir(root, language, hash string) string {
	return filepath.Join(root, language+"-"+hash)
}

func ReadyPath(envDir string) string { return filepath.Join(envDir, ".ready.json") }

func EnvRoot() string { return os.Getenv("DEVKIT_DEPS_CACHE_DIR") }

func PlatformRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "devkit-deps"), nil
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "devkit-deps", "cache"), nil
	default:
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			return filepath.Join(xdg, "devkit-deps"), nil
		}
		return filepath.Join(home, ".cache", "devkit-deps"), nil
	}
}

func DefaultRoot() (string, error) {
	if env := EnvRoot(); env != "" {
		return env, nil
	}
	return PlatformRoot()
}

func PixiTarget(p Platform) (string, *diag.Diagnostic) {
	switch p.GOOS + "/" + p.GOARCH {
	case "darwin/arm64":
		return "osx-arm64", nil
	case "darwin/amd64":
		return "osx-64", nil
	case "linux/arm64":
		return "linux-aarch64", nil
	case "linux/amd64":
		return "linux-64", nil
	default:
		return "", &diag.Diagnostic{
			Code: "platform.unsupported", Message: "no Pixi target for " + p.GOOS + "/" + p.GOARCH,
			Phase: diag.PhasePlan, Exit: 3,
		}
	}
}

func WriteReady(envDir string, ready Ready) error {
	if ready.Schema == 0 {
		ready.Schema = 1
	}
	if ready.Captured == nil {
		ready.Captured = map[string]string{}
	}
	if ready.RequiredFiles == nil {
		ready.RequiredFiles = []string{}
	}
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(ready)
	if err != nil {
		return err
	}
	tmp := filepath.Join(envDir, ".ready.json.tmp")
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ReadyPath(envDir))
}

func ReadReady(envDir string) (Ready, error) {
	body, err := os.ReadFile(ReadyPath(envDir))
	if err != nil {
		return Ready{}, incomplete("readiness marker is missing")
	}
	var ready Ready
	if err := json.Unmarshal(body, &ready); err != nil || ready.Schema != 1 {
		return Ready{}, incomplete("readiness marker is invalid")
	}
	for _, rel := range ready.RequiredFiles {
		path := filepath.Join(envDir, rel)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return Ready{}, incomplete("required file is missing: " + rel)
		}
	}
	return ready, nil
}

func incomplete(message string) *diag.Diagnostic {
	return &diag.Diagnostic{Code: "cache.incomplete", Message: message, Phase: diag.PhasePrepare, Exit: 4}
}

func RemoveTree(root string) error {
	clean := filepath.Clean(root)
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return &diag.Diagnostic{Code: "cache.boundary", Message: "refusing to delete " + root, Phase: diag.PhasePrepare, Exit: 4}
	}
	info, err := os.Lstat(clean)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(clean)
	}
	return removeChildren(clean)
}

func removeChildren(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		if info.IsDir() {
			if err := removeChildren(path); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.Remove(dir)
}
