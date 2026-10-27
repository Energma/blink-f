// Package update replaces the running blink binary with the latest GitHub
// release. The release archives are produced by .github/workflows/release.yml
// and carry rmx inside, so one update refreshes both.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Release is the subset of GitHub's release JSON we need.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Asset is one downloadable file on a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Client fetches and installs releases of one GitHub repository.
type Client struct {
	Repo string // "owner/name"
	HTTP *http.Client
}

// New returns a Client for repo with a bounded HTTP timeout.
func New(repo string) *Client {
	return &Client{Repo: repo, HTTP: &http.Client{Timeout: 5 * time.Minute}}
}

// Latest returns the newest published release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	url := "https://api.github.com/repos/" + c.Repo + "/releases/latest"
	body, err := c.get(ctx, url)
	var he *httpError
	if errors.As(err, &he) && he.status == http.StatusNotFound {
		return nil, fmt.Errorf("no release published yet for %s", c.Repo)
	}
	if err != nil {
		return nil, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	if rel.Tag == "" {
		return nil, fmt.Errorf("no release published yet")
	}
	return &rel, nil
}

// AssetName is the archive the release workflow produces for this platform:
// <bin>-<tag>-<goos>-<goarch>.tar.gz, or .zip on Windows.
func AssetName(bin, tag, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s-%s-%s-%s%s", bin, tag, goos, goarch, ext)
}

// Install downloads the archive for this platform, verifies its checksum,
// and swaps the binary at dest for the one inside. It returns the tag that
// was installed.
func (c *Client) Install(ctx context.Context, rel *Release, bin, dest string) error {
	name := AssetName(bin, rel.Tag, runtime.GOOS, runtime.GOARCH)
	var archive, sum *Asset
	for i := range rel.Assets {
		switch rel.Assets[i].Name {
		case name:
			archive = &rel.Assets[i]
		case name + ".sha256":
			sum = &rel.Assets[i]
		}
	}
	if archive == nil {
		return fmt.Errorf("release %s has no build for %s/%s (%s)", rel.Tag, runtime.GOOS, runtime.GOARCH, name)
	}
	if sum == nil {
		return fmt.Errorf("release %s has no checksum for %s", rel.Tag, name)
	}

	data, err := c.get(ctx, archive.URL)
	if err != nil {
		return err
	}
	want, err := c.get(ctx, sum.URL)
	if err != nil {
		return err
	}
	if err := Verify(data, string(want)); err != nil {
		return err
	}

	exe := bin
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	payload, err := Extract(data, name, exe)
	if err != nil {
		return err
	}
	return Replace(dest, payload)
}

// Verify checks data against a "<hex>  <name>" line as `shasum -a 256` prints.
func Verify(data []byte, sumLine string) error {
	fields := strings.Fields(sumLine)
	if len(fields) == 0 {
		return fmt.Errorf("empty checksum file")
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != strings.ToLower(fields[0]) {
		return fmt.Errorf("checksum mismatch: archive is corrupt or tampered")
	}
	return nil
}

// Extract returns the file named exe (by base name) from a .tar.gz or .zip
// archive, chosen by the archive's file name.
func Extract(data []byte, archiveName, exe string) ([]byte, error) {
	if strings.HasSuffix(archiveName, ".zip") {
		return extractZip(data, exe)
	}
	return extractTarGz(data, exe)
}

func extractTarGz(data []byte, exe string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == exe {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", exe)
}

func extractZip(data []byte, exe string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && filepath.Base(f.Name) == exe {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("read archive: %w", err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", exe)
}

// Replace writes payload next to dest and renames it over dest, so a crash
// mid-write never leaves a half binary. Windows won't let a running exe be
// overwritten, so there the old one is moved aside first and cleaned up on
// the next update.
func Replace(dest string, payload []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, filepath.Base(dest)+".*.new")
	if err != nil {
		return fmt.Errorf("write to %s (try with sudo?): %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		cleanup()
		return fmt.Errorf("make executable: %w", err)
	}
	if runtime.GOOS == "windows" {
		old := dest + ".old"
		os.Remove(old)
		if err := os.Rename(dest, old); err != nil && !os.IsNotExist(err) {
			cleanup()
			return fmt.Errorf("move old binary aside: %w", err)
		}
	}
	if err := os.Rename(tmpName, dest); err != nil {
		cleanup()
		return fmt.Errorf("replace %s: %w", dest, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "blink-update")
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{url: url, status: resp.StatusCode, text: resp.Status}
	}
	// Release archives are a few MB; cap well above that so a bad server
	// can't make us buffer forever.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	return body, nil
}

type httpError struct {
	url    string
	status int
	text   string
}

func (e *httpError) Error() string { return fmt.Sprintf("fetch %s: HTTP %s", e.url, e.text) }
