package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type playerStatus struct {
	Root              string
	Installed         bool
	LocalVersion      string
	LocalVersionName  string
	RemoteVersion     string
	RemoteVersionName string
	UpdateState       string
	DownloadMode      string
	ResourceCount     int
	ResourceBytes     uint64
	PackageType       int
	LaunchEntry       string
	LaunchEntryState  string
	Preload           string
	FreeBytes         uint64
	HasFreeBytes      bool
}

func downloadModeLabel(mode int) string {
	switch mode {
	case 2:
		return "REALITY（逐文件）"
	case 0, 1:
		return "SLICE（分片压缩包）"
	default:
		return fmt.Sprintf("未知（%d）", mode)
	}
}

func responseLaunchEntry(response Response) string {
	if entry := strings.TrimSpace(response.Data.Released.LaunchEntry); entry != "" {
		return entry
	}
	var launchInfo map[string]any
	if json.Unmarshal(response.Data.LaunchInfo, &launchInfo) == nil {
		if entry, _ := launchInfo["launchEntry"].(string); strings.TrimSpace(entry) != "" {
			return strings.TrimSpace(entry)
		}
	}
	var gameConfig map[string]any
	if json.Unmarshal(response.Data.GameConfig, &gameConfig) == nil {
		if entry, _ := gameConfig["launchEntry"].(string); strings.TrimSpace(entry) != "" {
			return strings.TrimSpace(entry)
		}
	}
	return ""
}

func loadCurrentManifest(state string) (Manifest, bool) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(state, "manifest.json"))
	if err != nil || json.Unmarshal(b, &m) != nil {
		return m, false
	}
	if m.Version == "" || (len(m.Files) == 0 && len(m.Extracted) == 0) {
		return m, false
	}
	return m, true
}

func manifestSummary(m Manifest) (int, uint64) {
	var total uint64
	if len(m.Files) > 0 {
		for _, f := range m.Files {
			if f.Size > 0 {
				total += uint64(f.Size)
			}
		}
		return len(m.Files), total
	}
	for _, f := range m.Extracted {
		if f.Size > 0 {
			total += uint64(f.Size)
		}
	}
	return len(m.Extracted), total
}

func collectPlayerStatus(root, state string, response Response) playerStatus {
	s := playerStatus{
		Root:              root,
		RemoteVersion:     releaseVersion(response.Data.Released),
		RemoteVersionName: strings.TrimSpace(response.Data.Released.Name),
		DownloadMode:      downloadModeLabel(response.Data.DownloadMode),
		LaunchEntry:       responseLaunchEntry(response),
	}

	if local, err := loadVersion(root, state); err == nil {
		s.Installed = true
		s.LocalVersion = local.Version
		s.LocalVersionName = local.VersionName
	}

	switch {
	case !s.Installed:
		s.UpdateState = "尚未安装"
	case s.RemoteVersion == "":
		s.UpdateState = "无法判断"
	case s.LocalVersion == s.RemoteVersion:
		s.UpdateState = "已是最新"
	default:
		s.UpdateState = fmt.Sprintf("可更新：%s → %s", s.LocalVersion, s.RemoteVersion)
	}

	if m, ok := loadCurrentManifest(state); ok {
		s.ResourceCount, s.ResourceBytes = manifestSummary(m)
		s.PackageType = m.PackageType
	}

	if s.LaunchEntry == "" {
		s.LaunchEntryState = "官方元数据未提供"
	} else if !s.Installed {
		s.LaunchEntryState = "尚未安装"
	} else if p, err := safe(root, s.LaunchEntry); err == nil {
		if st, statErr := os.Stat(p); statErr == nil && st.Mode().IsRegular() {
			s.LaunchEntryState = "已存在"
		} else {
			s.LaunchEntryState = "缺失"
		}
	} else {
		s.LaunchEntryState = "路径无效"
	}

	preloadVersion := releaseVersion(response.Data.Preload)
	if preloadVersion == "" {
		s.Preload = "无"
	} else {
		s.Preload = preloadVersion
		if st, err := os.Stat(filepath.Join(state, "preload-"+preloadVersion+".json")); err == nil && st.Mode().IsRegular() {
			s.Preload += "（已预下载）"
		} else {
			s.Preload += "（可预下载）"
		}
	}

	if free, err := availableBytes(root); err == nil {
		s.FreeBytes = free
		s.HasFreeBytes = true
	}
	return s
}

func versionDisplay(version, name string) string {
	version = strings.TrimSpace(version)
	name = strings.TrimSpace(name)
	switch {
	case version == "":
		return "未知"
	case name == "" || name == version:
		return version
	default:
		return fmt.Sprintf("%s (%s)", version, name)
	}
}

func printPlayerStatus(w io.Writer, root, state string, response Response) error {
	s := collectPlayerStatus(root, state, response)

	fmt.Fprintln(w, "Azur Promilia Linux Manager")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  游戏目录:   %s\n", s.Root)
	if s.Installed {
		fmt.Fprintln(w, "  安装状态:   已安装")
		fmt.Fprintf(w, "  本地版本:   %s\n", versionDisplay(s.LocalVersion, s.LocalVersionName))
	} else {
		fmt.Fprintln(w, "  安装状态:   未安装")
		fmt.Fprintln(w, "  本地版本:   —")
	}
	fmt.Fprintf(w, "  官方版本:   %s\n", versionDisplay(s.RemoteVersion, s.RemoteVersionName))
	fmt.Fprintf(w, "  更新状态:   %s\n", s.UpdateState)
	fmt.Fprintf(w, "  下载模式:   %s\n", s.DownloadMode)

	if s.ResourceCount > 0 {
		pkg := ""
		if s.PackageType > 0 {
			pkg = fmt.Sprintf(" · packageType %d", s.PackageType)
		}
		fmt.Fprintf(w, "  本地清单:   %d 个文件 · %s%s\n", s.ResourceCount, formatBytes(s.ResourceBytes), pkg)
	} else {
		fmt.Fprintln(w, "  本地清单:   无")
	}

	if s.LaunchEntry != "" {
		fmt.Fprintf(w, "  启动入口:   %s · %s\n", s.LaunchEntry, s.LaunchEntryState)
	} else {
		fmt.Fprintf(w, "  启动入口:   %s\n", s.LaunchEntryState)
	}
	fmt.Fprintf(w, "  预下载:     %s\n", s.Preload)
	if s.HasFreeBytes {
		fmt.Fprintf(w, "  磁盘可用:   %s\n", formatBytes(s.FreeBytes))
	}
	return nil
}
