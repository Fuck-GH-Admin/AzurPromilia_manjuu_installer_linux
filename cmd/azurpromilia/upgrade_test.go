package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSelectReleaseHonorsStable(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v2.0.0-preview.1", Prerelease: true},
		{TagName: "v1.9.0"},
	}
	got, err := selectRelease(releases, false)
	if err != nil || got.TagName != "v2.0.0-preview.1" {
		t.Fatalf("preview selection=%+v err=%v", got, err)
	}
	got, err = selectRelease(releases, true)
	if err != nil || got.TagName != "v1.9.0" {
		t.Fatalf("stable selection=%+v err=%v", got, err)
	}
}

func TestAssetDigest(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got := assetDigest(githubReleaseAsset{Digest: "sha256:" + hash}); got != hash {
		t.Fatalf("digest=%q", got)
	}
	if got := assetDigest(githubReleaseAsset{Digest: "md5:" + hash}); got != "" {
		t.Fatalf("unexpected non-sha256 digest=%q", got)
	}
}

func TestReleaseAssetNameMatchesRuntime(t *testing.T) {
	name, err := releaseAssetName("v1.2.3")
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		if err == nil {
			t.Fatalf("expected unsupported platform error, got %q", name)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	want := "azurpromilia-v1.2.3-linux-" + runtime.GOARCH + ".tar.gz"
	if name != want {
		t.Fatalf("asset=%q want=%q", name, want)
	}
}

type archiveEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
	mode     int64
}

func writeReleaseArchive(t *testing.T, entries []archiveEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "release.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		h := &tar.Header{
			Name:     entry.name,
			Typeflag: entry.typeflag,
			Linkname: entry.linkname,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
		}
		if entry.typeflag == tar.TypeDir || entry.typeflag == tar.TypeSymlink {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSafeExtractRelease(t *testing.T) {
	archive := writeReleaseArchive(t, []archiveEntry{
		{name: "azurpromilia-v1-linux-amd64/", typeflag: tar.TypeDir, mode: 0755},
		{name: "azurpromilia-v1-linux-amd64/azurpromilia", body: "binary", typeflag: tar.TypeReg, mode: 0755},
		{name: "azurpromilia-v1-linux-amd64/azp", typeflag: tar.TypeSymlink, linkname: "azurpromilia", mode: 0777},
		{name: "azurpromilia-v1-linux-amd64/tools/", typeflag: tar.TypeDir, mode: 0755},
		{name: "azurpromilia-v1-linux-amd64/tools/bootstrap.py", body: "print('ok')", typeflag: tar.TypeReg, mode: 0644},
	})
	dst := t.TempDir()
	if err := safeExtractRelease(archive, dst); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dst, "azurpromilia"))
	if err != nil || string(b) != "binary" {
		t.Fatalf("extracted binary=%q err=%v", b, err)
	}
	if _, err := os.Lstat(filepath.Join(dst, "azp")); !os.IsNotExist(err) {
		t.Fatalf("extractor should validate but not recreate archive symlink: %v", err)
	}
}

func TestSafeExtractReleaseRejectsTraversalAndUnexpectedSymlink(t *testing.T) {
	for name, entries := range map[string][]archiveEntry{
		"traversal": {
			{name: "pkg/", typeflag: tar.TypeDir, mode: 0755},
			{name: "pkg/../escape", body: "x", typeflag: tar.TypeReg, mode: 0644},
		},
		"symlink": {
			{name: "pkg/", typeflag: tar.TypeDir, mode: 0755},
			{name: "pkg/azurpromilia", body: "binary", typeflag: tar.TypeReg, mode: 0755},
			{name: "pkg/evil", typeflag: tar.TypeSymlink, linkname: "/etc/passwd", mode: 0777},
		},
	} {
		t.Run(name, func(t *testing.T) {
			archive := writeReleaseArchive(t, entries)
			err := safeExtractRelease(archive, t.TempDir())
			if err == nil {
				t.Fatal("unsafe archive was accepted")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("unsafe archive returned an empty error")
			}
		})
	}
}
