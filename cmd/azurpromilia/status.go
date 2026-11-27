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

func displayCellWidth(s string) int {
	width := 0
	for _, r := range s {
		switch {
		case r == '\t':
			width += 4
		case r >= 0x1100 && (
			r <= 0x115f ||
				r == 0x2329 || r == 0x232a ||
				(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
				(r >= 0xac00 && r <= 0xd7a3) ||
				(r >= 0xf900 && r <= 0xfaff) ||
				(r >= 0xfe10 && r <= 0xfe19) ||
				(r >= 0xfe30 && r <= 0xfe6f) ||
				(r >= 0xff00 && r <= 0xff60) ||
				(r >= 0xffe0 && r <= 0xffe6) ||
				(r >= 0x1f300 && r <= 0x1faff)):
			width += 2
		default:
			width++
		}
	}
	return width
}

func padStatusCell(s string, width int) string {
	if n := displayCellWidth(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func printStatusTable(w io.Writer, rows [][2]string) {
	labelWidth := 8
	valueWidth := 12
	for _, row := range rows {
		if n := displayCellWidth(row[0]); n > labelWidth {
			labelWidth = n
		}
		if n := displayCellWidth(row[1]); n > valueWidth {
			valueWidth = n
		}
	}
	line := func(left, middle, right string) {
		fmt.Fprintf(w, "%s%s%s%s%s\n",
			left,
			strings.Repeat("─", labelWidth+2),
			middle,
			strings.Repeat("─", valueWidth+2),
			right,
		)
	}
	line("┌", "┬", "┐")
	for i, row := range rows {
		fmt.Fprintf(w, "│ %s │ %s │\n",
			padStatusCell(row[0], labelWidth),
			padStatusCell(row[1], valueWidth),
		)
		if i != len(rows)-1 {
			line("├", "┼", "┤")
		}
	}
	line("└", "┴", "┘")
}

func statusNextAction(s playerStatus) string {
	switch {
	case !s.Installed:
		return "下一步：azp install"
	case strings.HasPrefix(s.UpdateState, "可更新"):
		return "有新版本可用：azp update"
	case s.LaunchEntryState == "缺失":
		return "启动入口缺失：azp sync"
	case strings.Contains(s.Preload, "可预下载"):
		return "存在预下载：azp predownload"
	default:
		return "当前版本可用；需要完整检查时运行 azp verify"
	}
}

func printPlayerStatus(w io.Writer, root, state string, response Response) error {
	s := collectPlayerStatus(root, state, response)
	rows := [][2]string{
		{"工具版本", toolVersion},
		{"游戏目录", s.Root},
	}
	if s.Installed {
		rows = append(rows,
			[2]string{"安装状态", "已安装"},
			[2]string{"本地版本", versionDisplay(s.LocalVersion, s.LocalVersionName)},
		)
	} else {
		rows = append(rows,
			[2]string{"安装状态", "未安装"},
			[2]string{"本地版本", "—"},
		)
	}
	rows = append(rows,
		[2]string{"官方版本", versionDisplay(s.RemoteVersion, s.RemoteVersionName)},
		[2]string{"更新状态", s.UpdateState},
		[2]string{"下载模式", s.DownloadMode},
	)

	if s.ResourceCount > 0 {
		pkg := ""
		if s.PackageType > 0 {
			pkg = fmt.Sprintf(" · %s / packageType %d", packageModeName(s.PackageType), s.PackageType)
		}
		rows = append(rows, [2]string{
			"本地清单",
			fmt.Sprintf("%d 个文件 · %s%s", s.ResourceCount, formatBytes(s.ResourceBytes), pkg),
		})
	} else {
		rows = append(rows, [2]string{"本地清单", "无"})
	}

	if s.LaunchEntry != "" {
		rows = append(rows, [2]string{"启动入口", s.LaunchEntry + " · " + s.LaunchEntryState})
	} else {
		rows = append(rows, [2]string{"启动入口", s.LaunchEntryState})
	}
	rows = append(rows, [2]string{"预下载", s.Preload})
	if s.HasFreeBytes {
		rows = append(rows, [2]string{"磁盘可用", formatBytes(s.FreeBytes)})
	}

	fmt.Fprintln(w, "Azur Promilia Linux Manager")
	fmt.Fprintln(w)
	printStatusTable(w, rows)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "→ %s\n", statusNextAction(s))
	printUpdateAvailableNotice(w, root, state, response)
	return nil
}
