package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
	}{
		{"linux", "amd64", "blink-v1.2.3-linux-amd64.tar.gz"},
		{"darwin", "arm64", "blink-v1.2.3-darwin-arm64.tar.gz"},
		{"windows", "amd64", "blink-v1.2.3-windows-amd64.zip"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, AssetName("blink", "v1.2.3", tt.goos, tt.goarch))
	}
}

func TestVerify(t *testing.T) {
	data := []byte("payload")
	sum := sha256.Sum256(data)
	line := hex.EncodeToString(sum[:]) + "  blink-v1-linux-amd64.tar.gz\n"

	assert.NoError(t, Verify(data, line))
	assert.Error(t, Verify([]byte("tampered"), line))
	assert.Error(t, Verify(data, ""))
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func zipped(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	files := map[string][]byte{
		"blink-v1-linux-amd64/README.md": []byte("docs"),
		"blink-v1-linux-amd64/blink":     []byte("ELF"),
	}
	got, err := Extract(tarGz(t, files), "x.tar.gz", "blink")
	require.NoError(t, err)
	assert.Equal(t, []byte("ELF"), got)

	got, err = Extract(zipped(t, map[string][]byte{"blink-v1/blink.exe": []byte("MZ")}), "x.zip", "blink.exe")
	require.NoError(t, err)
	assert.Equal(t, []byte("MZ"), got)

	_, err = Extract(tarGz(t, map[string][]byte{"only/README.md": []byte("x")}), "x.tar.gz", "blink")
	assert.ErrorContains(t, err, "not found")
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "blink")
	require.NoError(t, os.WriteFile(dest, []byte("old"), 0o755))

	require.NoError(t, Replace(dest, []byte("new")))
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), got)
	info, _ := os.Stat(dest)
	if runtime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o100)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".new", "temp file left behind")
	}
}

// End to end against a fake GitHub: latest release lookup, download,
// checksum, extraction, and the swap on disk.
func TestInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive format differs on windows")
	}
	tag := "v9.9.9"
	name := AssetName("blink", tag, runtime.GOOS, runtime.GOARCH)
	archive := tarGz(t, map[string][]byte{"blink-" + tag + "/blink": []byte("fresh")})
	sum := sha256.Sum256(archive)

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/Energma/blink-f/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
			tag, name, srv.URL+"/dl/"+name, name+".sha256", srv.URL+"/dl/"+name+".sha256")
	})
	mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(archive) })
	mux.HandleFunc("/dl/"+name+".sha256", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	c := New("Energma/blink-f")
	c.HTTP = srv.Client()
	// Point the API lookup at the fake server.
	c.HTTP.Transport = rewriteHost(srv)

	rel, err := c.Latest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, tag, rel.Tag)

	dest := filepath.Join(t.TempDir(), "blink")
	require.NoError(t, os.WriteFile(dest, []byte("stale"), 0o755))
	require.NoError(t, c.Install(context.Background(), rel, "blink", dest))

	got, _ := os.ReadFile(dest)
	assert.Equal(t, []byte("fresh"), got)
}

// rewriteHost sends api.github.com requests to the test server.
func rewriteHost(srv *httptest.Server) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.github.com" {
			r.URL.Scheme = "http"
			r.URL.Host = srv.Listener.Addr().String()
		}
		return http.DefaultTransport.RoundTrip(r)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
