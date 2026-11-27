package main

import (
	"archive/tar"
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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/Fuck-GH-Admin/AzurPromilia_manjuu_installer_linux/releases?per_page=20"

var toolVersion = "dev"

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	Draft      bool                 `json:"draft"`
	Prerelease bool                 `json:"prerelease"`
	Assets     []githubReleaseAsset `json:"assets"`
}

func releaseHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 3 * time.Minute,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 20 * time.Second,
			IdleConnTimeout:        30 * time.Second,
		},
	}
}

func fetchReleases(ctx context.Context) ([]githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil { return nil, err }
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "azurpromilia-linux/"+toolVersion)
	resp, err := releaseHTTPClient().Do(req)
	if err != nil { return nil, fmt.Errorf("check GitHub releases: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub release API returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil { return nil, err }
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("decode GitHub releases: %w", err)
	}
	return releases, nil
}

func selectRelease(releases []githubRelease, stable bool) (githubRelease, error) {
	for _, r := range releases {
		if r.Draft || strings.TrimSpace(r.TagName) == "" { continue }
		if stable && r.Prerelease { continue }
		return r, nil
	}
	if stable { return githubRelease{}, errors.New("no stable GitHub release is available") }
	return githubRelease{}, errors.New("no GitHub release is available")
}

func releaseAsset(release githubRelease, name string) (githubReleaseAsset, bool) {
	for _, a := range release.Assets {
		if a.Name == name { return a, true }
	}
	return githubReleaseAsset{}, false
}

func releaseAssetName(tag string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("self-upgrade currently supports Linux only, not %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", fmt.Errorf("no packaged release for linux/%s", runtime.GOARCH)
	}
	return fmt.Sprintf("azurpromilia-%s-linux-%s.tar.gz", tag, runtime.GOARCH), nil
}

func assetDigest(asset githubReleaseAsset) string {
	const p = "sha256:"
	if !strings.HasPrefix(strings.ToLower(asset.Digest), p) { return "" }
	v := strings.TrimSpace(asset.Digest[len(p):])
	if len(v) != sha256.Size*2 { return "" }
	if _, err := hex.DecodeString(v); err != nil { return "" }
	return strings.ToLower(v)
}

func fetchChecksumFile(ctx context.Context, release githubRelease, assetName string) (string, error) {
	sums, ok := releaseAsset(release, "SHA256SUMS")
	if !ok { return "", errors.New("release has no asset digest or SHA256SUMS") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sums.BrowserDownloadURL, nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", "azurpromilia-linux/"+toolVersion)
	resp, err := releaseHTTPClient().Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SHA256SUMS returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil { return "", err }
	for _, line := range strings.Split(string(body), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 { continue }
		name := strings.TrimPrefix(f[len(f)-1], "*")
		if name != assetName || len(f[0]) != sha256.Size*2 { continue }
		if _, err := hex.DecodeString(f[0]); err == nil { return strings.ToLower(f[0]), nil }
	}
	return "", fmt.Errorf("SHA256SUMS does not contain %s", assetName)
}

func downloadReleaseAsset(ctx context.Context, asset githubReleaseAsset, dst string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", "azurpromilia-linux/"+toolVersion)
	resp, err := releaseHTTPClient().Do(req)
	if err != nil { return "", fmt.Errorf("download release asset: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release asset returned HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil { return "", err }
	ok := false
	defer func() {
		_ = f.Close()
		if !ok { _ = os.Remove(dst) }
	}()

	h := sha256.New()
	var n int64
	total := asset.Size
	if total <= 0 { total = resp.ContentLength }
	lastAt, lastBytes := time.Now(), int64(0)
	buf := make([]byte, 1<<20)
	for {
		read, readErr := resp.Body.Read(buf)
		if read > 0 {
			n += int64(read)
			if asset.Size > 0 && n > asset.Size { return "", errors.New("release asset exceeded declared size") }
			if _, err := f.Write(buf[:read]); err != nil { return "", err }
			_, _ = h.Write(buf[:read])
			now := time.Now()
			if now.Sub(lastAt) >= progressInterval() {
				elapsed := now.Sub(lastAt).Seconds()
				speed := 0.0
				if elapsed > 0 { speed = float64(n-lastBytes) / elapsed }
				renderTransferProgress("工具更新", n, total, 0, 0, 1, speed)
				lastAt, lastBytes = now, n
			}
		}
		if readErr == io.EOF { break }
		if readErr != nil { return "", readErr }
	}
	if asset.Size > 0 && n != asset.Size {
		return "", fmt.Errorf("release asset size mismatch: got %d want %d", n, asset.Size)
	}
	if err := f.Sync(); err != nil { return "", err }
	if err := f.Close(); err != nil { return "", err }
	ok = true
	finishTransferProgress("工具更新", 1, 0, 1, n, nil)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func safeExtractRelease(archivePath, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil { return err }
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil { return err }
	defer gz.Close()
	tr := tar.NewReader(gz)

	var root string
	var extracted int64
	const limit = int64(256 << 20)
	for {
		h, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { return err }
		name := strings.TrimPrefix(filepath.ToSlash(h.Name), "./")
		if h.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		parts := strings.Split(name, "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == "." || parts[0] == ".." {
			return fmt.Errorf("unsafe release archive path %q", h.Name)
		}
		if root == "" { root = parts[0] } else if root != parts[0] {
			return errors.New("release archive contains multiple top-level roots")
		}
		if len(parts) == 1 {
			if h.Typeflag != tar.TypeDir { return errors.New("release archive root is not a directory") }
			continue
		}
		rel := strings.Join(parts[1:], "/")
		for _, p := range strings.Split(rel, "/") {
			if p == "" || p == "." || p == ".." { return fmt.Errorf("unsafe release archive path %q", h.Name) }
		}
		target, err := safe(dst, rel)
		if err != nil { return err }
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil { return err }
		case tar.TypeReg, tar.TypeRegA:
			if h.Size < 0 || h.Size > limit-extracted { return errors.New("release archive is unexpectedly large") }
			extracted += h.Size
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil { return err }
			mode := os.FileMode(h.Mode).Perm()
			if mode == 0 { mode = 0644 }
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil { return err }
			copied, copyErr := io.Copy(out, io.LimitReader(tr, h.Size+1))
			closeErr := out.Close()
			if copyErr != nil { return copyErr }
			if closeErr != nil { return closeErr }
			if copied != h.Size { return fmt.Errorf("release file %s size mismatch", rel) }
		case tar.TypeSymlink:
			if rel != "azp" || h.Linkname != "azurpromilia" {
				return fmt.Errorf("unexpected symlink in release archive: %s -> %s", rel, h.Linkname)
			}
		default:
			return fmt.Errorf("unsupported release archive entry type for %s", rel)
		}
	}
	if root == "" { return errors.New("empty release archive") }
	st, err := os.Stat(filepath.Join(dst, "azurpromilia"))
	if err != nil || !st.Mode().IsRegular() { return errors.New("release archive has no azurpromilia binary") }
	return nil
}

func packagedExecutableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil { return "", err }
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil { return "", err }
	if filepath.Base(exe) != "azurpromilia" {
		return "", fmt.Errorf("self-upgrade requires a release package; current executable is %s", exe)
	}
	dir := filepath.Dir(exe)
	st, err := os.Stat(filepath.Join(dir, "tools", "bootstrap.py"))
	if err != nil || !st.Mode().IsRegular() {
		return "", errors.New("self-upgrade requires tools/bootstrap.py beside the packaged binary")
	}
	return dir, nil
}

func copyUpgradeFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { return err }
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".azp-upgrade-*")
	if err != nil { return err }
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok { _ = os.Remove(tmpName) }
	}()
	if err := tmp.Chmod(mode); err != nil { return err }
	if _, err := io.Copy(tmp, in); err != nil { return err }
	if err := tmp.Sync(); err != nil { return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(tmpName, dst); err != nil { return err }
	ok = true
	return nil
}

func installReleaseFiles(srcDir, dstDir string) error {
	for _, rel := range []string{
		"tools/bootstrap.py",
		"tools/fetch_packageconfig.cjs",
		"tools/steam-launch-options.example",
		"README.md",
		"LICENSE",
		"SHA256SUMS",
	} {
		src := filepath.Join(srcDir, filepath.FromSlash(rel))
		if _, err := os.Stat(src); err != nil { return fmt.Errorf("release package missing %s", rel) }
		if err := copyUpgradeFile(src, filepath.Join(dstDir, filepath.FromSlash(rel)), 0644); err != nil {
			return fmt.Errorf("update %s: %w", rel, err)
		}
	}

	newBin := filepath.Join(srcDir, "azurpromilia")
	cmd := exec.Command(newBin, "--version")
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("new binary self-check failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	if entries, err := os.ReadDir(filepath.Join(srcDir, "docs")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() { continue }
			src := filepath.Join(srcDir, "docs", entry.Name())
			st, err := os.Stat(src)
			if err != nil || !st.Mode().IsRegular() { continue }
			if err := copyUpgradeFile(src, filepath.Join(dstDir, "docs", entry.Name()), st.Mode().Perm()); err != nil {
				warnf("documentation update skipped for %s: %v", entry.Name(), err)
			}
		}
	}
	return copyUpgradeFile(newBin, filepath.Join(dstDir, "azurpromilia"), 0755)
}

func runUpgradeCommand(args []string) error {
	var check, stable, force bool
	for _, arg := range args {
		switch arg {
		case "--check": check = true
		case "--stable": stable = true
		case "--force": force = true
		case "-h", "--help":
			printCommandHelp(os.Stdout, cliProgramName(), "upgrade")
			return nil
		default:
			return fmt.Errorf("unknown upgrade option %q", arg)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	releases, err := fetchReleases(ctx)
	if err != nil { return err }
	release, err := selectRelease(releases, stable)
	if err != nil { return err }

	fmt.Printf("工具版本: %s\n", toolVersion)
	fmt.Printf("最新发布: %s", release.TagName)
	if release.Prerelease { fmt.Print(" (pre-release)") }
	fmt.Println()

	if toolVersion == release.TagName && !force {
		fmt.Println("已经是当前发布版本。")
		return nil
	}
	if check {
		if toolVersion == "dev" {
			fmt.Println("当前是开发构建；只报告 Release，不自动判断它是否比 main 更新。")
		} else {
			fmt.Println("有不同的 Release 可用；运行 azp upgrade 执行更新。")
		}
		return nil
	}
	if toolVersion == "dev" && !force {
		return errors.New("refusing to replace a development build with a published release; use azp upgrade --force if intentional")
	}

	name, err := releaseAssetName(release.TagName)
	if err != nil { return err }
	asset, ok := releaseAsset(release, name)
	if !ok { return fmt.Errorf("release %s has no asset %s", release.TagName, name) }

	expected := assetDigest(asset)
	if expected == "" {
		expected, err = fetchChecksumFile(ctx, release, name)
		if err != nil { return err }
	}
	targetDir, err := packagedExecutableDir()
	if err != nil { return err }

	tmpDir, err := os.MkdirTemp("", "azp-upgrade-*")
	if err != nil { return err }
	defer os.RemoveAll(tmpDir)
	archive := filepath.Join(tmpDir, name)
	actual, err := downloadReleaseAsset(ctx, asset, archive)
	if err != nil { return err }
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("release SHA-256 mismatch: got %s want %s", actual, expected)
	}
	infof("release archive SHA-256 verified: %s", actual)

	extracted := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extracted, 0700); err != nil { return err }
	if err := safeExtractRelease(archive, extracted); err != nil { return fmt.Errorf("extract release: %w", err) }
	if err := installReleaseFiles(extracted, targetDir); err != nil { return err }
	fmt.Printf("azp 已更新到 %s。\n", release.TagName)
	fmt.Println("当前进程仍是旧版本；下一次运行会使用新版本。")
	return nil
}
