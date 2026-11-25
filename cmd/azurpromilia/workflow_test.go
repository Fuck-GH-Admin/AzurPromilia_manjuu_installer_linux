package main

import (
	"archive/zip"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func md5Of(data []byte) string { return fmt.Sprintf("%x", md5.Sum(data)) }

func TestSelectPackageTypeFromOriginalDownloadMode(t *testing.T) {
	packages := []Package{
		{Type: 1, URL: "https://cdn.invalid/slice", MD5: md5Of([]byte("s")), Size: 1},
		{Type: 2, URL: "https://cdn.invalid/reality", MD5: md5Of([]byte("r")), Size: 1},
	}
	for _, tc := range []struct {
		mode int
		want int
	}{
		{mode: 0, want: 1},
		{mode: 1, want: 1},
		{mode: 2, want: 2},
	} {
		got, err := selectPackageType(tc.mode, "auto", packages)
		if err != nil || got != tc.want {
			t.Fatalf("downloadMode=%d: got %d, err=%v; want %d", tc.mode, got, err, tc.want)
		}
	}
	if got, err := selectPackageType(1, "reality", packages); err != nil || got != 2 {
		t.Fatalf("explicit reality: got %d, err=%v", got, err)
	}
	if got, err := selectPackageType(2, "slice", packages); err != nil || got != 1 {
		t.Fatalf("explicit slice: got %d, err=%v", got, err)
	}
}

func TestSelectPackageTypeFallsBackOnlyWhenAuto(t *testing.T) {
	packages := []Package{{Type: 2, URL: "https://cdn.invalid/reality", MD5: md5Of([]byte("r")), Size: 1}}
	if got, err := selectPackageType(0, "auto", packages); err != nil || got != 2 {
		t.Fatalf("auto fallback: got %d, err=%v", got, err)
	}
	if _, err := selectPackageType(1, "slice", packages); err == nil {
		t.Fatal("explicit unavailable slice mode should fail")
	}
}

func TestExcludedPath(t *testing.T) {
	exclusions := []ExcludedFile{
		{FilePath: "config/local.ini", FileType: 0},
		{FilePath: "user-data", FileType: 1},
	}
	for _, p := range []string{"config/local.ini", "CONFIG/LOCAL.INI", "user-data/profile.bin", "_index/1.0.0"} {
		if !excludedPath(p, exclusions) {
			t.Fatalf("expected %q to be excluded", p)
		}
	}
	for _, p := range []string{"config/global.ini", "user-data-backup/file.bin", "game.exe"} {
		if excludedPath(p, exclusions) {
			t.Fatalf("unexpected exclusion for %q", p)
		}
	}
}

func TestCleanupOldFilesPreservesExcludedAndCurrentFiles(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		"obsolete.bin": "old",
		"keep.bin": "current",
		"config/local.ini": "user",
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(p, []byte(data), 0644); err != nil { t.Fatal(err) }
	}
	old := Manifest{
		Files: []Entry{
			{Name: "obsolete.bin"},
			{Name: "keep.bin"},
			{Name: "config/local.ini"},
		},
		Directories: []string{"config"},
	}
	current := Manifest{
		Files: []Entry{{Name: "keep.bin"}},
		Excluded: []ExcludedFile{{FilePath: "config/local.ini", FileType: 0}},
	}
	if err := cleanupOldFiles(root, old, current); err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(root, "obsolete.bin")); !os.IsNotExist(err) {
		t.Fatalf("obsolete file still exists or unexpected stat error: %v", err)
	}
	for _, name := range []string{"keep.bin", "config/local.ini"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("expected preserved file %s: %v", name, err)
		}
	}
}

func TestSeedPreloadFilesUsesOnlyVerifiedFiles(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "_pre_download", "2.0")
	dst := filepath.Join(root, "data", "asset.bin")
	src := filepath.Join(stage, "data", "asset.bin")
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(dst, []byte("old"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(src, []byte("new-data"), 0644); err != nil { t.Fatal(err) }
	entry := Entry{Name: "data/asset.bin", Size: int64(len("new-data")), MD5: md5Of([]byte("new-data"))}
	reused, err := seedPreloadFiles(root, stage, []Entry{entry})
	if err != nil { t.Fatal(err) }
	if reused != entry.Size { t.Fatalf("reused %d bytes, want %d", reused, entry.Size) }
	if !verify(dst, entry) { t.Fatal("preload file was not atomically installed") }
	if _, err := os.Stat(src); !os.IsNotExist(err) { t.Fatalf("staged file still exists: %v", err) }
}

func TestZipExtractedEntriesAndVerification(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "game.zip")
	f, err := os.Create(archive)
	if err != nil { t.Fatal(err) }
	zw := zip.NewWriter(f)
	w, err := zw.Create("bin/game.exe")
	if err != nil { t.Fatal(err) }
	if _, err = w.Write([]byte("test game payload")); err != nil { t.Fatal(err) }
	if err = zw.Close(); err != nil { t.Fatal(err) }
	if err = f.Close(); err != nil { t.Fatal(err) }

	entries, err := zipExtractedEntries(archive)
	if err != nil { t.Fatal(err) }
	if len(entries) != 1 || entries[0].Name != "bin/game.exe" {
		t.Fatalf("unexpected extracted entries: %+v", entries)
	}
	dest := filepath.Join(root, "installed")
	if err = extractZIP(archive, dest); err != nil { t.Fatal(err) }
	if err = verifyExtractedEntries(dest, entries); err != nil { t.Fatal(err) }
	if err = os.WriteFile(filepath.Join(dest, "bin/game.exe"), []byte("corrupt"), 0644); err != nil { t.Fatal(err) }
	if err = verifyExtractedEntries(dest, entries); err == nil {
		t.Fatal("corrupted extracted file passed verification")
	}
}


func TestSelectPackageTypeDoesNotOverrideKnownServerMode(t *testing.T) {
	type1 := []Package{{Type: 1, URL: "https://cdn.invalid/slice", MD5: md5Of([]byte("s")), Size: 1}}
	type2 := []Package{{Type: 2, URL: "https://cdn.invalid/reality", MD5: md5Of([]byte("r")), Size: 1}}
	if _, err := selectPackageType(1, "auto", type2); err == nil {
		t.Fatal("downloadMode=SLICE must not silently fall back to REALITY")
	}
	if _, err := selectPackageType(2, "auto", type1); err == nil {
		t.Fatal("downloadMode=REALITY must not silently fall back to SLICE")
	}
}


func TestRunFileWorkersCancelsInFlightAndStopsDispatch(t *testing.T) {
	files := []Entry{{Name: "bad"}, {Name: "slow"}, {Name: "never"}}
	sentinel := errors.New("first file failed")
	var started sync.WaitGroup
	started.Add(2)
	var mu sync.Mutex
	called := make(map[string]int)

	process := func(ctx context.Context, entry Entry) error {
		mu.Lock()
		called[entry.Name]++
		mu.Unlock()
		if entry.Name == "never" {
			return nil
		}
		started.Done()
		if entry.Name == "bad" {
			started.Wait()
			return sentinel
		}
		<-ctx.Done()
		return ctx.Err()
	}

	stats := runFileWorkers(context.Background(), files, 2, true, process)
	if !errors.Is(stats.First, sentinel) {
		t.Fatalf("first error = %v, want %v", stats.First, sentinel)
	}
	if stats.Failed != 1 {
		t.Fatalf("failed = %d, want 1", stats.Failed)
	}
	if stats.Cancelled < 1 {
		t.Fatalf("cancelled = %d, want at least 1", stats.Cancelled)
	}
	mu.Lock()
	defer mu.Unlock()
	if called["never"] != 0 {
		t.Fatalf("queued task started after first failure: %+v", called)
	}
}

func TestRunFileWorkersKeepsScanningWhenStopOnErrorDisabled(t *testing.T) {
	sentinel := errors.New("verification mismatch")
	files := []Entry{{Name: "bad"}, {Name: "good"}}
	var mu sync.Mutex
	called := make(map[string]int)
	stats := runFileWorkers(context.Background(), files, 1, false, func(_ context.Context, entry Entry) error {
		mu.Lock()
		called[entry.Name]++
		mu.Unlock()
		if entry.Name == "bad" {
			return sentinel
		}
		return nil
	})
	if !errors.Is(stats.First, sentinel) || stats.Failed != 1 || stats.Completed != 1 {
		t.Fatalf("unexpected worker stats: %+v", stats)
	}
	if called["bad"] != 1 || called["good"] != 1 {
		t.Fatalf("verification did not inspect all entries: %+v", called)
	}
}

func TestRequiredExtractionBytesCountsMissingAndCorruptFiles(t *testing.T) {
	root := t.TempDir()
	good := []byte("good")
	corrupt := []byte("evil")
	if err := os.WriteFile(filepath.Join(root, "good.bin"), good, 0644); err != nil {
		t.Fatal(err)
	}
	entries := []ExtractedEntry{
		{Name: "good.bin", Size: int64(len(good)), CRC32: crc32.ChecksumIEEE(good)},
		{Name: "corrupt.bin", Size: int64(len(good)), CRC32: crc32.ChecksumIEEE(good)},
		{Name: "missing.bin", Size: 7, CRC32: crc32.ChecksumIEEE([]byte("missing"))},
	}
	if err := os.WriteFile(filepath.Join(root, "corrupt.bin"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}
	required, err := requiredExtractionBytes(root, entries)
	if err != nil {
		t.Fatal(err)
	}
	want := int64(len(good) + 7)
	if required != want {
		t.Fatalf("required bytes = %d, want %d", required, want)
	}
}

func TestZipExtractedEntriesRejectsFileDirectoryConflict(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "conflict.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"a/b.txt", "a"} {
		w, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := w.Write([]byte("payload")); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = zipExtractedEntries(archive); err == nil {
		t.Fatal("accepted a file that conflicts with a directory path")
	}
}
