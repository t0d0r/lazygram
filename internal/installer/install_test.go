// Package installer holds the offline tests of install.sh at the
// repository root. The real script runs under sh (and dash when present)
// with a fake curl first in PATH that serves a fake release from a
// directory, so every path of the script is exercised without the network.
package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const repo = "https://github.com/t0d0r/lazygram"

// fakeCurl answers like GitHub: a HEAD of releases/latest prints the
// redirect named by FAKE_LATEST (no answer when empty), and a download
// copies $FAKE_ROOT/<version>/<file>. Every call is logged.
const fakeCurl = `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_LOG"
head=0; out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    --proto|--proto-redir) shift ;;
    --*) ;;
    -*I*) head=1 ;;
    -*) ;;
    *) url="$1" ;;
  esac
  shift
done
case "$url" in https://*) ;; *) exit 1 ;; esac
if [ "$head" = 1 ]; then
  [ "$url" = "https://github.com/t0d0r/lazygram/releases/latest" ] || exit 22
  [ -n "$FAKE_LATEST" ] || exit 6
  printf 'HTTP/2 302\r\nlocation: %s\r\n\r\n' "$FAKE_LATEST"
  exit 0
fi
f="$FAKE_ROOT/${url#https://github.com/t0d0r/lazygram/releases/download/}"
[ -f "$f" ] || exit 22
cp "$f" "$out"
`

// platform is the release name of this system, as install.sh computes it.
func platform(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("install.sh supports Linux and macOS")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("install.sh supports amd64 and arm64")
	}
	return runtime.GOOS + "-" + runtime.GOARCH
}

// release writes a fake release of version into root: a tarball whose
// lazygram prints reports, and SHA256SUMS with sums (nil: the true one).
func release(t *testing.T, root, version, reports string, sums func(name, sum string) string) {
	t.Helper()
	name := "lazygram-" + version + "-" + platform(t)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	body := "#!/bin/sh\necho '" + reports + "'\n"
	for _, h := range []*tar.Header{
		{Name: name + "/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: name + "/lazygram", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(body))},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = zw.Close()
	dir := filepath.Join(root, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".tar.gz"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	line := hex.EncodeToString(sum[:]) + "  " + name + ".tar.gz\n"
	if sums != nil {
		line = sums(name, hex.EncodeToString(sum[:]))
	}
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

type run struct {
	out     string
	err     error
	log     string
	prefix  string
	install string // path of the installed binary
}

// install runs install.sh with shell, reading the script from a file or,
// like curl | sh, from stdin.
func install(t *testing.T, shell string, stdin bool, root string, env ...string) run {
	t.Helper()
	script, err := filepath.Abs("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	shim := filepath.Join(tmp, "shim")
	if err := os.MkdirAll(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shim, "curl"), []byte(fakeCurl), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(tmp, "prefix")
	log := filepath.Join(tmp, "curl.log")
	var cmd *exec.Cmd
	if stdin {
		cmd = exec.Command(shell)
		f, err := os.Open(script)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		cmd.Stdin = f
	} else {
		cmd = exec.Command(shell, script)
	}
	cmd.Env = append([]string{
		"PATH=" + shim + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + filepath.Join(tmp, "home"),
		"LAZYGRAM_PREFIX=" + prefix,
		"FAKE_ROOT=" + root,
		"FAKE_LOG=" + log,
		"FAKE_LATEST=" + repo + "/releases/tag/v9.9.9",
	}, env...)
	out, err := cmd.CombinedOutput()
	logged, _ := os.ReadFile(log)
	return run{out: string(out), err: err, log: string(logged), prefix: prefix, install: filepath.Join(prefix, "bin", "lazygram")}
}

// shells are the shells install.sh is run with: sh always, dash when
// installed (the strictest common POSIX sh).
func shells() []string {
	out := []string{"sh"}
	if p, err := exec.LookPath("dash"); err == nil {
		out = append(out, p)
	}
	return out
}

func TestInstallsTheLatestReleaseHTTPSOnly(t *testing.T) {
	for _, shell := range shells() {
		for _, stdin := range []bool{false, true} {
			root := t.TempDir()
			release(t, root, "v9.9.9", "lazygram v9.9.9", nil)
			r := install(t, shell, stdin, root)
			if r.err != nil {
				t.Fatalf("%s stdin=%v: %v\n%s", shell, stdin, r.err, r.out)
			}
			got, err := exec.Command(r.install).Output()
			fi, _ := os.Stat(r.install)
			if err != nil || strings.TrimSpace(string(got)) != "lazygram v9.9.9" || fi == nil || fi.Mode().Perm() != 0o755 {
				t.Fatalf("%s: installed binary %q %v %v", shell, got, err, fi)
			}
			for _, want := range []string{"lazygram v9.9.9 for", "checksum ok", "installed " + r.install} {
				if !strings.Contains(r.out, want) {
					t.Fatalf("%s: output lacks %q:\n%s", shell, want, r.out)
				}
			}
			calls := strings.Split(strings.TrimSpace(r.log), "\n")
			if len(calls) != 3 || !strings.Contains(calls[0], "-fsSI") {
				t.Fatalf("%s: one HEAD then two downloads: %q", shell, calls)
			}
			for _, c := range calls {
				if !strings.Contains(c, "--proto =https") || !strings.Contains(c, "--tlsv1.2") {
					t.Fatalf("%s: a call without HTTPS-only flags: %q", shell, c)
				}
			}
			for _, c := range calls[1:] {
				if !strings.Contains(c, "--proto-redir =https") {
					t.Fatalf("%s: a download may follow a redirect off HTTPS: %q", shell, c)
				}
			}
			if left, _ := filepath.Glob(filepath.Join(r.prefix, "bin", ".lazygram-install.*")); len(left) > 0 {
				t.Fatalf("%s: temporary file left: %v", shell, left)
			}
		}
	}
}

func TestAPinnedVersionSkipsTheLatestLookup(t *testing.T) {
	root := t.TempDir()
	release(t, root, "v9.9.8", "lazygram v9.9.8", nil)
	r := install(t, "sh", false, root, "LAZYGRAM_VERSION=v9.9.8")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "-fsSI") || !strings.Contains(r.log, "/v9.9.8/") {
		t.Fatalf("no lookup, the pinned release downloaded: %q", r.log)
	}
}

func TestRefusals(t *testing.T) {
	sumOf := func(sums func(name, sum string) string) func(name, sum string) string { return sums }
	cases := []struct {
		name, want string
		env        []string
		reports    string
		sums       func(name, sum string) string
		downloads  bool // whether the refusal comes after the downloads
	}{
		{name: "stray tag pinned", want: "a release version looks like", env: []string{"LAZYGRAM_VERSION=v0.4.11-1-gd0bdc3f"}},
		{name: "branch pinned", want: "a release version looks like", env: []string{"LAZYGRAM_VERSION=master"}},
		{name: "latest names a stray tag", want: "a release version looks like", env: []string{"FAKE_LATEST=" + repo + "/releases/tag/v0.4.11-1-gd0bdc3f"}},
		{name: "latest points elsewhere", want: "did not name a latest release", env: []string{"FAKE_LATEST=https://evil.example/t0d0r/lazygram/releases/tag/v9.9.9"}},
		{name: "no answer for latest", want: "no answer from GitHub", env: []string{"FAKE_LATEST="}},
		{name: "unknown release", want: "download failed", env: []string{"LAZYGRAM_VERSION=v1.2.3"}},
		{name: "checksum mismatch", want: "checksum mismatch", downloads: true,
			sums: sumOf(func(name, _ string) string { return strings.Repeat("0", 64) + "  " + name + ".tar.gz\n" })},
		{name: "not listed", want: "is not listed", downloads: true,
			sums: sumOf(func(_, sum string) string { return sum + "  something-else.tar.gz\n" })},
		{name: "wrong version inside", want: "refusing to install", downloads: true, reports: "lazygram v1.0.0"},
	}
	for _, c := range cases {
		root := t.TempDir()
		reports := c.reports
		if reports == "" {
			reports = "lazygram v9.9.9"
		}
		release(t, root, "v9.9.9", reports, c.sums)
		r := install(t, "sh", false, root, c.env...)
		if r.err == nil || !strings.Contains(r.out, c.want) {
			t.Fatalf("%s: want a refusal with %q, got %v:\n%s", c.name, c.want, r.err, r.out)
		}
		if _, err := os.Stat(r.install); !os.IsNotExist(err) {
			t.Fatalf("%s: something was installed", c.name)
		}
		if !c.downloads && strings.Contains(r.log, "-o ") && c.want != "download failed" {
			t.Fatalf("%s: refused only after downloading: %q", c.name, r.log)
		}
	}
}
