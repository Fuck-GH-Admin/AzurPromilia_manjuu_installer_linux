package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAvailablePackageTypesAndLabels(t *testing.T) {
	r := fakeStatusResponse().Data.Released
	got := availablePackageTypes(r)
	if len(got) != 2 || got[0] != 2 || got[1] != 1 {
		t.Fatalf("availablePackageTypes=%v", got)
	}
	if label := availablePackageLabel(r); label != "REALITY · SLICE" {
		t.Fatalf("availablePackageLabel=%q", label)
	}

	r.Packages = r.Packages[:1]
	if label := availablePackageLabel(r); label != "REALITY" {
		t.Fatalf("single package label=%q", label)
	}
}

func TestPrintResourceOverview(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveVersion(root, state, Manifest{Version: "1.0.1", VersionName: "Old"}); err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	printResourceOverview(&b, root, state, fakeStatusResponse())
	out := b.String()
	for _, want := range []string{
		"官方版本 / 资源",
		"正式版",
		"1.0.2",
		"REALITY",
		"SLICE",
		"可更新 1.0.1 → 1.0.2",
		"预下载",
		"1.0.3",
		"官方推荐下载模式",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("resource overview missing %q:\n%s", want, out)
		}
	}
}

func TestResourceReleaseItemsByInstallState(t *testing.T) {
	response := fakeStatusResponse()

	items := resourceReleaseItems(response, false, "")
	var keys []string
	for _, item := range items {
		keys = append(keys, item.Key)
	}
	joined := strings.Join(keys, " ")
	for _, want := range []string{"install:auto", "install:reality", "install:slice"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fresh install menu missing %s: %v", want, keys)
		}
	}

	items = resourceReleaseItems(response, true, "1.0.1")
	if len(items) < 1 || items[0].Key != "update" {
		t.Fatalf("outdated install should offer update: %+v", items)
	}

	items = resourceReleaseItems(response, true, "1.0.2")
	joined = ""
	for _, item := range items {
		joined += " " + item.Key
	}
	for _, want := range []string{"verify", "repair"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("current install menu missing %s: %+v", want, items)
		}
	}
}

func TestPreloadStateRequiresCurrentReleasedVersion(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	response := fakeStatusResponse()

	if got := preloadStateLabel(root, state, response); got != "安装正式版后可预下载" {
		t.Fatalf("no-install preload state=%q", got)
	}
	if err := saveVersion(root, state, Manifest{Version: "1.0.1"}); err != nil {
		t.Fatal(err)
	}
	if got := preloadStateLabel(root, state, response); got != "请先更新到当前正式版" {
		t.Fatalf("outdated preload state=%q", got)
	}
	if err := saveVersion(root, state, Manifest{Version: "1.0.2"}); err != nil {
		t.Fatal(err)
	}
	if got := preloadStateLabel(root, state, response); got != "可预下载" {
		t.Fatalf("current preload state=%q", got)
	}
	if err := os.WriteFile(filepath.Join(state, "preload-1.0.3.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := preloadStateLabel(root, state, response); got != "已预下载" {
		t.Fatalf("prepared preload state=%q", got)
	}
}

func TestCachedGameUpdateHint(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveVersion(root, state, Manifest{Version: "1.0.1"}); err != nil {
		t.Fatal(err)
	}
	response := fakeStatusResponse()
	b, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "packageconfig.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if got := cachedGameUpdateHint(root); got != "有更新：1.0.1 → 1.0.2" {
		t.Fatalf("cached update hint=%q", got)
	}
}
