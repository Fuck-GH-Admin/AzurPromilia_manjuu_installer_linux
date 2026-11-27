package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgressBar(t *testing.T) {
	if got := progressBar(0, 100, 10); got != strings.Repeat("░", 10) {
		t.Fatalf("zero progress=%q", got)
	}
	if got := progressBar(100, 100, 10); got != strings.Repeat("█", 10) {
		t.Fatalf("complete progress=%q", got)
	}
	got := progressBar(50, 100, 10)
	if got != strings.Repeat("█", 5)+strings.Repeat("░", 5) {
		t.Fatalf("half progress=%q", got)
	}
}

func TestFileTransferSummaryShowsOutcome(t *testing.T) {
	t.Setenv("CI", "1")
	if err := configureOutput(false, false, false); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	entry := Entry{Name: "resource", MD5: "d41d8cd98f00b204e9800998ecf8427e"}
	if err := os.WriteFile(filepath.Join(root, entry.Name), nil, 0600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		entry  Entry
		cancel bool
		want   string
	}{
		{name: "completed", entry: entry, want: "校验 complete: files 1/1 failed=0"},
		{name: "interrupted", entry: entry, cancel: true, want: "下载 interrupted: files 0/1 failed=0"},
		{name: "failed", entry: Entry{Name: "missing"}, want: "校验 failed: files 0/1 failed=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			original := os.Stderr
			os.Stderr = w
			err = runFiles(ctx, root, Manifest{Files: []Entry{tc.entry}}, 1, !tc.cancel)
			os.Stderr = original
			w.Close()
			if (err == nil) != (tc.name == "completed") {
				t.Fatalf("unexpected result: %v", err)
			}
			output, err := io.ReadAll(r)
			if err != nil || !strings.Contains(string(output), tc.want) {
				t.Fatalf("wanted %q, got %q (%v)", tc.want, output, err)
			}
		})
	}
}

func TestConfigureOutputRejectsVerboseQuiet(t *testing.T) {
	if err := configureOutput(true, true, false); err == nil {
		t.Fatal("verbose + quiet should fail")
	}
	if err := configureOutput(true, false, true); err != nil {
		t.Fatal(err)
	}
	if !isVerboseOutput() {
		t.Fatal("verbose output was not enabled")
	}
	if outputColor.Load() {
		t.Fatal("no-color should disable color")
	}
	if err := configureOutput(false, false, false); err != nil {
		t.Fatal(err)
	}
}
