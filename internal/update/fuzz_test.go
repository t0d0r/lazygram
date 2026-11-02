package update

import (
	"archive/tar"
	"bytes"
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// FuzzParse feeds Parse version strings: the built-in one (git describe
// output, "dev", Homebrew's bare X.Y.Z) and the tag of the latest release
// from GitHub. A release version is three runs of ASCII digits with an
// optional leading v, so anything else (a git describe suffix, -dirty,
// blanks, a newline, digits of another script) is refused, and a refusal
// is the zero Version. An accepted version says what the string says:
// each part is the number written there, String parses back to the same
// version with or without its v, and its tarball name is one plain URL
// path element and one SHA256SUMS field.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"v0.3.0", "0.3.0", "v1.12.7", "dev", "", "v", "v..", "v0.3", "v1.2.3.4",
		"v0.3.0-2-gabc1234", "v0.3.0-dirty", "v0.4.21-1-gf449972", "abc1234",
		"vv1.2.3", "V1.2.3", "v1.2.3\n", " v1.2.3", "v1.2.3 ", "v1.2.3\x00", "v-1.2.3",
		"v+1.2.3", "v1.2.3+meta", "v01.002.0003", "v0.0.0", "v9223372036854775807.0.0",
		"v9223372036854775808.0.0", "v0.0.99999999999999999999", "v١.٢.٣", "v１.２.３",
		"в1.2.3", "v1.2.3\u0301", "v1.2.3🎉", "v1.2.3\x1b[0m", "v1.2.3\u009b", "v1.2\xff.3",
		"v" + strings.Repeat("9", 4096) + ".0.0", "v1." + strings.Repeat("0", 4096) + "7.0",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, ok := Parse(s)
		if !ok {
			if v != (Version{}) {
				t.Fatalf("Parse(%q) refused but returned %v", s, v)
			}
			return
		}
		parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
		if len(parts) != 3 {
			t.Fatalf("Parse(%q) = %v accepts %d parts", s, v, len(parts))
		}
		for i, n := range []int{v.Major, v.Minor, v.Patch} {
			digits := strings.TrimLeft(parts[i], "0")
			if digits == "" {
				digits = "0"
			}
			if strings.Trim(parts[i], "0123456789") != "" || strconv.Itoa(n) != digits {
				t.Fatalf("Parse(%q) = %v: part %q read as %d", s, v, parts[i], n)
			}
		}
		if w, ok := Parse(v.String()); !ok || w != v {
			t.Fatalf("Parse(%q) = %v, but Parse(%q) = %v, %v", s, v, v.String(), w, ok)
		}
		if w, ok := Parse(strings.TrimPrefix(v.String(), "v")); !ok || w != v {
			t.Fatalf("Homebrew form of %v parses as %v, %v", v, w, ok)
		}
		name := TarballName(v, runtime.GOOS, runtime.GOARCH)
		if strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789.-") != "" {
			t.Fatalf("tarball name %q for %v is not a plain path element", name, v)
		}
	})
}

// FuzzCompare checks that Compare orders every pair of release versions
// Parse can return (each part from 0 to the largest int; a larger number
// is refused, never wrapped) by major, then minor, then patch, and is
// antisymmetric, so "up to date" and "update available" never both hold.
func FuzzCompare(f *testing.F) {
	f.Add(uint64(0), uint64(2), uint64(1), uint64(0), uint64(3), uint64(0))
	f.Add(uint64(0), uint64(10), uint64(0), uint64(0), uint64(9), uint64(9))
	f.Add(uint64(1), uint64(0), uint64(0), uint64(0), uint64(99), uint64(99))
	f.Add(uint64(0), uint64(3), uint64(0), uint64(0), uint64(3), uint64(0))
	f.Add(uint64(math.MaxInt64), uint64(0), uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(0), uint64(math.MaxInt64), uint64(1), uint64(0), uint64(math.MaxInt64), uint64(0))
	f.Add(uint64(math.MaxInt64)+1, uint64(0), uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(math.MaxUint64), uint64(math.MaxUint64), uint64(math.MaxUint64), uint64(0), uint64(0), uint64(0))
	f.Fuzz(func(t *testing.T, a1, a2, a3, b1, b2, b3 uint64) {
		parse := func(x, y, z uint64) (Version, bool) {
			s := fmt.Sprintf("v%d.%d.%d", x, y, z)
			v, ok := Parse(s)
			fits := x <= math.MaxInt && y <= math.MaxInt && z <= math.MaxInt
			if ok != fits || ok && v != (Version{int(x), int(y), int(z)}) {
				t.Fatalf("Parse(%q) = %v, %v", s, v, ok)
			}
			return v, ok
		}
		a, okA := parse(a1, a2, a3)
		b, okB := parse(b1, b2, b3)
		if !okA || !okB {
			return
		}
		want := cmp.Or(cmp.Compare(a.Major, b.Major), cmp.Compare(a.Minor, b.Minor), cmp.Compare(a.Patch, b.Patch))
		if got := a.Compare(b); got != want || b.Compare(a) != -want || a.Compare(a) != 0 {
			t.Fatalf("%v.Compare(%v) = %d, reverse %d, want %d", a, b, got, b.Compare(a), want)
		}
	})
}

// FuzzVerify feeds Verify a SHA256SUMS as the release serves it and a
// file name. A download passes only when SHA256SUMS lists its digest for
// that name: a nil error means some line of sums is the digest of data,
// in either case, and name or *name, so a digest listed under another
// release file never passes. A file that sha256sum can list in one field
// (no blank in its name, no leading *) passes when its line comes first,
// in text or binary mode and with the digest in either case.
func FuzzVerify(f *testing.F) {
	sum := sha256.Sum256([]byte("binary"))
	good := hex.EncodeToString(sum[:])
	for _, c := range []struct{ sums, name, data string }{
		{good + "  a.tar.gz\n", "a.tar.gz", "binary"},
		{good + " *a.tar.gz\r\n", "a.tar.gz", "binary"},
		{strings.ToUpper(good) + "\ta.tar.gz", "a.tar.gz", "binary"},
		{strings.Repeat("0", 64) + "  a.tar.gz\n" + good + "  a.tar.gz\n", "a.tar.gz", "binary"},
		{good + "  a.tar.gz extra\n", "a.tar.gz", "binary"},
		{good + "  *a.tar.gz\n", "*a.tar.gz", "binary"},
		{good + "  \n", "", "binary"},
		{"", "", ""},
		{"\x1b[31m" + good + "  a.tar.gz\x1b[0m\n", "a.tar.gz", "binary"},
		{good + "  файл.tar.gz\n", "файл.tar.gz", "binary"},
		{good + "  e\u0301🎉.tar.gz\n", "e\u0301🎉.tar.gz", "binary"},
		{good + "  a\u00a0b\n", "a\u00a0b", "binary"},
		{good + "  \xff\xfe.tar.gz\n", "\xff\xfe.tar.gz", "binary"},
		{strings.Repeat("x", 70000) + "\n" + good + "  a.tar.gz\n", "a.tar.gz", "binary"},
		// The digest of data under another name, or in the wrong field.
		{good + "  b.tar.gz\n", "a.tar.gz", "binary"},
		{good + "  b.tar.gz\n" + strings.Repeat("0", 64) + "  a.tar.gz\n", "a.tar.gz", "binary"},
		{good + "  a.tar.gz.sig\n", "a.tar.gz", "binary"},
		{"a.tar.gz  " + good + "\n", "a.tar.gz", "binary"},
	} {
		f.Add([]byte(c.sums), c.name, []byte(c.data))
	}
	f.Fuzz(func(t *testing.T, sums []byte, name string, data []byte) {
		d := sha256.Sum256(data)
		digest := hex.EncodeToString(d[:])
		listed := func() bool {
			for _, line := range strings.Split(string(sums), "\n") {
				if w := strings.Fields(line); len(w) == 2 && strings.ToLower(w[0]) == digest && (w[1] == name || w[1] == "*"+name) {
					return true
				}
			}
			return false
		}
		if Verify(sums, name, data) == nil && !listed() {
			t.Fatalf("%q passed without a line for it with its digest %s in SHA256SUMS %q", name, digest, sums)
		}
		if fields := strings.Fields(name); len(fields) != 1 || fields[0] != name || strings.HasPrefix(name, "*") || len(name) > 4096 {
			return
		}
		for _, line := range []string{digest + "  " + name + "\n", strings.ToUpper(digest) + " *" + name + "\r\n"} {
			if err := Verify(append([]byte(line), sums...), name, data); err != nil {
				t.Fatalf("%q listed first as %q: %v", name, line, err)
			}
		}
	})
}

// seedTarball builds a release-like tarball for the seed corpus. Stored
// (uncompressed) deflate blocks keep the tar headers visible to the
// fuzzer's byte mutations.
func seedTarball(level int, entries ...entry) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, level)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o755, Size: int64(len(e.body))}
		if e.typ != tar.TypeReg {
			h.Size, h.Linkname = 0, "/etc/passwd"
		}
		_ = tw.WriteHeader(h)
		_, _ = tw.Write([]byte(e.body))
	}
	_ = tw.Close()
	_ = zw.Close()
	return buf.Bytes()
}

// FuzzExtractBinary feeds ExtractBinary a downloaded tarball. What it
// returns is the content of the archive's first DIR/lazygram, which is a
// regular file, in full: as many bytes as its header says, never more
// than the download cap.
func FuzzExtractBinary(f *testing.F) {
	const dir = "lazygram-v0.4.1-linux-amd64"
	for _, level := range []int{gzip.NoCompression, gzip.BestSpeed} {
		f.Add(seedTarball(level, entry{dir + "/README.md", tar.TypeReg, "readme"}, entry{dir + "/lazygram", tar.TypeReg, "new binary"}))
		f.Add(seedTarball(level, entry{dir + "/lazygram", tar.TypeSymlink, ""}))
		f.Add(seedTarball(level, entry{dir + "/lazygram", tar.TypeDir, ""}, entry{dir + "/lazygram", tar.TypeReg, "x"}))
		f.Add(seedTarball(level, entry{"lazygram-v0.4.1-darwin-arm64/lazygram", tar.TypeReg, "other"}))
		f.Add(seedTarball(level, entry{"./" + dir + "/lazygram", tar.TypeReg, "dot"}, entry{dir + "//lazygram", tar.TypeReg, "slash"}))
		f.Add(seedTarball(level, entry{dir + "/" + strings.Repeat("ж", 80) + "/lazygram", tar.TypeReg, "long name"}))
		f.Add(seedTarball(level))
	}
	whole := seedTarball(gzip.NoCompression, entry{dir + "/lazygram", tar.TypeReg, strings.Repeat("\x7fELF", 300)})
	f.Add(whole[:len(whole)/2])
	// An entry one byte over the cap, built here rather than stored: it
	// was cut to the cap and returned without an error.
	f.Add(zeroTarball(f, dir, maxFile+1))
	f.Add([]byte{})
	f.Add([]byte("not a tarball"))
	f.Add([]byte{0x1f, 0x8b})
	f.Fuzz(func(t *testing.T, tgz []byte) {
		bin, err := ExtractBinary(tgz, dir)
		if err != nil {
			return
		}
		if len(bin) > maxFile {
			t.Fatalf("returned %d bytes, over the %d byte cap", len(bin), maxFile)
		}
		zr, err := gzip.NewReader(bytes.NewReader(tgz))
		if err != nil {
			t.Fatalf("returned %d bytes from a stream that is not gzip: %v", len(bin), err)
		}
		tr := tar.NewReader(zr)
		for {
			h, err := tr.Next()
			if err != nil {
				t.Fatalf("returned %d bytes, but the archive has no %s/lazygram: %v", len(bin), dir, err)
			}
			if h.Name != dir+"/lazygram" {
				continue
			}
			body, _ := io.ReadAll(tr)
			if h.Typeflag != tar.TypeReg || h.Size != int64(len(bin)) || !bytes.Equal(body, bin) {
				t.Fatalf("returned %d bytes for a %c entry of %d bytes", len(bin), h.Typeflag, h.Size)
			}
			return
		}
	})
}
