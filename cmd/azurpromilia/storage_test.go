package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckAvailableSpace(t *testing.T) {
	if err := checkAvailableSpace(5, 5, "test"); err != nil {
		t.Fatalf("exactly sufficient space rejected: %v", err)
	}
	if err := checkAvailableSpace(6, 5, "test"); err == nil {
		t.Fatal("insufficient space was accepted")
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
