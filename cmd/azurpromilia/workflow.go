package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
)

type versionInfo struct {
	Version     string `json:"version"`
	VersionName string `json:"versionName"`
}

func runWorkflow(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	installer := fs.String("installer", "", "original Windows installer EXE")
	packageConfig := fs.String("packageconfig", "", "official packageconfig JSON; fetched automatically if omitted")
	root := fs.String("root", "AzurPromilia", "game directory")
	workers := fs.Int("workers", 4, "parallel downloads (1..32)")
	mode := fs.String("mode", "auto", "install package mode: auto, slice, reality")
	python := fs.String("python", "python3", "Python executable for metadata bootstrap")
	wine := fs.String("wine", "wine", "Wine executable for metadata bootstrap")
	refresh := fs.Bool("refresh", false, "refresh cached official package metadata")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *installer == "" {
		return errors.New("--installer is required")
	}
	if *workers < 1 || *workers > 32 {
		return errors.New("--workers must be 1..32")
	}
	if command == "install" && *mode != "auto" && *mode != "slice" && *mode != "reality" {
		return errors.New("--mode must be auto, slice or reality")
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(abs, 0755); err != nil {
		return err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	state := filepath.Join(abs, ".azurpromilia")
	if err = os.MkdirAll(state, 0700); err != nil {
		return err
	}
	configPath, err := ensurePackageConfig(ctx, *installer, *packageConfig, state, *python, *wine, *refresh)
	if err != nil {
		return err
	}
	response, err := readPackageResponse(configPath)
	if err != nil {
		return err
	}
	switch command {
	case "info":
		return printPackageInfo(response)
	case "install":
		return installRelease(ctx, abs, state, *installer, response, *mode, *workers)
	case "update":
		return updateRelease(ctx, abs, state, *installer, response, *workers)
	case "repair":
		return repairRelease(ctx, abs, state, *installer, response, *workers)
	case "preload":
		return preloadRelease(ctx, abs, state, *installer, response, *workers)
	default:
		return fmt.Errorf("unknown workflow command %q", command)
	}
}

func ensurePackageConfig(ctx context.Context, installer, requested, state, python, wine string, refresh bool) (string, error) {
	out := requested
	if out == "" {
		out = filepath.Join(state, "packageconfig.json")
		// Never silently reuse stale server metadata for an update check.
		refresh = true
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	if refresh {
		if err = os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	if st, err := os.Stat(abs); err == nil && st.Mode().IsRegular() {
		return abs, nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return "", err
	}
	script, err := findBootstrapScript()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, python, script, "--installer", installer, "--out", abs, "--wine", wine)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logf("METADATA bootstrap: %s %s", python, script)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("official metadata bootstrap failed: %w", err)
	}
	if st, err := os.Stat(abs); err != nil || !st.Mode().IsRegular() {
		return "", errors.New("bootstrap returned without a packageconfig file")
	}
	return abs, nil
}

func findBootstrapScript() (string, error) {
	exe, _ := os.Executable()
	candidates := []string{
		filepath.Join("tools", "bootstrap.py"),
		filepath.Join(filepath.Dir(exe), "tools", "bootstrap.py"),
		filepath.Join(filepath.Dir(exe), "bootstrap.py"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return filepath.Abs(p)
		}
	}
	return "", errors.New("tools/bootstrap.py not found; run from the source repository or keep tools/ beside the binary")
}

func readPackageResponse(path string) (Response, error) {
	var response Response
	b, err := os.ReadFile(path)
	if err != nil {
		return response, err
	}
	if err = json.Unmarshal(b, &response); err != nil {
		return response, fmt.Errorf("invalid packageconfig JSON: %w", err)
	}
	if response.Code != 0 {
		return response, fmt.Errorf("official package API returned code %d", response.Code)
	}
	if releaseVersion(response.Data.Released) == "" {
		return response, errors.New("packageconfig has no released version")
	}
	return response, nil
}

func releaseVersion(r Release) string {
	s := strings.TrimSpace(string(r.Version))
	if s == "" || s == "null" {
		return ""
	}
	if strings.HasPrefix(s, "\"") {
		var v string
		if json.Unmarshal([]byte(s), &v) == nil {
			return strings.TrimSpace(v)
		}
	}
	return strings.Trim(s, "\"")
}

func selectPackageType(downloadMode int, override string, packages []Package) (int, error) {
	want := 1 // Original launcher enum: SLICE=1, REALITY=2; default is SLICE.
	switch override {
	case "auto":
		if downloadMode == 2 {
			want = 2
		}
	case "slice":
		want = 1
	case "reality":
		want = 2
	default:
		return 0, fmt.Errorf("unknown package mode %q", override)
	}
	has := func(t int) bool {
		for _, p := range packages {
			if p.Type == t && p.URL != "" && validMD5(p.MD5) && p.Size >= 0 {
				return true
			}
		}
		return false
	}
	if has(want) {
		return want, nil
	}
	if override == "auto" {
		if want == 1 && has(2) {
			return 2, nil
		}
		if want == 2 && has(1) {
			return 1, nil
		}
	}
	return 0, fmt.Errorf("server downloadMode=%d selects packageType=%d, but that package is missing or invalid", downloadMode, want)
}

func findPackage(r Release, typ int) (*Package, error) {
	for _, p := range r.Packages {
		if p.Type == typ {
			if p.URL == "" || !validMD5(p.MD5) || p.Size < 0 {
				return nil, fmt.Errorf("packageType=%d has invalid URL, size or MD5", typ)
			}
			v := p
			return &v, nil
		}
	}
	return nil, fmt.Errorf("packageType=%d is not present in official response", typ)
}

func packageCDNs(r Release, p Package) []string {
	if len(r.CDN) > 0 {
		return append([]string(nil), r.CDN...)
	}
	return append([]string(nil), p.CDN...)
}

func fetchManifest(ctx context.Context, root, state, installer string, r Release, typ int) (Manifest, []byte, error) {
	var m Manifest
	pkg, err := findPackage(r, typ)
	if err != nil {
		return m, nil, err
	}
	version := releaseVersion(r)
	if version == "" || !regexp.MustCompile("^[a-zA-Z0-9_.-]+$").MatchString(version) || version == "." || version == ".." {
		return m, nil, errors.New("invalid release version")
	}
	cdn := packageCDNs(r, *pkg)
	indexRoot := filepath.Join(state, "index")
	entry := Entry{Name: version + "-" + fmt.Sprint(typ), Remote: pkg.URL, MD5: pkg.MD5, Size: pkg.Size}
	var network atomic.Int64
	if err = download(ctx, indexRoot, entry, cdn, &network); err != nil {
		return m, nil, fmt.Errorf("download official index: %w", err)
	}
	encrypted, err := os.ReadFile(filepath.Join(indexRoot, entry.Name))
	if err != nil {
		return m, nil, err
	}
	key, iv, err := installerKeys(installer)
	if err != nil {
		return m, nil, err
	}
	plain, err := decrypt(encrypted, key, iv)
	if err != nil {
		return m, nil, fmt.Errorf("decrypt official index: %w", err)
	}
	m, err = parseIndex(plain, typ == 1)
	if err != nil {
		return m, nil, fmt.Errorf("parse official index: %w", err)
	}
	m.Version = version
	m.VersionName = r.Name
	m.PackageType = typ
	m.CDN = cdn
	m.Excluded = append([]ExcludedFile(nil), r.GameExcludedFileList...)
	return m, encrypted, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".azurpromilia-write-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func writeJSONAtomic(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b, 0644)
}

func saveOfficialIndex(root, version string, encrypted []byte) error {
	return writeFileAtomic(filepath.Join(root, "_index", version), encrypted, 0644)
}

func saveVersion(root, state string, m Manifest) error {
	v := versionInfo{Version: m.Version, VersionName: m.VersionName}
	if err := writeJSONAtomic(filepath.Join(root, "_index", "version"), v); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(state, "version.json"), v)
}

func saveManifest(state string, m Manifest) error {
	dir := filepath.Join(state, "manifests")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(dir, fmt.Sprintf("%s-%d.json", m.Version, m.PackageType)), m); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(state, "manifest.json"), m)
}

func loadVersion(root, state string) (versionInfo, error) {
	for _, p := range []string{filepath.Join(root, "_index", "version"), filepath.Join(state, "version.json")} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var v versionInfo
		if json.Unmarshal(b, &v) == nil && v.Version != "" {
			return v, nil
		}
	}
	return versionInfo{}, errors.New("no installed version metadata; run install first")
}

func loadOldManifest(root, state, installer, version string) (Manifest, bool, error) {
	var m Manifest
	if version == "" {
		return m, false, nil
	}
	for _, p := range []string{
		filepath.Join(state, "manifests", version+"-2.json"),
		filepath.Join(state, "manifest.json"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var candidate Manifest
		if json.Unmarshal(b, &candidate) == nil && candidate.Version == version && (len(candidate.Files) > 0 || len(candidate.Extracted) > 0) {
			return candidate, true, nil
		}
	}
	indexPath := filepath.Join(root, "_index", version)
	encrypted, err := os.ReadFile(indexPath)
	if err != nil {
		return m, false, nil
	}
	key, iv, err := installerKeys(installer)
	if err != nil {
		return m, false, err
	}
	plain, err := decrypt(encrypted, key, iv)
	if err != nil {
		return m, false, fmt.Errorf("read previous official index: %w", err)
	}
	m, err = parseIndex(plain, false)
	if err != nil {
		return m, false, fmt.Errorf("parse previous official index: %w", err)
	}
	m.Version = version
	m.PackageType = 2
	return m, true, nil
}

func ensureDirectories(root string, dirs []string) error {
	for _, d := range dirs {
		d = strings.TrimRight(strings.ReplaceAll(d, "\\", "/"), "/")
		if d == "" {
			continue
		}
		p, err := safe(root, d)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}
	return nil
}

func excludedPath(name string, exclusions []ExcludedFile) bool {
	name = strings.ToLower(strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/"))
	if name == "_index" || strings.HasPrefix(name, "_index/") || name == "_pre_download" || strings.HasPrefix(name, "_pre_download/") {
		return true
	}
	for _, x := range exclusions {
		p := strings.ToLower(strings.Trim(strings.ReplaceAll(x.FilePath, "\\", "/"), "/"))
		if p == "" {
			continue
		}
		if x.FileType == 1 {
			if name == p || strings.HasPrefix(name, p+"/") {
				return true
			}
		} else if name == p {
			return true
		}
	}
	return false
}

func cleanupOldFiles(root string, old, current Manifest) error {
	if len(old.Files) == 0 {
		logf("CLEAN skipped: no previous per-file manifest; unknown files will be preserved")
		return nil
	}
	newFiles := make(map[string]bool, len(current.Files))
	for _, f := range current.Files {
		newFiles[strings.ToLower(filepath.ToSlash(f.Name))] = true
	}
	removed := 0
	for _, f := range old.Files {
		key := strings.ToLower(filepath.ToSlash(f.Name))
		if newFiles[key] || excludedPath(f.Name, current.Excluded) {
			continue
		}
		p, err := safe(root, f.Name)
		if err != nil {
			return err
		}
		if err = os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove obsolete file %s: %w", f.Name, err)
		}
		removed++
	}
	newDirs := make(map[string]bool, len(current.Directories))
	for _, d := range current.Directories {
		newDirs[strings.ToLower(filepath.ToSlash(strings.Trim(d, "/\\")))] = true
	}
	for i := len(old.Directories) - 1; i >= 0; i-- {
		d := strings.Trim(old.Directories[i], "/\\")
		key := strings.ToLower(filepath.ToSlash(d))
		if d == "" || newDirs[key] || excludedPath(d, current.Excluded) {
			continue
		}
		p, err := safe(root, d)
		if err != nil {
			return err
		}
		if err = os.Remove(p); err != nil && !os.IsNotExist(err) && !errors.Is(err, os.ErrExist) {
			logf("CLEAN keep directory %s: %v", d, err)
		}
	}
	logf("CLEAN removed %d obsolete files", removed)
	return nil
}

func zipExtractedEntries(path string) ([]ExtractedEntry, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	out := make([]ExtractedEntry, 0, len(z.File))
	seen := make(map[string]bool, len(z.File))
	if len(z.File) > 100000 {
		return nil, fmt.Errorf("ZIP has too many entries: %d", len(z.File))
	}
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("ZIP symlink refused: %s", f.Name)
		}
		name := strings.ReplaceAll(f.Name, "\\", "/")
		key := strings.ToLower(strings.Trim(name, "/"))
		if key == "" || strings.HasPrefix(key, ".azurpromilia/") || key == "_index" || strings.HasPrefix(key, "_index/") || key == "_pre_download" || strings.HasPrefix(key, "_pre_download/") {
			return nil, fmt.Errorf("ZIP uses reserved or empty path: %s", f.Name)
		}
		if seen[key] {
			return nil, fmt.Errorf("ZIP contains duplicate path: %s", f.Name)
		}
		seen[key] = true
		if _, err := safe(os.TempDir(), name); err != nil {
			return nil, err
		}
		out = append(out, ExtractedEntry{Name: name, Size: int64(f.UncompressedSize64), CRC32: f.CRC32})
	}
	return out, nil
}

func verifyExtractedEntries(root string, entries []ExtractedEntry) error {
	var failed int
	for _, e := range entries {
		p, err := safe(root, e.Name)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			failed++
			logf("FAIL extracted %s: %v", e.Name, err)
			continue
		}
		st, statErr := f.Stat()
		h := crc32.NewIEEE()
		n, copyErr := io.Copy(h, f)
		f.Close()
		if statErr != nil || !st.Mode().IsRegular() || copyErr != nil || n != e.Size || h.Sum32() != e.CRC32 {
			failed++
			logf("FAIL extracted %s", e.Name)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d extracted files invalid or missing", failed)
	}
	logf("VERIFY extracted files: %d OK", len(entries))
	return nil
}

func printPackageInfo(response Response) error {
	r := response.Data.Released
	fmt.Printf("Released: %s (%s)\n", releaseVersion(r), r.Name)
	mode := response.Data.DownloadMode
	if mode == 0 {
		mode = 1
	}
	modeName := "slice"
	if mode == 2 {
		modeName = "reality"
	}
	fmt.Printf("Download mode: %d (%s)\n", mode, modeName)
	fmt.Printf("Game folder: %s\nGame entry: %s\n", r.GameFolderName, r.LaunchEntry)
	for _, p := range r.Packages {
		fmt.Printf("Package type %d: %d bytes, %s, CDN hosts=%d\n", p.Type, p.Size, p.MD5, len(packageCDNs(r, p)))
	}
	if v := releaseVersion(response.Data.Preload); v != "" {
		fmt.Printf("Preload: %s (%s)\n", v, response.Data.Preload.Name)
		for _, p := range response.Data.Preload.Packages {
			fmt.Printf("Preload package type %d: %d bytes, %s\n", p.Type, p.Size, p.MD5)
		}
	} else {
		fmt.Println("Preload: none")
	}
	if len(response.Data.GameConfig) > 0 && string(response.Data.GameConfig) != "null" {
		var value any
		if json.Unmarshal(response.Data.GameConfig, &value) == nil {
			b, _ := json.MarshalIndent(value, "", "  ")
			fmt.Printf("Game config metadata: %s\n", b)
		}
	}
	if len(response.Data.LaunchInfo) > 0 && string(response.Data.LaunchInfo) != "null" {
		var value any
		if json.Unmarshal(response.Data.LaunchInfo, &value) == nil {
			b, _ := json.MarshalIndent(value, "", "  ")
			fmt.Printf("Launch metadata (for manual Proton setup): %s\n", b)
		}
	}
	return nil
}

func installRelease(ctx context.Context, root, state, installer string, response Response, mode string, workers int) error {
	if _, err := os.Stat(filepath.Join(root, "_index", "version")); err == nil {
		return errors.New("this directory already has installed-version metadata; use update or repair")
	}
	r := response.Data.Released
	typ, err := selectPackageType(response.Data.DownloadMode, mode, r.Packages)
	if err != nil {
		return err
	}
	m, encrypted, err := fetchManifest(ctx, root, state, installer, r, typ)
	if err != nil {
		return err
	}
	logf("INSTALL version=%s name=%s packageType=%d files=%d directories=%d", m.Version, m.VersionName, typ, len(m.Files), len(m.Directories))
	if typ == 2 {
		if err = runFiles(ctx, root, m, workers, false); err != nil {
			return err
		}
		if err = ensureDirectories(root, m.Directories); err != nil {
			return err
		}
		if err = saveOfficialIndex(root, m.Version, encrypted); err != nil {
			return err
		}
		if err = saveManifest(state, m); err != nil {
			return err
		}
		return saveVersion(root, state, m)
	}

	slices := filepath.Join(state, "slices", m.Version)
	if err = runFiles(ctx, slices, m, workers, false); err != nil {
		return err
	}
	archive := filepath.Join(state, "archives", m.Version+".zip")
	if err = merge(slices, m, archive); err != nil {
		return err
	}
	extracted, err := zipExtractedEntries(archive)
	if err != nil {
		return err
	}
	if err = extractZIP(archive, root); err != nil {
		return err
	}
	if err = verifyExtractedEntries(root, extracted); err != nil {
		return err
	}
	m.Extracted = extracted
	m.Files = nil
	_ = os.RemoveAll(slices)
	_ = os.Remove(archive)
	_ = writeFileAtomic(filepath.Join(state, "index", m.Version+"-1"), encrypted, 0600)

	// The original launcher also fetches the type-2 index after slice installation.
	// It is used as the per-file baseline for subsequent update and repair operations.
	if _, pErr := findPackage(r, 2); pErr == nil {
		if reality, raw, e := fetchManifest(ctx, root, state, installer, r, 2); e == nil {
			if e = saveOfficialIndex(root, reality.Version, raw); e != nil {
				return e
			}
			if e = saveManifest(state, reality); e != nil {
				return e
			}
			return saveVersion(root, state, reality)
		} else {
			logf("WARN type-2 baseline index unavailable after slice install: %v", e)
		}
	}
	if err = saveManifest(state, m); err != nil {
		return err
	}
	return saveVersion(root, state, m)
}

func repairRelease(ctx context.Context, root, state, installer string, response Response, workers int) error {
	local, err := loadVersion(root, state)
	if err != nil {
		return err
	}
	r := response.Data.Released
	version := releaseVersion(r)
	if version != local.Version {
		return fmt.Errorf("installed version is %s but server version is %s; run update instead", local.Version, version)
	}
	m, encrypted, err := fetchManifest(ctx, root, state, installer, r, 2)
	if err != nil {
		return err
	}
	logf("REPAIR version=%s files=%d", m.Version, len(m.Files))
	if err = runFiles(ctx, root, m, workers, false); err != nil {
		return err
	}
	if err = ensureDirectories(root, m.Directories); err != nil {
		return err
	}
	if err = saveOfficialIndex(root, m.Version, encrypted); err != nil {
		return err
	}
	if err = saveManifest(state, m); err != nil {
		return err
	}
	return saveVersion(root, state, m)
}

func updateRelease(ctx context.Context, root, state, installer string, response Response, workers int) error {
	local, err := loadVersion(root, state)
	if err != nil {
		return err
	}
	r := response.Data.Released
	version := releaseVersion(r)
	if version == local.Version {
		return fmt.Errorf("already at latest version %s; use repair to verify and restore files", version)
	}
	m, encrypted, err := fetchManifest(ctx, root, state, installer, r, 2)
	if err != nil {
		return err
	}
	old, hasOld, err := loadOldManifest(root, state, installer, local.Version)
	if err != nil {
		return err
	}
	stage := filepath.Join(root, "_pre_download", m.Version)
	if st, statErr := os.Stat(stage); statErr == nil && st.IsDir() {
		used, seedErr := seedPreloadFiles(root, stage, m.Files)
		if seedErr != nil {
			logf("WARN predownload cache not fully usable: %v", seedErr)
		}
		logf("UPDATE reused %d verified bytes from preload cache", used)
	}
	logf("UPDATE %s -> %s; files=%d", local.Version, m.Version, len(m.Files))
	if err = runFiles(ctx, root, m, workers, false); err != nil {
		return err
	}
	if err = ensureDirectories(root, m.Directories); err != nil {
		return err
	}
	if hasOld {
		if err = cleanupOldFiles(root, old, m); err != nil {
			return err
		}
	} else {
		logf("WARN no previous file manifest; preserving untracked files")
	}
	if err = saveOfficialIndex(root, m.Version, encrypted); err != nil {
		return err
	}
	if err = saveManifest(state, m); err != nil {
		return err
	}
	if err = saveVersion(root, state, m); err != nil {
		return err
	}
	_ = os.RemoveAll(stage)
	_ = os.Remove(filepath.Join(state, "preload-"+m.Version+".json"))
	return nil
}

func seedPreloadFiles(root, stage string, files []Entry) (int64, error) {
	var reused int64
	for _, e := range files {
		src, err := safe(stage, e.Name)
		if err != nil {
			return reused, err
		}
		if !verify(src, e) {
			continue
		}
		dst, err := safe(root, e.Name)
		if err != nil {
			return reused, err
		}
		if verify(dst, e) {
			_ = os.Remove(src)
			reused += e.Size
			continue
		}
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return reused, err
		}
		part := dst + ".part"
		_ = os.Remove(part)
		if err = os.Rename(src, part); err != nil {
			return reused, err
		}
		if !verify(part, e) {
			_ = os.Remove(part)
			return reused, fmt.Errorf("preload file failed verification: %s", e.Name)
		}
		if err = os.Rename(part, dst); err != nil {
			return reused, err
		}
		reused += e.Size
	}
	return reused, nil
}

func preloadRelease(ctx context.Context, root, state, installer string, response Response, workers int) error {
	current := response.Data.Released
	preload := response.Data.Preload
	version := releaseVersion(preload)
	if version == "" {
		return errors.New("server has no preload release")
	}
	if version == releaseVersion(current) {
		return errors.New("preload version is identical to released version")
	}
	cur, curRaw, err := fetchManifest(ctx, root, state, installer, current, 2)
	if err != nil {
		return fmt.Errorf("fetch current release index: %w", err)
	}
	pre, preRaw, err := fetchManifest(ctx, root, state, installer, preload, 2)
	if err != nil {
		return fmt.Errorf("fetch preload index: %w", err)
	}
	if err = saveOfficialIndex(root, cur.Version, curRaw); err != nil { return err }
	if err = saveOfficialIndex(root, pre.Version, preRaw); err != nil { return err }
	currentByName := make(map[string]string, len(cur.Files))
	for _, f := range cur.Files {
		currentByName[strings.ToLower(filepath.ToSlash(f.Name))] = strings.ToLower(f.MD5)
	}
	stage := filepath.Join(root, "_pre_download", pre.Version)
	if err = os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	diff := make([]Entry, 0, len(pre.Files))
	var unchanged int
	for _, f := range pre.Files {
		if currentByName[strings.ToLower(filepath.ToSlash(f.Name))] == strings.ToLower(f.MD5) {
			unchanged++
			continue
		}
		p, e := safe(stage, f.Name)
		if e != nil {
			return e
		}
		if verify(p, f) {
			continue
		}
		diff = append(diff, f)
	}
	pre.Files = diff
	logf("PRELOAD version=%s changed=%d unchanged=%d downloadBytes=%d", pre.Version, len(diff), unchanged, totalSize(diff))
	if len(diff) > 0 {
		if err = runFiles(ctx, stage, pre, workers, false); err != nil {
			return err
		}
	}
	if err = writeJSONAtomic(filepath.Join(state, "preload-"+pre.Version+".json"), pre); err != nil {
		return err
	}
	logf("PRELOAD READY %s", pre.Version)
	return nil
}

func totalSize(files []Entry) int64 {
	var n int64
	for _, f := range files {
		n += f.Size
	}
	return n
}
