package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestCombinedSpaceRequirementIncludesFutureArchive(t *testing.T) {
	const archiveBytes int64 = 75145275761
	required, err := combinedSpaceRequirement(archiveBytes, archiveBytes)
	if err != nil {
		t.Fatal(err)
	}
	const want int64 = 150290551522
	if required != want {
		t.Fatalf("combined requirement = %d, want %d", required, want)
	}
	available := uint64(94*(1<<30) + 98*(1<<30)/100)
	if err := checkAvailableSpace(uint64(required), available, "SLICE download + ZIP merge"); err == nil {
		t.Fatal("accepted a disk budget that cannot hold both slices and the merged ZIP")
	}
}

func TestCombinedSpaceRequirementRejectsNegativeAndOverflow(t *testing.T) {
	if _, err := combinedSpaceRequirement(1, -1); err == nil {
		t.Fatal("accepted a negative reserve")
	}
	if _, err := combinedSpaceRequirement(maxInt64-1, 2); err == nil {
		t.Fatal("accepted an overflowing combined requirement")
	}
}

func TestCheckAvailableSpace(t *testing.T) {
	if err := checkAvailableSpace(5, 5, "test"); err != nil {
		t.Fatalf("exactly sufficient space rejected: %v", err)
	}
	if err := checkAvailableSpace(6, 5, "test"); err == nil {
		t.Fatal("insufficient space was accepted")
	}
}

func TestPreflightFilesWithReserveRejectsOverflowBeforeDispatch(t *testing.T) {
	root := t.TempDir()
	entries := []Entry{{Name: "asset.bin", Size: 2, MD5: md5Of([]byte("ok"))}}
	if _, _, err := preflightFilesWithReserve(root, entries, maxInt64-1, "SLICE download + ZIP merge"); err == nil {
		t.Fatal("accepted overflowing download plus archive reserve")
	}
}

func TestPreflightFilesCountsOnlyMissingOrInvalidResources(t *testing.T) {
	root := t.TempDir()
	good := []byte("good")
	corrupt := []byte("evil")
	if err := os.WriteFile(filepath.Join(root, "good.bin"), good, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corrupt.bin"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Name: "good.bin", Size: int64(len(good)), MD5: md5Of(good)},
		{Name: "corrupt.bin", Size: int64(len(good)), MD5: md5Of(good)},
		{Name: "missing.bin", Size: 11, MD5: md5Of([]byte("missingfile"))},
	}
	valid, required, err := preflightFiles(root, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !valid["good.bin"] || valid["corrupt.bin"] || valid["missing.bin"] {
		t.Fatalf("unexpected verified-file map: %#v", valid)
	}
	if want := int64(len(good) + 11); required != want {
		t.Fatalf("required bytes = %d, want %d", required, want)
	}
}

func TestPreflightFilesRejectsNonRegularDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "asset.bin"), 0755); err != nil {
		t.Fatal(err)
	}
	_, _, err := preflightFiles(root, []Entry{{
		Name: "asset.bin", Size: 1, MD5: md5Of([]byte("x")),
	}})
	if err == nil {
		t.Fatal("directory accepted as a file destination")
	}
}

func TestPreflightFilesRejectsSizeOverflow(t *testing.T) {
	root := t.TempDir()
	entries := []Entry{
		{Name: "one.bin", Size: maxInt64, MD5: md5Of(nil)},
		{Name: "two.bin", Size: 1, MD5: md5Of(nil)},
	}
	if _, _, err := preflightFiles(root, entries); err == nil {
		t.Fatal("overflowing download size estimate was accepted")
	}
}


func TestPreflightFilesReusesCompleteVerifiedPart(t *testing.T) {
	root := t.TempDir()
	data := []byte("already downloaded")
	entry := Entry{Name: "asset.bin", Size: int64(len(data)), MD5: md5Of(data)}
	if err := os.WriteFile(filepath.Join(root, "asset.bin.part"), data, 0644); err != nil {
		t.Fatal(err)
	}
	valid, required, err := preflightFiles(root, []Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	if valid[entry.Name] {
		t.Fatal("partial file was incorrectly marked as installed")
	}
	if required != 0 {
		t.Fatalf("required bytes = %d, want 0 for a complete verified partial", required)
	}
	var transferred atomic.Int64
	if err := downloadAfterPreflight(context.Background(), root, entry, []string{"https://cdn.invalid"}, &transferred); err != nil {
		t.Fatal(err)
	}
	if !verify(filepath.Join(root, entry.Name), entry) {
		t.Fatal("verified partial was not promoted to the destination")
	}
	if transferred.Load() != 0 {
		t.Fatalf("transferred %d bytes despite a complete verified partial", transferred.Load())
	}
}
