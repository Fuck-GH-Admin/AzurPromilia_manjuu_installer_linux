package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCommonCLIOptions(t *testing.T) {
	got, err := parseCommonCLIOptions([]string{"-p", "/games/azur", "install", "-j", "8", "--mode", "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/games/azur" || got.Workers != "8" {
		t.Fatalf("unexpected common options: %+v", got)
	}
	if strings.Join(got.Rest, " ") != "install --mode auto" {
		t.Fatalf("unexpected remaining args: %#v", got.Rest)
	}
}


func TestGlobalOutputOptionsAndVersion(t *testing.T) {
	got, err := parseCommonCLIOptions([]string{"--verbose", "--no-color", "--version"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Verbose || !got.NoColor || !got.Version {
		t.Fatalf("unexpected global options: %+v", got)
	}
}

func TestVerifyVersionRemainsCommandOption(t *testing.T) {
	got, err := parseCommonCLIOptions([]string{"verify", "--target", "preload", "--version", "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version {
		t.Fatalf("verify --version was stolen by global parser: %+v", got)
	}
	if strings.Join(got.Rest, " ") != "verify --target preload --version 1.2.3" {
		t.Fatalf("unexpected verify args: %#v", got.Rest)
	}
}

func TestGlobalVersionStillWorksAfterOtherCommands(t *testing.T) {
	got, err := parseCommonCLIOptions([]string{"status", "--version"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Version {
		t.Fatalf("status --version should still resolve to tool version: %+v", got)
	}
}

func TestCLIConfigRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := filepath.Join(t.TempDir(), "AzurPromilia")
	got, err := rememberGamePath(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("remembered path=%q want=%q", got, filepath.Clean(want))
	}
	loaded, err := configuredGamePath()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != got {
		t.Fatalf("loaded path=%q want=%q", loaded, got)
	}
	configFile, err := cliConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0077 != 0 {
		t.Fatalf("config permissions too broad: %o", st.Mode().Perm())
	}
	if err := setPreferredWorkers(8); err != nil {
		t.Fatal(err)
	}
	if err := setPreferredInstallMode("reality"); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GamePath != got || preferredWorkers(cfg) != 8 || preferredInstallMode(cfg) != "reality" {
		t.Fatalf("saved defaults were not preserved: %+v", cfg)
	}
}

func TestHelpIsPlayerFacing(t *testing.T) {
	var b bytes.Buffer
	printTopLevelHelp(&b, "azp")
	s := b.String()
	for _, want := range []string{"azp -p", "install", "status", "resources", "versions", "sync", "upgrade", "completion", "--version", "--verbose", "--help"} {
		if !strings.Contains(s, want) {
			t.Fatalf("help missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "packageconfig required") {
		t.Fatalf("top-level help leaked legacy implementation detail:\n%s", s)
	}
}

func TestCommandAliases(t *testing.T) {
	cases := map[string]string{
		"status":      "status",
		"sync":        "repair",
		"predownload": "preload",
		"versions":    "resources",
		"verify":      "verify",
	}
	for in, want := range cases {
		if got := canonicalCommand(in); got != want {
			t.Fatalf("canonicalCommand(%q)=%q want=%q", in, got, want)
		}
	}
}
