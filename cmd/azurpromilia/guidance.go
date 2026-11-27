package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func printWorkflowNextSteps(w io.Writer, command, root, state string, response Response) {
	fmt.Fprintln(w)
	switch command {
	case "install":
		version := releaseVersion(response.Data.Released)
		fmt.Fprintf(w, "✓ 安装完成：%s\n", versionDisplay(version, response.Data.Released.Name))
		fmt.Fprintf(w, "  下一步：%s verify\n", cliProgramName())
		fmt.Fprintln(w, "  校验通过后，按 docs/PROTON_SETUP.md 配置 Steam / Proton 并启动游戏。")
	case "update":
		version := releaseVersion(response.Data.Released)
		fmt.Fprintf(w, "✓ 游戏已更新到：%s\n", versionDisplay(version, response.Data.Released.Name))
		fmt.Fprintf(w, "  建议：%s verify\n", cliProgramName())
		fmt.Fprintln(w, "  校验通过后即可继续使用原来的 Steam / Proton 启动配置。")
	case "repair":
		fmt.Fprintln(w, "✓ 修复完成。")
		fmt.Fprintf(w, "  建议：%s verify\n", cliProgramName())
		if entry := responseLaunchEntry(response); entry != "" {
			if p, err := safe(root, entry); err == nil {
				if st, statErr := os.Stat(p); statErr == nil && st.Mode().IsRegular() {
					fmt.Fprintln(w, "  启动入口已存在；校验通过后可以重新启动游戏。")
				}
			}
		}
	case "preload":
		version := releaseVersion(response.Data.Preload)
		fmt.Fprintf(w, "✓ 预下载已准备：%s\n", versionDisplay(version, response.Data.Preload.Name))
		fmt.Fprintln(w, "  暂时不要手工覆盖游戏目录。正式版发布后运行：")
		fmt.Fprintf(w, "  %s update\n", cliProgramName())
		fmt.Fprintln(w, "  update 会优先复用已经校验过的预下载文件。")
	}
}

func printVerifyNextSteps(w io.Writer, target, version string) {
	fmt.Fprintln(w)
	if target == "preload" {
		fmt.Fprintf(w, "✓ 预下载资源校验通过：%s\n", version)
		fmt.Fprintln(w, "  正式发布后运行 azp update；不要手工把 _pre_download 覆盖到游戏目录。")
		return
	}
	fmt.Fprintf(w, "✓ 游戏文件校验通过：%s\n", version)
	fmt.Fprintln(w, "  文件完整性没有发现问题，可以继续启动游戏。")
}

func printUpdateAvailableNotice(w io.Writer, root, state string, response Response) {
	local, err := loadVersion(root, state)
	if err != nil {
		return
	}
	remote := releaseVersion(response.Data.Released)
	if remote == "" || local.Version == remote {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "⚠ 有可用游戏更新：%s → %s\n", local.Version, remote)
	fmt.Fprintf(w, "  运行：%s update\n", cliProgramName())
}

func preloadManifestExists(state, version string) bool {
	if version == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(state, "preload-"+version+".json"))
	return err == nil && st.Mode().IsRegular()
}
