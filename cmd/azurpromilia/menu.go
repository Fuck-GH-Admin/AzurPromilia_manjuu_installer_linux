package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	in, inErr := os.Stdin.Stat()
	out, outErr := os.Stdout.Stat()
	return inErr == nil && outErr == nil &&
		in.Mode()&os.ModeCharDevice != 0 &&
		out.Mode()&os.ModeCharDevice != 0
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
		{Key: "status", Label: "状态", Description: "查看本地与官方版本、更新状态和磁盘信息", Blocked: blockPath},
		{Key: "install", Label: "安装", Description: "下载并安装当前正式版", Blocked: blockInstall},
		{Key: "update", Label: "更新", Description: "将现有安装更新到当前正式版", Blocked: blockInstalled},
		{Key: "verify", Label: "校验", Description: "完整检查本地游戏文件，不下载", Blocked: blockInstalled},
		{Key: "sync", Label: "修复 / Sync", Description: "校验并重新下载缺失或损坏文件", Blocked: blockInstalled},
		{Key: "predownload", Label: "预下载", Description: "下载下一版本的差异资源（Preview）", Blocked: blockInstalled},
		{Key: "path", Label: "游戏目录", Description: "查看或修改默认游戏目录"},
		{Key: "help", Label: "帮助", Description: "显示完整命令帮助"},
		{Key: "quit", Label: "退出", Description: "关闭菜单"},
	}

	initial := 0
	switch {
	case noPath:
		initial = 6
	case !installed:
		initial = 1
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
