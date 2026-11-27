package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func packageModeName(typ int) string {
	switch typ {
	case 1:
		return "SLICE"
	case 2:
		return "REALITY"
	default:
		return fmt.Sprintf("TYPE-%d", typ)
	}
}

func availablePackageTypes(r Release) []int {
	var out []int
	for _, typ := range []int{2, 1} {
		if _, err := findPackage(r, typ); err == nil {
			out = append(out, typ)
		}
	}
	return out
}

func availablePackageLabel(r Release) string {
	types := availablePackageTypes(r)
	if len(types) == 0 {
		return "无可用资源"
	}
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, packageModeName(typ))
	}
	return strings.Join(names, " · ")
}

func releaseStateLabel(root, state string, response Response) string {
	local, err := loadVersion(root, state)
	remote := releaseVersion(response.Data.Released)
	switch {
	case err != nil:
		return "可安装"
	case remote == "":
		return "官方版本未知"
	case local.Version == remote:
		return "当前已安装"
	default:
		return fmt.Sprintf("可更新 %s → %s", local.Version, remote)
	}
}

func preloadStateLabel(root, state string, response Response) string {
	version := releaseVersion(response.Data.Preload)
	if version == "" {
		return "当前无预下载"
	}
	local, err := loadVersion(root, state)
	if err != nil {
		return "安装正式版后可预下载"
	}
	current := releaseVersion(response.Data.Released)
	if current != "" && local.Version != current {
		return "请先更新到当前正式版"
	}
	if st, err := os.Stat(filepath.Join(state, "preload-"+version+".json")); err == nil && st.Mode().IsRegular() {
		return "已预下载"
	}
	return "可预下载"
}

func printResourceOverview(w io.Writer, root, state string, response Response) {
	fmt.Fprintln(w, "Azur Promilia 官方版本 / 资源")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "┌──────────┬──────────────────────┬────────────────────────────")
	fmt.Fprintln(w, "│ 类型     │ 版本                 │ 可用资源 / 状态")
	fmt.Fprintln(w, "├──────────┼──────────────────────┼────────────────────────────")
	fmt.Fprintf(w, "│ 正式版   │ %-20s │ %s · %s\n",
		versionDisplay(releaseVersion(response.Data.Released), response.Data.Released.Name),
		availablePackageLabel(response.Data.Released),
		releaseStateLabel(root, state, response),
	)
	if version := releaseVersion(response.Data.Preload); version != "" {
		fmt.Fprintf(w, "│ 预下载   │ %-20s │ %s · %s\n",
			versionDisplay(version, response.Data.Preload.Name),
			availablePackageLabel(response.Data.Preload),
			preloadStateLabel(root, state, response),
		)
	} else {
		fmt.Fprintln(w, "│ 预下载   │ —                    │ 当前无官方预下载")
	}
	fmt.Fprintln(w, "└──────────┴──────────────────────┴────────────────────────────")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "官方推荐下载模式: %s\n", downloadModeLabel(response.Data.DownloadMode))
	fmt.Fprintln(w, "说明：官方接口只提供当前正式版与当前预下载版，不提供任意历史版本列表。")
}

func cachedOfficialResponse(root string) (Response, bool) {
	var response Response
	if strings.TrimSpace(root) == "" {
		return response, false
	}
	path := filepath.Join(root, ".azurpromilia", "packageconfig.json")
	r, err := readPackageResponse(path)
	if err != nil {
		return response, false
	}
	return r, true
}

func cachedGameUpdateHint(root string) string {
	response, ok := cachedOfficialResponse(root)
	if !ok {
		return ""
	}
	state := filepath.Join(root, ".azurpromilia")
	local, err := loadVersion(root, state)
	if err != nil {
		return ""
	}
	remote := releaseVersion(response.Data.Released)
	if remote == "" || remote == local.Version {
		return ""
	}
	return fmt.Sprintf("有更新：%s → %s", local.Version, remote)
}

func resourceReleaseItems(response Response, installed bool, localVersion string) []menuItem {
	released := response.Data.Released
	remote := releaseVersion(released)
	if !installed {
		items := []menuItem{}
		if _, err := selectPackageType(response.Data.DownloadMode, "auto", released.Packages); err == nil {
			items = append(items, menuItem{
				Key: "install:auto", Label: "按官方推荐安装",
				Description: downloadModeLabel(response.Data.DownloadMode),
			})
		}
		if _, err := findPackage(released, 2); err == nil {
			items = append(items, menuItem{
				Key: "install:reality", Label: "安装 REALITY",
				Description: "逐文件模式，当前实测路径",
			})
		}
		if _, err := findPackage(released, 1); err == nil {
			items = append(items, menuItem{
				Key: "install:slice", Label: "安装 SLICE",
				Description: "分片 ZIP 模式（Preview）",
			})
		}
		items = append(items, menuItem{Key: "back", Label: "返回", Description: "返回版本列表"})
		return items
	}

	if remote != "" && localVersion != remote {
		items := []menuItem{}
		if _, err := findPackage(released, 2); err == nil {
			items = append(items, menuItem{
				Key: "update", Label: "更新到 " + remote,
				Description: "使用 REALITY 逐文件更新链路",
			})
		}
		items = append(items, menuItem{Key: "back", Label: "返回", Description: "返回版本列表"})
		return items
	}

	return []menuItem{
		{Key: "verify", Label: "完整校验", Description: "只检查，不下载"},
		{Key: "repair", Label: "校验并修复", Description: "重新下载缺失或损坏资源"},
		{Key: "back", Label: "返回", Description: "返回版本列表"},
	}
}

func runReleasedResourceMenu(ctx context.Context, root, state, installer string, response Response, workers int) error {
	local, localErr := loadVersion(root, state)
	installed := localErr == nil
	items := resourceReleaseItems(response, installed, local.Version)
	choice, err := chooseSimpleMenu(
		"正式版 "+versionDisplay(releaseVersion(response.Data.Released), response.Data.Released.Name),
		"请选择针对当前正式版的操作。",
		items,
		0,
	)
	if err != nil || choice == "" || choice == "back" {
		return err
	}

	switch {
	case strings.HasPrefix(choice, "install:"):
		mode := strings.TrimPrefix(choice, "install:")
		if err := installRelease(ctx, root, state, installer, response, mode, workers); err != nil {
			return err
		}
		printWorkflowNextSteps(os.Stdout, "install", root, state, response)
		return nil
	case choice == "update":
		if err := updateRelease(ctx, root, state, installer, response, workers); err != nil {
			return err
		}
		printWorkflowNextSteps(os.Stdout, "update", root, state, response)
		return nil
	case choice == "repair":
		if err := repairRelease(ctx, root, state, installer, response, workers); err != nil {
			return err
		}
		printWorkflowNextSteps(os.Stdout, "repair", root, state, response)
		return nil
	case choice == "verify":
		if err := runCommand("verify", []string{"--root", root, "--workers", strconv.Itoa(workers)}); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown released-resource action %q", choice)
	}
}

func runPreloadResourceMenu(ctx context.Context, root, state, installer string, response Response, workers int) error {
	version := releaseVersion(response.Data.Preload)
	if version == "" {
		return nil
	}
	local, err := loadVersion(root, state)
	if err != nil {
		warnf("预下载需要已有正式版安装；请先安装当前正式版")
		return nil
	}
	current := releaseVersion(response.Data.Released)
	if current != "" && local.Version != current {
		warnf("本地版本 %s 落后于正式版 %s；请先运行 azp update", local.Version, current)
		return nil
	}
	if _, err := findPackage(response.Data.Preload, 2); err != nil {
		warnf("当前预下载没有可供本工具使用的 REALITY 资源")
		return nil
	}
	choice, err := chooseSimpleMenu(
		"预下载 "+versionDisplay(version, response.Data.Preload.Name),
		"预下载不会立即切换游戏版本；正式发布后再执行 update。",
		[]menuItem{
			{Key: "preload", Label: "开始预下载", Description: "下载与当前正式版不同的 REALITY 文件"},
			{Key: "back", Label: "返回", Description: "返回版本列表"},
		},
		0,
	)
	if err != nil || choice == "" || choice == "back" {
		return err
	}
	if err := preloadRelease(ctx, root, state, installer, response, workers); err != nil {
		return err
	}
	printWorkflowNextSteps(os.Stdout, "preload", root, state, response)
	return nil
}

func runResourceSelector(ctx context.Context, root, state, installer string, response Response, workers int) error {
	if !terminalAvailable() {
		printResourceOverview(os.Stdout, root, state, response)
		return nil
	}

	for {
		local, localErr := loadVersion(root, state)
		releasedDesc := availablePackageLabel(response.Data.Released) + " · " + releaseStateLabel(root, state, response)
		items := []menuItem{
			{
				Key: "released",
				Label: "正式版 " + releaseVersion(response.Data.Released),
				Description: releasedDesc,
			},
		}
		if version := releaseVersion(response.Data.Preload); version != "" {
			items = append(items, menuItem{
				Key: "preload",
				Label: "预下载 " + version,
				Description: availablePackageLabel(response.Data.Preload) + " · " + preloadStateLabel(root, state, response),
			})
		}
		items = append(items, menuItem{Key: "back", Label: "返回", Description: "退出版本 / 资源选择"})

		subtitle := "官方只暴露当前正式版与当前预下载版。"
		if localErr == nil {
			subtitle = "本地 " + local.Version + " · " + subtitle
		}
		choice, err := chooseSimpleMenu("版本 / 资源选择", subtitle, items, 0)
		if err != nil || choice == "" || choice == "back" {
			return err
		}
		switch choice {
		case "released":
			if err := runReleasedResourceMenu(ctx, root, state, installer, response, workers); err != nil {
				return err
			}
		case "preload":
			if err := runPreloadResourceMenu(ctx, root, state, installer, response, workers); err != nil {
				return err
			}
		}
		// Refresh the local-only state labels after an action without another metadata fetch.
	}
}
