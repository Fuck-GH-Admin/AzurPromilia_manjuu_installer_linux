package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeStatusResponse() Response {
	var r Response
	r.Code = 0
	r.Data.DownloadMode = 2
	r.Data.Released.Version = json.RawMessage(`"1.0.2"`)
	r.Data.Released.Name = "Release 1.0.2"
	r.Data.Released.LaunchEntry = "AzurPromilia.exe"
	r.Data.Released.Packages = []Package{
		{Type: 2, URL: "https://example.invalid/reality", MD5: "00000000000000000000000000000000", Size: 123},
		{Type: 1, URL: "https://example.invalid/slice", MD5: "11111111111111111111111111111111", Size: 456},
	}
	r.Data.Preload.Version = json.RawMessage(`"1.0.3"`)
	r.Data.Preload.Name = "Preload 1.0.3"
	r.Data.Preload.Packages = []Package{
		{Type: 2, URL: "https://example.invalid/preload", MD5: "22222222222222222222222222222222", Size: 789},
	}
	return r
}

func TestCollectPlayerStatusInstalledUpdateAvailable(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:     "1.0.1",
		VersionName: "Release 1.0.1",
		PackageType: 2,
		Files: []Entry{
			{Name: "AzurPromilia.exe", Size: 1024},
			{Name: "data/a.bin", Size: 2048},
		},
	}
	if err := saveVersion(root, state, m); err != nil {
		t.Fatal(err)
	}
	if err := saveManifest(state, m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AzurPromilia.exe"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "preload-1.0.3.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	s := collectPlayerStatus(root, state, fakeStatusResponse())
	if !s.Installed || s.LocalVersion != "1.0.1" || s.RemoteVersion != "1.0.2" {
		t.Fatalf("unexpected versions: %+v", s)
	}
	if !strings.Contains(s.UpdateState, "1.0.1") || !strings.Contains(s.UpdateState, "1.0.2") {
		t.Fatalf("unexpected update state: %s", s.UpdateState)
	}
	if s.DownloadMode != "REALITY（逐文件）" {
		t.Fatalf("unexpected mode: %s", s.DownloadMode)
	}
	if s.ResourceCount != 2 || s.ResourceBytes != 3072 || s.PackageType != 2 {
		t.Fatalf("unexpected manifest summary: %+v", s)
	}
	if s.LaunchEntryState != "已存在" {
		t.Fatalf("launch entry state: %s", s.LaunchEntryState)
	}
	if s.Preload != "1.0.3（已预下载）" {
		t.Fatalf("preload state: %s", s.Preload)
	}
}

func TestPrintPlayerStatusIsConcise(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := printPlayerStatus(&b, root, state, fakeStatusResponse()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"┌", "┬", "└", "工具版本", "游戏目录", "安装状态", "官方版本", "更新状态", "下载模式", "启动入口", "预下载", "→"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"CDN hosts", "Game config metadata", "Launch metadata"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("player status leaked diagnostic field %q:\n%s", unwanted, out)
		}
	}
}

func TestBuildMainMenuWithoutPathStartsAtPath(t *testing.T) {
	items, selected := buildMainMenu("")
	if selected < 0 || selected >= len(items) || items[selected].Key != "path" {
		t.Fatalf("unexpected initial selection: %d %+v", selected, items)
	}
	var install, status menuItem
	for _, item := range items {
		switch item.Key {
		case "install":
			install = item
		case "status":
			status = item
		}
	}
	if install.Blocked != "" {
		t.Fatalf("install should be able to prompt for a path: %+v", install)
	}
	if status.Blocked == "" {
		t.Fatalf("status should require a configured path: %+v", status)
	}
}

func TestBuildMainMenuInstalledEnablesMaintenance(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveVersion(root, state, Manifest{Version: "1.0.2", VersionName: "Release"}); err != nil {
		t.Fatal(err)
	}
	items, selected := buildMainMenu(root)
	if items[selected].Key != "status" {
		t.Fatalf("installed game should default to status, got %s", items[selected].Key)
	}
	for _, item := range items {
		switch item.Key {
		case "install":
			if item.Blocked == "" {
				t.Fatal("install should be blocked for an existing installation")
			}
		case "update", "verify", "sync":
			if item.Blocked != "" {
				t.Fatalf("%s unexpectedly blocked: %s", item.Key, item.Blocked)
			}
		}
	}
}

func TestRenderMainMenuContainsNavigationHints(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := setPreferredWorkers(8); err != nil {
		t.Fatal(err)
	}
	if err := setPreferredInstallMode("reality"); err != nil {
		t.Fatal(err)
	}
	items, selected := buildMainMenu("")
	out := renderMainMenu(items, selected, "")
	for _, want := range []string{"Azur Promilia Linux Manager", "↑/↓", "Enter", "Esc/q", "游戏目录", "版本 / 资源", "高级选项", "更新工具", "8 workers", "reality"} {
		if !strings.Contains(out, want) {
			t.Fatalf("menu missing %q:\n%s", want, out)
		}
	}
}


func TestMainMenuShowsCachedGameUpdateHint(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".azurpromilia")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveVersion(root, state, Manifest{Version: "1.0.1", VersionName: "Old"}); err != nil {
		t.Fatal(err)
	}
	response := fakeStatusResponse()
	if err := writeJSONAtomic(filepath.Join(state, "packageconfig.json"), response); err != nil {
		t.Fatal(err)
	}
	items, selected := buildMainMenu(root)
	out := renderMainMenu(items, selected, root)
	if !strings.Contains(out, "有更新：1.0.1 → 1.0.2") {
		t.Fatalf("menu did not surface cached update hint:\n%s", out)
	}
}

func TestStatusNextActionPrioritizesUpdate(t *testing.T) {
	s := playerStatus{
		Installed:     true,
		LocalVersion:  "1.0.1",
		RemoteVersion: "1.0.2",
		UpdateState:   "可更新：1.0.1 → 1.0.2",
		Preload:       "1.0.3（可预下载）",
	}
	got := statusNextAction(s)
	if !strings.Contains(got, "azp update") {
		t.Fatalf("next action=%q", got)
	}
}
