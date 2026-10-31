// Package update finds the latest lazygram release on GitHub and installs
// it the way the running binary was installed: with brew, or by replacing
// the binary with the one from the release tarball. Every download is
// checked against the release's SHA256SUMS before anything is installed.
package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// Repo is the GitHub repository that publishes the releases.
	Repo = "t0d0r/lazygram"
	// Formula is the Homebrew formula that installs lazygram.
	Formula = "t0d0r/tap/lazygram"
	// maxFile limits every download; the release files are a few MiB.
	maxFile = 64 << 20
)

// Version is a release version, vMAJOR.MINOR.PATCH.
type Version struct{ Major, Minor, Patch int }

var versionRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// Parse accepts vX.Y.Z and X.Y.Z (Homebrew builds drop the v). Anything
// else, such as "dev" or git describe output between tags, is not a
// release version.
func Parse(s string) (Version, bool) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	var n [3]int
	for i := range n {
		v, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Version{}, false
		}
		n[i] = v
	}
	return Version{n[0], n[1], n[2]}, true
}

func (v Version) String() string { return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Compare returns -1, 0 or 1 as v is older than, equal to or newer than w.
func (v Version) Compare(w Version) int {
	for _, d := range []int{v.Major - w.Major, v.Minor - w.Minor, v.Patch - w.Patch} {
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
	}
	return 0
}

// TarballName is the release tarball for one platform.
func TarballName(v Version, goos, goarch string) string {
	return fmt.Sprintf("lazygram-%s-%s-%s.tar.gz", v, goos, goarch)
}

// Client talks to the GitHub release pages.
type Client struct {
	HTTP *http.Client
	Base string // https://github.com; tests point it at a local server
	Repo string
}

// New returns a client for the real GitHub.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 2 * time.Minute}, Base: "https://github.com", Repo: Repo}
}

// Latest reads the version from the redirect of releases/latest to
// releases/tag/vX.Y.Z, without following it.
func (c *Client) Latest() (Version, error) {
	hc := *c.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	url := c.Base + "/" + c.Repo + "/releases/latest"
	resp, err := hc.Get(url)
	if err != nil {
		return Version{}, fmt.Errorf("latest release: %w", err)
	}
	_ = resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode/100 != 3 || !strings.Contains(loc, "/releases/tag/") {
		return Version{}, fmt.Errorf("latest release: %s answered %s without a release tag", url, resp.Status)
	}
	tag := path.Base(loc)
	v, ok := Parse(tag)
	if !ok {
		return Version{}, fmt.Errorf("latest release: %q is not a release version", tag)
	}
	return v, nil
}

// Download fetches one file of release v. Redirects (GitHub sends asset
// downloads to its asset host) must stay on HTTPS.
func (c *Client) Download(v Version, name string) ([]byte, error) {
	hc := *c.HTTP
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing redirect to %s", req.URL.Redacted())
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	url := fmt.Sprintf("%s/%s/releases/download/%s/%s", c.Base, c.Repo, v, name)
	resp, err := hc.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFile+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	if len(data) > maxFile {
		return nil, fmt.Errorf("download %s: larger than %d MiB", name, maxFile>>20)
	}
	return data, nil
}

// Verify checks data against its line in a sha256sum-format SHA256SUMS.
func Verify(sums []byte, name string, data []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 2 || strings.TrimPrefix(f[1], "*") != name {
			continue
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(f[0], hex.EncodeToString(sum[:])) {
			return fmt.Errorf("%s: checksum does not match SHA256SUMS", name)
		}
		return nil
	}
	return fmt.Errorf("%s: not listed in SHA256SUMS", name)
}

// fetch downloads name and SHA256SUMS of release v and checks the sum.
func (c *Client) fetch(v Version, name string) ([]byte, error) {
	sums, err := c.Download(v, "SHA256SUMS")
	if err != nil {
		return nil, err
	}
	data, err := c.Download(v, name)
	if err != nil {
		return nil, err
	}
	return data, Verify(sums, name, data)
}

// ExtractBinary returns the regular file DIR/lazygram from a release
// tarball, where DIR is the tarball's name without .tar.gz.
func ExtractBinary(tgz []byte, dir string) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, err
	}
	want := dir + "/lazygram"
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("tarball has no %s", want)
		}
		if err != nil {
			return nil, err
		}
		if h.Name != want {
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("%s in the tarball is not a regular file", want)
		}
		return io.ReadAll(io.LimitReader(tr, maxFile))
	}
}

// Replace writes bin next to path and renames it over path, so path is
// never half written. A running process keeps its old file.
func Replace(path string, bin []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".lazygram-update-*")
	if err != nil {
		return fmt.Errorf("cannot write %s (rerun with sudo, or install the release by hand): %w", dir, err)
	}
	tmp := f.Name()
	_, err = f.Write(bin)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o755)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// Method is how lazygram was installed, and so how it is updated.
type Method string

// Install methods.
const (
	Homebrew Method = "homebrew"
	Binary   Method = "binary"
)

// Executable is the running binary with symlinks resolved, so a Homebrew
// install shows its Cellar path.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// Detect tells the install method from the resolved executable path.
func Detect(exe string) Method {
	if strings.Contains(exe, "/Cellar/lazygram/") {
		return Homebrew
	}
	return Binary
}

// Install installs release v by method. exe is the running binary, which
// the Binary method replaces.
func (c *Client) Install(m Method, v Version, exe string) error {
	if m == Homebrew {
		return interactive("brew", "upgrade", Formula)
	}
	return c.installBinary(v, exe, runtime.GOOS, runtime.GOARCH)
}

func (c *Client) installBinary(v Version, exe, goos, goarch string) error {
	name := TarballName(v, goos, goarch)
	tgz, err := c.fetch(v, name)
	if err != nil {
		return err
	}
	bin, err := ExtractBinary(tgz, strings.TrimSuffix(name, ".tar.gz"))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return Replace(exe, bin)
}

// interactive runs a command on the terminal, so brew shows its progress.
func interactive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
