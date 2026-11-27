package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionScriptsExposePlayerCommands(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("%s completion: %v", shell, err)
		}
		for _, want := range []string{"azp", "install", "status", "upgrade", "completion"} {
			if !strings.Contains(script, want) {
				t.Fatalf("%s completion missing %q", shell, want)
			}
		}
	}
}

func TestCompletionInstallUsesUserDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, shell := range []string{"bash", "zsh", "fish"} {
		paths, err := installCompletion(shell)
		if err != nil {
			t.Fatalf("install %s completion: %v", shell, err)
		}
		if len(paths) == 0 {
			t.Fatalf("%s completion installed no paths", shell)
		}
		for _, path := range paths {
			if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(home)+string(os.PathSeparator)) {
				t.Fatalf("%s completion escaped HOME: %s", shell, path)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), "azp") {
				t.Fatalf("%s completion file looks wrong: %s", shell, path)
			}
		}
	}
}

func TestDetectShell(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")
	got, err := detectShell()
	if err != nil || got != "fish" {
		t.Fatalf("detectShell=%q err=%v", got, err)
	}
	t.Setenv("SHELL", "/bin/nushell")
	if _, err := detectShell(); err == nil {
		t.Fatal("unsupported shell should fail")
	}
}
