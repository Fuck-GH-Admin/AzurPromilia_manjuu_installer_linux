package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type menuItem struct {
	Key         string
	Label       string
	Description string
	Blocked     string
}

func terminalAvailable() bool {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return false
	}
	in, inErr := os.Stdin.Stat()
	out, outErr := os.Stdout.Stat()
	if inErr != nil || outErr != nil ||
		in.Mode()&os.ModeCharDevice == 0 ||
		out.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	var inTerm, outTerm syscall.Termios
	return ioctlTermios(os.Stdin.Fd(), syscall.TCGETS, &inTerm) == nil &&
		ioctlTermios(os.Stdout.Fd(), syscall.TCGETS, &outTerm) == nil
}

func localMenuState(path string) (bool, string) {
	if strings.TrimSpace(path) == "" {
		return false, ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, ""
	}
	state := filepath.Join(abs, ".azurpromilia")
	v, err := loadVersion(abs, state)
	if err != nil {
		return false, ""
	}
	return true, v.Version
}

func buildMainMenu(path string) ([]menuItem, int) {
	installed, version := localMenuState(path)
	noPath := strings.TrimSpace(path) == ""
	updateHint := cachedGameUpdateHint(path)
	updateDescription := "将现有安装更新到当前正式版"
	if updateHint != "" {
		updateDescription = updateHint
	}

	blockPath := ""
	if noPath {
		blockPath = "请先设置游戏目录"
	}
	blockInstalled := ""
	if !installed {
		blockInstalled = "需要先安装游戏"
	}
	blockInstall := ""
	if installed {
		if version != "" {
			blockInstall = "已安装 " + version + "，请使用更新或修复"
		} else {
			blockInstall = "已检测到现有安装，请使用更新或修复"
		}
	}

	items := []menuItem{
		{Key: "status", Label: "状态", Description: "查看漂亮状态表、版本差异和下一步建议", Blocked: blockPath},
		{Key: "resources", Label: "版本 / 资源", Description: "浏览正式版、预下载版与官方可用资源模式", Blocked: blockPath},
		{Key: "install", Label: "安装", Description: "下载并安装当前正式版", Blocked: blockInstall},
		{Key: "update", Label: "更新", Description: updateDescription, Blocked: blockInstalled},
		{Key: "verify", Label: "校验", Description: "完整检查本地游戏文件，不下载", Blocked: blockInstalled},
		{Key: "sync", Label: "修复 / Sync", Description: "校验并重新下载缺失或损坏文件", Blocked: blockInstalled},
		{Key: "predownload", Label: "预下载", Description: "下载下一版本的差异资源（Preview）", Blocked: blockInstalled},
		{Key: "path", Label: "游戏目录", Description: "查看或修改默认游戏目录"},
		{Key: "advanced", Label: "高级选项", Description: "默认并发数、安装模式与 Shell Completion"},
		{Key: "upgrade", Label: "更新工具", Description: "从经过 SHA-256 校验的 GitHub Release 更新 azp"},
		{Key: "help", Label: "帮助", Description: "显示完整命令帮助"},
		{Key: "quit", Label: "退出", Description: "关闭菜单"},
	}

	initial := 0
	switch {
	case noPath:
		initial = 7
	case !installed:
		initial = 2
	default:
		initial = 0
	}
	return items, initial
}

func renderMainMenu(items []menuItem, selected int, path string) string {
	var b strings.Builder
	b.WriteString("\x1b[2J\x1b[H")
	b.WriteString("\x1b[1;36mAzur Promilia Linux Manager\x1b[0m\n\n")
	if path == "" {
		b.WriteString(" 游戏目录: \x1b[33m未设置\x1b[0m\n")
	} else {
		b.WriteString(" 游戏目录: " + path + "\n")
		if installed, version := localMenuState(path); installed {
			b.WriteString(" 本地版本: " + version + "\n")
		} else {
			b.WriteString(" 本地状态: 未安装\n")
		}
	}
	if cfg, err := loadCLIConfig(); err == nil {
		fmt.Fprintf(&b, " 默认设置: %d workers · %s\n", preferredWorkers(cfg), preferredInstallMode(cfg))
	}
	if hint := cachedGameUpdateHint(path); hint != "" {
		fmt.Fprintf(&b, " \x1b[1;33m⚠ %s\x1b[0m\n", hint)
	}
	b.WriteString("\n")

	for i, item := range items {
		pointer := "  "
		style := ""
		reset := ""
		if i == selected {
			pointer = "› "
			style = "\x1b[7;1m"
			reset = "\x1b[0m"
		} else if item.Blocked != "" {
			style = "\x1b[90m"
			reset = "\x1b[0m"
		}
		fmt.Fprintf(&b, " %s%s%-14s%s %s", pointer, style, item.Label, reset, item.Description)
		if item.Blocked != "" {
			fmt.Fprintf(&b, "  \x1b[33m[%s]\x1b[0m", item.Blocked)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n \x1b[90m↑/↓ 选择   Enter 确认   Esc/q 退出\x1b[0m\n")
	return b.String()
}

func ioctlTermios(fd uintptr, request uintptr, value *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		request,
		uintptr(unsafe.Pointer(value)),
		0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func enterMenuTerminal() (*syscall.Termios, error) {
	fd := os.Stdin.Fd()
	var original syscall.Termios
	if err := ioctlTermios(fd, syscall.TCGETS, &original); err != nil {
		return nil, err
	}
	raw := original
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1
	if err := ioctlTermios(fd, syscall.TCSETS, &raw); err != nil {
		return nil, err
	}
	fmt.Print("\x1b[?25l")
	return &original, nil
}

func leaveMenuTerminal(original *syscall.Termios) {
	if original != nil {
		_ = ioctlTermios(os.Stdin.Fd(), syscall.TCSETS, original)
	}
	fmt.Print("\x1b[0m\x1b[?25h\x1b[2J\x1b[H")
}

func readMenuKey() (string, error) {
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		switch buf[0] {
		case 3:
			return "quit", nil
		case '\r', '\n':
			return "enter", nil
		case 'q', 'Q':
			return "quit", nil
		case 27:
			seq := append([]byte(nil), buf[:n]...)
			if len(seq) == 1 {
				more := make([]byte, 4)
				m, readErr := os.Stdin.Read(more)
				if readErr != nil {
					return "", readErr
				}
				seq = append(seq, more[:m]...)
			}
			if len(seq) >= 3 && seq[1] == '[' {
				switch seq[2] {
				case 'A':
					return "up", nil
				case 'B':
					return "down", nil
				}
			}
			return "quit", nil
		}
	}
}

func chooseMainMenu(path string) (string, error) {
	items, selected := buildMainMenu(path)
	original, err := enterMenuTerminal()
	if err != nil {
		return "", err
	}
	defer leaveMenuTerminal(original)

	message := ""
	for {
		fmt.Print(renderMainMenu(items, selected, path))
		if message != "" {
			fmt.Printf("\n \x1b[33m%s\x1b[0m\n", message)
		}
		key, err := readMenuKey()
		if err != nil {
			return "", err
		}
		switch key {
		case "up":
			selected = (selected - 1 + len(items)) % len(items)
			message = ""
		case "down":
			selected = (selected + 1) % len(items)
			message = ""
		case "quit":
			return "quit", nil
		case "enter":
			item := items[selected]
			if item.Blocked != "" {
				message = item.Blocked
				continue
			}
			return item.Key, nil
		}
	}
}

func renderSimpleMenu(title, subtitle string, items []menuItem, selected int) string {
	var b strings.Builder
	b.WriteString("\x1b[2J\x1b[H")
	fmt.Fprintf(&b, "\x1b[1;36m%s\x1b[0m\n", title)
	if subtitle != "" {
		fmt.Fprintf(&b, "\x1b[90m%s\x1b[0m\n", subtitle)
	}
	b.WriteString("\n")
	for i, item := range items {
		pointer := "  "
		style := ""
		reset := ""
		if i == selected {
			pointer = "› "
			style = "\x1b[7;1m"
			reset = "\x1b[0m"
		}
		fmt.Fprintf(&b, " %s%s%-18s%s %s\n", pointer, style, item.Label, reset, item.Description)
	}
	b.WriteString("\n \x1b[90m↑/↓ 选择   Enter 确认   Esc/q 返回\x1b[0m\n")
	return b.String()
}

func chooseSimpleMenu(title, subtitle string, items []menuItem, selected int) (string, error) {
	if len(items) == 0 {
		return "", errors.New("menu has no options")
	}
	if selected < 0 || selected >= len(items) {
		selected = 0
	}
	original, err := enterMenuTerminal()
	if err != nil {
		return "", err
	}
	defer leaveMenuTerminal(original)
	for {
		fmt.Print(renderSimpleMenu(title, subtitle, items, selected))
		key, err := readMenuKey()
		if err != nil {
			return "", err
		}
		switch key {
		case "up":
			selected = (selected - 1 + len(items)) % len(items)
		case "down":
			selected = (selected + 1) % len(items)
		case "quit":
			return "", nil
		case "enter":
			return items[selected].Key, nil
		}
	}
}

func runAdvancedMenu() error {
	for {
		cfg, err := loadCLIConfig()
		if err != nil {
			return err
		}
		workers := preferredWorkers(cfg)
		mode := preferredInstallMode(cfg)
		action, err := chooseSimpleMenu(
			"高级选项",
			"这里保存的是默认值；命令行参数仍可临时覆盖。",
			[]menuItem{
				{Key: "workers", Label: "并发下载", Description: fmt.Sprintf("当前 %d workers", workers)},
				{Key: "mode", Label: "安装包模式", Description: "当前 " + mode},
				{Key: "completion", Label: "Shell Completion", Description: "为当前 shell 安装命令补全"},
				{Key: "back", Label: "返回", Description: "回到主菜单"},
			},
			0,
		)
		if err != nil || action == "" || action == "back" {
			return err
		}
		switch action {
		case "workers":
			options := []int{2, 4, 8, 16, 24, 32}
			items := make([]menuItem, 0, len(options))
			selected := 0
			for i, n := range options {
				items = append(items, menuItem{Key: fmt.Sprintf("%d", n), Label: fmt.Sprintf("%d workers", n), Description: "下载与校验并发数"})
				if n == workers {
					selected = i
				}
			}
			choice, err := chooseSimpleMenu("默认并发数", "较高并发不一定更快；默认推荐 4。", items, selected)
			if err != nil || choice == "" {
				if err != nil { return err }
				continue
			}
			n, err := strconv.Atoi(choice)
			if err != nil { return err }
			if err := setPreferredWorkers(n); err != nil { return err }
		case "mode":
			items := []menuItem{
				{Key: "auto", Label: "auto", Description: "跟随官方 downloadMode（推荐）"},
				{Key: "reality", Label: "reality", Description: "强制逐文件模式"},
				{Key: "slice", Label: "slice", Description: "强制分片 ZIP 模式（仍属 Preview）"},
			}
			selected := 0
			for i, item := range items {
				if item.Key == mode { selected = i }
			}
			choice, err := chooseSimpleMenu("默认安装模式", "只影响 install；服务器模式仍建议保持 auto。", items, selected)
			if err != nil || choice == "" {
				if err != nil { return err }
				continue
			}
			if err := setPreferredInstallMode(choice); err != nil { return err }
		case "completion":
			if err := runCompletionCommand([]string{"install"}); err != nil {
				return err
			}
			fmt.Print("按 Enter 返回高级选项...")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		}
	}
}

func promptGamePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	defaultPath := filepath.Join(home, "Games", "AzurPromilia")
	fmt.Printf("游戏目录 [%s]: ", defaultPath)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return "", err
	}
	path := strings.TrimSpace(line)
	if path == "" {
		path = defaultPath
	}
	remembered, err := rememberGamePath(path)
	if err != nil {
		return "", err
	}
	fmt.Printf("已记住游戏目录: %s\n", remembered)
	return remembered, nil
}

func runInteractiveMenu() error {
	for {
		path, err := configuredGamePath()
		if err != nil {
			return err
		}
		action, err := chooseMainMenu(path)
		if err != nil {
			return err
		}
		switch action {
		case "quit", "":
			return nil
		case "help":
			printTopLevelHelp(os.Stdout, cliProgramName())
			return nil
		case "path":
			if _, err := promptGamePath(); err != nil {
				return err
			}
			time.Sleep(350 * time.Millisecond)
			continue
		case "advanced":
			if err := runAdvancedMenu(); err != nil {
				return err
			}
			continue
		case "resources":
			if err := runCLI([]string{"resources"}); err != nil {
				return err
			}
			continue
		case "upgrade":
			return runCLI([]string{"upgrade"})
		case "install":
			if path == "" {
				if _, err := promptGamePath(); err != nil {
					return err
				}
			}
			return runCLI([]string{"install"})
		default:
			return runCLI([]string{action})
		}
	}
}
