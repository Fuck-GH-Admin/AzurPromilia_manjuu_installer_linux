package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfoUsesOptionalGameConfigAndLaunchEntry(t *testing.T) {
	var response Response
	response.Data.GameConfig = json.RawMessage(`{"gameFolderName":"Game_Data","launchEntry":"config.exe"}`)
	response.Data.LaunchInfo = json.RawMessage(`{"launchEntry":"launcher.exe"}`)
	var output bytes.Buffer
	if err := printPackageInfo(&output, response); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Game folder: Game_Data\nGame entry: launcher.exe\n") {
		t.Fatalf("unexpected info output:\n%s", &output)
	}
}

func TestPerFileIndexRejectsMismatchedCounts(t *testing.T) {
	file := "0,3,900150983cd24fb0d6963f7d28e17f72,data/game.exe,/game.exe,\n"
	if _, err := parseIndex([]byte("2,2,0,\n"+file), false); err == nil {
		t.Fatal("accepted a count header that does not match parsed records")
	}
}

func TestZipExtractedEntriesRejectsReservedAndDuplicatePaths(t *testing.T) {
	for name, paths := range map[string][]string{
		"reserved": []string{"_index/version"},
		"duplicate": []string{"bin/game.exe", "bin/game.exe"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "game.zip")
			f, err := os.Create(archive)
			if err != nil { t.Fatal(err) }
			zw := zip.NewWriter(f)
			for _, p := range paths {
				w, err := zw.Create(p)
				if err != nil { t.Fatal(err) }
				if _, err = w.Write([]byte("payload")); err != nil { t.Fatal(err) }
			}
			if err = zw.Close(); err != nil { t.Fatal(err) }
			if err = f.Close(); err != nil { t.Fatal(err) }
			if _, err = zipExtractedEntries(archive); err == nil {
				t.Fatal("unsafe ZIP passed entry validation")
			}
		})
	}
}
