package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinInstallerSourceIsComplete(t *testing.T) {
	source, err := loadBuiltinInstallerSource()
	if err != nil {
		t.Fatal(err)
	}
	if source.Channel != "SC000" || source.Version != "1.0.2.2" {
		t.Fatalf("unexpected built-in source: %+v", source)
	}
	if source.Size != 190835304 {
		t.Fatalf("built-in installer size = %d", source.Size)
	}
	if source.SHA256 != "0a1d1b97947838146c7b8f3f0d27dbba79e60372d043f60cd5401f2979531282" {
		t.Fatalf("unexpected built-in installer SHA-256: %s", source.SHA256)
	}
}

func TestOfficialInstallerSignMatchesWebsiteAlgorithm(t *testing.T) {
	const timestamp = "1764151554000"
	sum := md5.Sum([]byte(timestamp + "gameId=35"))
	want := hex.EncodeToString(sum[:])
	if got := officialInstallerSign(timestamp); got != want {
		t.Fatalf("sign = %s, want %s", got, want)
	}
}

func TestParseOfficialInstallerURL(t *testing.T) {
	const want = "https://syncstation.manjuu.com/path/installer.exe"
	body := []byte(`{"code":0,"msg":"success","data":{"nativeLauncherDownloadUrl":"` + want + `"}}`)
	got, err := parseOfficialInstallerURL(body)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestParseOfficialInstallerURLRejectsUnsafeAndErrorResponses(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"code":1,"msg":"failed","data":{}}`),
		[]byte(`{"code":0,"msg":"success","data":{"nativeLauncherDownloadUrl":"http://example.com/installer.exe"}}`),
		[]byte(`{"code":0,"msg":"success","data":{"nativeLauncherDownloadUrl":""}}`),
	} {
		if _, err := parseOfficialInstallerURL(body); err == nil {
			t.Fatalf("accepted invalid response: %s", body)
		}
	}
}

func TestDownloadInstallerFileChecksSizeAndSHA256(t *testing.T) {
	payload := []byte("test installer payload")
	hash := sha256.Sum256(payload)
	wantHash := hex.EncodeToString(hash[:])
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "installer.exe")
	meta, err := downloadInstallerFile(
		context.Background(),
		server.Client(),
		server.URL,
		target,
		int64(len(payload)),
		wantHash,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Size != int64(len(payload)) || meta.SHA256 != wantHash {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatal("downloaded installer payload mismatch")
	}
}

func TestDownloadInstallerFileDoesNotPublishBadHash(t *testing.T) {
	payload := []byte("bad hash payload")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	root := t.TempDir()
	target := filepath.Join(root, "installer.exe")
	_, err := downloadInstallerFile(
		context.Background(),
		server.Client(),
		server.URL,
		target,
		int64(len(payload)),
		strings.Repeat("0", sha256.Size*2),
		nil,
	)
	if err == nil {
		t.Fatal("bad SHA-256 was accepted")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("invalid installer was published: %v", statErr)
	}
	if _, statErr := os.Stat(target + ".part"); !os.IsNotExist(statErr) {
		t.Fatalf("partial installer was not removed: %v", statErr)
	}
}

func TestValidateInstallerURLRequiresHTTPS(t *testing.T) {
	if _, err := validateInstallerURL("https://syncstation.manjuu.com/a.exe"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"http://syncstation.manjuu.com/a.exe",
		"file:///tmp/a.exe",
		"https://user:pass@example.com/a.exe",
		"",
	} {
		if _, err := validateInstallerURL(raw); err == nil {
			t.Fatalf("accepted unsafe installer URL %q", raw)
		}
	}
}
