// Package rmxbin ships the rmx multiplexer inside the blink binary so blink
// works on a machine with nothing but git installed. The binary is embedded
// at build time (see bin/NOTICE), unpacked to the user's cache directory on
// first use, and reused from there; the unpack directory is keyed by the
// content hash, so a rebuilt rmx never collides with an old copy.
package rmxbin

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed bin
var files embed.FS

var (
	once sync.Once
	path string
	err  error
)

func exeName() string {
	if runtime.GOOS == "windows" {
		return "rmx.exe"
	}
	return "rmx"
}

// Path returns an executable rmx: the embedded copy when blink was built
// with one, otherwise whatever `rmx` is on PATH. The embedded copy is
// unpacked once per process and cached across calls.
func Path() (string, error) {
	once.Do(func() { path, err = resolve() })
	return path, err
}

func resolve() (string, error) {
	data, readErr := files.ReadFile("bin/" + exeName())
	if readErr != nil {
		p, lookErr := exec.LookPath("rmx")
		if lookErr != nil {
			return "", fmt.Errorf("rmx is neither bundled in this build nor on PATH")
		}
		return p, nil
	}
	cache, cacheErr := os.UserCacheDir()
	if cacheErr != nil {
		return "", fmt.Errorf("locate cache dir for rmx: %w", cacheErr)
	}
	return unpack(data, filepath.Join(cache, "blink", "rmx"))
}

// unpack writes data as an executable under root/<hash>/ unless that file
// already exists. Writing to a temp name and renaming keeps a concurrent
// blink from ever seeing a half-written binary.
func unpack(data []byte, root string) (string, error) {
	sum := sha256.Sum256(data)
	dir := filepath.Join(root, hex.EncodeToString(sum[:8]))
	dst := filepath.Join(dir, exeName())
	if info, statErr := os.Stat(dst); statErr == nil && info.Size() == int64(len(data)) {
		return dst, nil
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return "", fmt.Errorf("create rmx dir: %w", mkErr)
	}
	tmp, tmpErr := os.CreateTemp(dir, exeName()+".*.tmp")
	if tmpErr != nil {
		return "", fmt.Errorf("unpack rmx: %w", tmpErr)
	}
	tmpName := tmp.Name()
	if _, wErr := tmp.Write(data); wErr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("unpack rmx: %w", wErr)
	}
	if cErr := tmp.Close(); cErr != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("unpack rmx: %w", cErr)
	}
	if chErr := os.Chmod(tmpName, 0o755); chErr != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("unpack rmx: %w", chErr)
	}
	if rErr := os.Rename(tmpName, dst); rErr != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("unpack rmx: %w", rErr)
	}
	return dst, nil
}
