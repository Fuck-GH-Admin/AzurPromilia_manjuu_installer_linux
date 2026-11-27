package main

import (
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
