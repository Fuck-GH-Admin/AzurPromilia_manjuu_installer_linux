package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	officialInstallerAPI        = "https://api-grp.manjuu.com/activity/web/download/info"
	officialInstallerGameID     = "35"
	officialInstallerAppID      = "350010"
	officialInstallerChannel    = "C000"
	officialInstallerSubChannel = "SC000"
	maxInstallerBytes           = int64(512 << 20)
)

//go:embed installer_source.json
var builtinInstallerSourceJSON []byte

type installerSource struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type installerCacheMetadata struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Version   string `json:"version,omitempty"`
	Source    string `json:"source,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type officialDownloadInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		NativeLauncherDownloadURL string `json:"nativeLauncherDownloadUrl"`
	} `json:"data"`
}

func loadBuiltinInstallerSource() (installerSource, error) {
	var source installerSource
	if err := json.Unmarshal(builtinInstallerSourceJSON, &source); err != nil {
		return source, fmt.Errorf("parse embedded installer fallback metadata: %w", err)
	}
	if source.Channel != officialInstallerSubChannel {
		return source, fmt.Errorf("embedded installer fallback channel is %q, want %q", source.Channel, officialInstallerSubChannel)
	}
	if source.Size <= 0 || len(source.SHA256) != sha256.Size*2 {
		return source, errors.New("embedded installer fallback metadata is incomplete")
	}
	if _, err := hex.DecodeString(source.SHA256); err != nil {
		return source, errors.New("embedded installer fallback SHA-256 is invalid")
	}
	if _, err := validateInstallerURL(source.URL); err != nil {
		return source, fmt.Errorf("embedded installer fallback URL: %w", err)
	}
	return source, nil
}

func officialInstallerSign(timestamp string) string {
	sum := md5.Sum([]byte(timestamp + "gameId=" + officialInstallerGameID))
	return hex.EncodeToString(sum[:])
}

func validateInstallerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return "", fmt.Errorf("installer URL must be HTTPS: %q", raw)
	}
	if u.User != nil {
		return "", errors.New("installer URL must not contain user information")
	}
	return u.String(), nil
}

func officialInstallerHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:    20 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:        90 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many installer download redirects")
			}
			if req.URL.Scheme != "https" {
				return errors.New("installer download redirect refused: HTTPS required")
			}
			return nil
		},
	}
}

func fetchOfficialInstallerURL(ctx context.Context, client *http.Client) (string, error) {
	if client == nil {
		client = officialInstallerHTTPClient()
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, officialInstallerAPI+"?gameId="+officialInstallerGameID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AzurPromilia-Linux-Downloader/preview")
	req.Header.Set("Origin", "https://azurpromilia.manjuu.com")
	req.Header.Set("Referer", "https://azurpromilia.manjuu.com/")
	req.Header.Set("grp-app-id", officialInstallerAppID)
	req.Header.Set("grp-channel-code", officialInstallerChannel)
	req.Header.Set("grp-sub-channel-code", officialInstallerSubChannel)
	req.Header.Set("device-id", "")
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", officialInstallerSign(timestamp))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("official installer discovery request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("official installer discovery returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return parseOfficialInstallerURL(body)
}

func parseOfficialInstallerURL(body []byte) (string, error) {
	var decoded officialDownloadInfoResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode official installer discovery response: %w", err)
	}
	if decoded.Code != 0 {
		return "", fmt.Errorf("official installer discovery API error %d: %s", decoded.Code, decoded.Msg)
	}
	resolved, err := validateInstallerURL(decoded.Data.NativeLauncherDownloadURL)
	if err != nil {
		return "", fmt.Errorf("official installer discovery response: %w", err)
	}
	return resolved, nil
}

func installerCacheDir(state string) (string, error) {
	if base, err := os.UserCacheDir(); err == nil && base != "" {
		return filepath.Join(base, "azurpromilia", "SC000"), nil
	}
	if state == "" {
		return "", errors.New("cannot determine installer cache directory")
	}
	return filepath.Join(state, "installer-cache"), nil
}

func readInstallerCacheMetadata(path string) (installerCacheMetadata, error) {
	var meta installerCacheMetadata
	raw, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err = json.Unmarshal(raw, &meta); err != nil {
		return meta, err
	}
	if _, err = validateInstallerURL(meta.URL); err != nil {
		return meta, err
	}
	if meta.Size <= 0 || len(meta.SHA256) != sha256.Size*2 {
		return meta, errors.New("cached installer metadata is incomplete")
	}
	if _, err = hex.DecodeString(meta.SHA256); err != nil {
		return meta, errors.New("cached installer SHA-256 is invalid")
	}
	return meta, nil
}

func hashInstallerFile(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	if !st.Mode().IsRegular() {
		return 0, "", fmt.Errorf("installer is not a regular file: %s", path)
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

func cachedInstallerValid(path string, meta installerCacheMetadata) bool {
	size, hash, err := hashInstallerFile(path)
	if err != nil || size != meta.Size || !strings.EqualFold(hash, meta.SHA256) {
		return false
	}
	if _, _, err = installerKeys(path); err != nil {
		return false
	}
	return true
}

func validateManualInstaller(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("manual installer: %w", err)
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("manual installer is not a regular file: %s", abs)
	}
	if _, _, err = installerKeys(abs); err != nil {
		return "", fmt.Errorf("manual installer is not a compatible Manjuu/SC000 installer: %w", err)
	}
	return abs, nil
}

func downloadInstallerFile(ctx context.Context, client *http.Client, rawURL, target string, expectedSize int64, expectedSHA string, validate func(string) error) (installerCacheMetadata, error) {
	resolved, err := validateInstallerURL(rawURL)
	if err != nil {
		return installerCacheMetadata{}, err
	}
	if client == nil {
		client = officialInstallerHTTPClient()
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return installerCacheMetadata{}, err
	}
	tmp := target + ".part"
	if err = os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return installerCacheMetadata{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return installerCacheMetadata{}, err
	}
	req.Header.Set("User-Agent", "AzurPromilia-Linux-Downloader/preview")
	resp, err := client.Do(req)
	if err != nil {
		return installerCacheMetadata{}, fmt.Errorf("download official installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return installerCacheMetadata{}, fmt.Errorf("download official installer returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxInstallerBytes {
		return installerCacheMetadata{}, fmt.Errorf("official installer is unexpectedly large: %d bytes", resp.ContentLength)
	}
	if expectedSize > 0 && resp.ContentLength >= 0 && resp.ContentLength != expectedSize {
		return installerCacheMetadata{}, fmt.Errorf("official installer content length %d does not match expected %d", resp.ContentLength, expectedSize)
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return installerCacheMetadata{}, err
	}
	removeTmp := true
	defer func() {
		_ = f.Close()
		if removeTmp {
			_ = os.Remove(tmp)
		}
	}()

	h := sha256.New()
	var total int64
	progressTotal := expectedSize
	if progressTotal <= 0 && resp.ContentLength > 0 {
		progressTotal = resp.ContentLength
	}
	buf := make([]byte, 1024*1024)
	lastProgressAt := time.Now()
	lastProgressBytes := int64(0)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > maxInstallerBytes {
				return installerCacheMetadata{}, fmt.Errorf("official installer exceeds %d bytes", maxInstallerBytes)
			}
			if _, err = f.Write(buf[:n]); err != nil {
				return installerCacheMetadata{}, err
			}
			_, _ = h.Write(buf[:n])
			now := time.Now()
			if now.Sub(lastProgressAt) >= progressInterval() {
				elapsed := now.Sub(lastProgressAt).Seconds()
				speed := 0.0
				if elapsed > 0 {
					speed = float64(total-lastProgressBytes) / elapsed
				}
				renderTransferProgress("启动器", total, progressTotal, 0, 0, 1, speed)
				lastProgressAt = now
				lastProgressBytes = total
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return installerCacheMetadata{}, readErr
		}
	}
	if err = f.Sync(); err != nil {
		return installerCacheMetadata{}, err
	}
	if err = f.Close(); err != nil {
		return installerCacheMetadata{}, err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if expectedSize > 0 && total != expectedSize {
		return installerCacheMetadata{}, fmt.Errorf("official installer size %d does not match expected %d", total, expectedSize)
	}
	if expectedSHA != "" && !strings.EqualFold(hash, expectedSHA) {
		return installerCacheMetadata{}, fmt.Errorf("official installer SHA-256 %s does not match expected %s", hash, expectedSHA)
	}
	if validate != nil {
		if err = validate(tmp); err != nil {
			return installerCacheMetadata{}, fmt.Errorf("downloaded official installer is not compatible with this downloader: %w", err)
		}
	}
	if err = os.Rename(tmp, target); err != nil {
		return installerCacheMetadata{}, err
	}
	removeTmp = false
	finishTransferProgress("启动器", total, progressTotal, 1, 0, 1, total)
	debugf("INSTALLER verified: %d bytes SHA256=%s", total, hash)
	return installerCacheMetadata{
		URL:       resolved,
		SHA256:    hash,
		Size:      total,
		Source:    "official",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func validateInstallerStructure(path string) error {
	_, _, err := installerKeys(path)
	return err
}

func saveInstallerCacheMetadata(path string, meta installerCacheMetadata) error {
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return writeFileAtomic(path, raw, 0600)
}

func resolveInstaller(ctx context.Context, requested, state string) (string, error) {
	if requested != "" {
		path, err := validateManualInstaller(requested)
		if err != nil {
			return "", err
		}
		logf("INSTALLER manual override: %s", path)
		return path, nil
	}

	fallback, err := loadBuiltinInstallerSource()
	if err != nil {
		return "", err
	}
	cacheDir, err := installerCacheDir(state)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(cacheDir, 0700); err != nil {
		return "", err
	}
	cachePath := filepath.Join(cacheDir, "official-installer.exe")
	metaPath := filepath.Join(cacheDir, "official-installer.json")
	cachedMeta, cacheMetaErr := readInstallerCacheMetadata(metaPath)
	cacheValid := cacheMetaErr == nil && cachedInstallerValid(cachePath, cachedMeta)

	client := officialInstallerHTTPClient()
	liveURL, liveErr := fetchOfficialInstallerURL(ctx, client)
	if liveErr == nil {
		logf("INSTALLER official source discovered")
		if cacheValid && cachedMeta.URL == liveURL {
			logf("INSTALLER cache hit: %s", cachePath)
			return cachePath, nil
		}
		var expectedSize int64
		var expectedSHA string
		version := ""
		if liveURL == fallback.URL {
			expectedSize = fallback.Size
			expectedSHA = fallback.SHA256
			version = fallback.Version
		}
		logf("INSTALLER downloading current official SC000 launcher")
		meta, downloadErr := downloadInstallerFile(ctx, client, liveURL, cachePath, expectedSize, expectedSHA, validateInstallerStructure)
		if downloadErr == nil {
			meta.Version = version
			meta.Source = "official-live"
			if err = saveInstallerCacheMetadata(metaPath, meta); err != nil {
				return "", err
			}
			return cachePath, nil
		}
		warnf("current official installer could not be cached: %v", downloadErr)
	} else {
		warnf("official installer discovery unavailable: %v", liveErr)
	}

	if cacheValid {
		logf("INSTALLER using previously verified cache: %s", cachePath)
		return cachePath, nil
	}

	logf("INSTALLER using built-in verified fallback %s", fallback.Version)
	meta, fallbackErr := downloadInstallerFile(ctx, client, fallback.URL, cachePath, fallback.Size, fallback.SHA256, validateInstallerStructure)
	if fallbackErr == nil {
		meta.Version = fallback.Version
		meta.Source = "builtin-fallback"
		if err = saveInstallerCacheMetadata(metaPath, meta); err != nil {
			return "", err
		}
		return cachePath, nil
	}
	return "", fmt.Errorf("automatic SC000 installer acquisition failed (live: %v; fallback: %v); check the network or pass --installer /path/to/SC000.exe", liveErr, fallbackErr)
}
