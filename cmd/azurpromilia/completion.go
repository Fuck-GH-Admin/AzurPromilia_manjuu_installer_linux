package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var completionCommands = []string{
	"install", "update", "repair", "sync", "verify", "status", "info",
	"preload", "predownload", "path", "upgrade", "completion", "help",
}

func detectShell() (string, error) {
	name := filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	switch name {
	case "bash", "zsh", "fish":
		return name, nil
	default:
		return "", fmt.Errorf("cannot detect a supported shell from $SHELL=%q; choose bash, zsh, or fish", os.Getenv("SHELL"))
	}
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q; choose bash, zsh, or fish", shell)
	}
}

func bashCompletionScript() string {
	return `# azp shell completion for bash
_azp_complete() {
    local cur prev cmd
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "$prev" in
        -p|--path|--root|--installer|--packageconfig|--python|--wine)
            COMPREPLY=( $(compgen -f -- "$cur") )
            return 0
            ;;
        -j|--workers)
            COMPREPLY=( $(compgen -W "2 4 8 16 24 32" -- "$cur") )
            return 0
            ;;
        --mode)
            COMPREPLY=( $(compgen -W "auto reality slice" -- "$cur") )
            return 0
            ;;
        --target)
            COMPREPLY=( $(compgen -W "released preload" -- "$cur") )
            return 0
            ;;
    esac

    cmd=""
    local i
    for ((i=1; i<COMP_CWORD; i++)); do
        case "${COMP_WORDS[i]}" in
            install|update|repair|sync|verify|status|info|preload|predownload|path|upgrade|completion|help)
                cmd="${COMP_WORDS[i]}"
                break
                ;;
        esac
    done

    if [[ -z "$cmd" ]]; then
        COMPREPLY=( $(compgen -W "install update repair sync verify status info preload predownload path upgrade completion help -p --path -j --workers -v --verbose -q --quiet --no-color --version -h --help" -- "$cur") )
        return 0
    fi

    case "$cmd" in
        install)
            COMPREPLY=( $(compgen -W "-p --path -j --workers --mode --installer --packageconfig --python --wine --refresh -v --verbose -q --quiet --no-color -h --help" -- "$cur") )
            ;;
        update|repair|sync|preload|predownload)
            COMPREPLY=( $(compgen -W "-p --path -j --workers --installer --packageconfig --python --wine --refresh -v --verbose -q --quiet --no-color -h --help" -- "$cur") )
            ;;
        verify)
            COMPREPLY=( $(compgen -W "-p --path -j --workers --target --version --no-extract -v --verbose -q --quiet --no-color -h --help" -- "$cur") )
            ;;
        status|info)
            COMPREPLY=( $(compgen -W "-p --path --installer --packageconfig --python --wine --refresh -v --verbose -q --quiet --no-color -h --help" -- "$cur") )
            ;;
        upgrade)
            COMPREPLY=( $(compgen -W "--check --stable --force -v --verbose -q --quiet --no-color -h --help" -- "$cur") )
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish install -h --help" -- "$cur") )
            ;;
        path)
            COMPREPLY=( $(compgen -d -- "$cur") )
            ;;
        help)
            COMPREPLY=( $(compgen -W "install update repair sync verify status info preload predownload path upgrade completion" -- "$cur") )
            ;;
    esac
}
complete -F _azp_complete azp azurpromilia
`
}

func zshCompletionScript() string {
	return `#compdef azp azurpromilia

_azp() {
  local -a commands
  commands=(
    'install:下载并安装当前正式版'
    'update:更新已有游戏安装'
    'repair:校验并修复游戏文件'
    'sync:repair 的别名'
    'verify:只校验游戏文件'
    'status:查看玩家状态摘要'
    'info:查看详细官方元数据'
    'preload:预下载下一版本'
    'predownload:preload 的别名'
    'path:查看或修改游戏目录'
    'upgrade:更新 azp 工具本身'
    'completion:生成或安装 shell 补全'
    'help:查看命令帮助'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    _arguments \
      '-p[游戏目录]:directory:_directories' \
      '--path[游戏目录]:directory:_directories' \
      '-j[并发数]:workers:(2 4 8 16 24 32)' \
      '--workers[并发数]:workers:(2 4 8 16 24 32)' \
      '-v[详细输出]' '--verbose[详细输出]' \
      '-q[安静模式]' '--quiet[安静模式]' \
      '--no-color[禁用颜色]' '--version[显示工具版本]' \
      '-h[帮助]' '--help[帮助]'
    return
  fi

  case "$words[2]" in
    install)
      _arguments '*:file:_files' '--mode[包模式]:mode:(auto reality slice)' '--installer[启动器]:file:_files' '--packageconfig[官方包清单]:file:_files' '--python[Python]:file:_files' '--wine[Wine]:file:_files' '--refresh[刷新元数据]' '-j[并发数]:workers:(2 4 8 16 24 32)' '--workers[并发数]:workers:(2 4 8 16 24 32)'
      ;;
    update|repair|sync|preload|predownload)
      _arguments '--installer[启动器]:file:_files' '--packageconfig[官方包清单]:file:_files' '--python[Python]:file:_files' '--wine[Wine]:file:_files' '--refresh[刷新元数据]' '-j[并发数]:workers:(2 4 8 16 24 32)' '--workers[并发数]:workers:(2 4 8 16 24 32)'
      ;;
    verify)
      _arguments '--target[校验目标]:target:(released preload)' '--version[预下载版本]:version:' '--no-extract[跳过解压内容校验]' '-j[并发数]:workers:(2 4 8 16 24 32)' '--workers[并发数]:workers:(2 4 8 16 24 32)'
      ;;
    status|info)
      _arguments '--installer[启动器]:file:_files' '--packageconfig[官方包清单]:file:_files' '--python[Python]:file:_files' '--wine[Wine]:file:_files' '--refresh[刷新元数据]'
      ;;
    upgrade)
      _arguments '--check[只检查更新]' '--stable[只使用稳定版]' '--force[即使版本相同也重新安装]'
      ;;
    completion)
      _values 'completion action' bash zsh fish install
      ;;
    path)
      _directories
      ;;
    help)
      _describe 'command' commands
      ;;
  esac
}

_azp "$@"
`
}

func fishCompletionScript() string {
	var b strings.Builder
	b.WriteString("# azp shell completion for fish\n")
	for _, prog := range []string{"azp", "azurpromilia"} {
		fmt.Fprintf(&b, "complete -c %s -f\n", prog)
		for _, cmd := range completionCommands {
			fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -a %s\n", prog, cmd)
		}
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -s p -l path -r -a '(__fish_complete_directories)' -d 'Game directory'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -s j -l workers -r -a '2 4 8 16 24 32' -d 'Parallel workers'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -s v -l verbose -d 'Verbose output'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -s q -l quiet -d 'Quiet output'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -l no-color -d 'Disable color'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -l version -d 'Show tool version'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from install' -l mode -r -a 'auto reality slice'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from verify' -l target -r -a 'released preload'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from upgrade' -l check -d 'Check only'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from upgrade' -l stable -d 'Stable releases only'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from upgrade' -l force -d 'Reinstall current release'\n", prog)
		fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish install'\n", prog)
	}
	return b.String()
}

func completionInstallPaths(shell string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	switch shell {
	case "bash":
		base := filepath.Join(home, ".local", "share", "bash-completion", "completions")
		return []string{filepath.Join(base, "azp"), filepath.Join(base, "azurpromilia")}, nil
	case "zsh":
		return []string{filepath.Join(home, ".zfunc", "_azp")}, nil
	case "fish":
		return []string{filepath.Join(home, ".config", "fish", "completions", "azp.fish")}, nil
	default:
		return nil, fmt.Errorf("unsupported shell %q", shell)
	}
}

func installCompletion(shell string) ([]string, error) {
	script, err := completionScript(shell)
	if err != nil {
		return nil, err
	}
	paths, err := completionInstallPaths(shell)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := writeFileAtomic(path, []byte(script), 0644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func runCompletionCommand(args []string) error {
	if len(args) > 2 {
		return errors.New("usage: azp completion [bash|zsh|fish|install [shell]]")
	}
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		printCommandHelp(os.Stdout, cliProgramName(), "completion")
		return nil
	}
	if len(args) > 0 && args[0] == "install" {
		shell := ""
		var err error
		if len(args) == 2 {
			shell = args[1]
		} else {
			shell, err = detectShell()
			if err != nil {
				return err
			}
		}
		paths, err := installCompletion(shell)
		if err != nil {
			return err
		}
		fmt.Printf("Installed %s completion:\n", shell)
		for _, path := range paths {
			fmt.Printf("  %s\n", path)
		}
		if shell == "zsh" {
			fmt.Println("If ~/.zfunc is not already in fpath, add: fpath=(~/.zfunc $fpath); autoload -Uz compinit && compinit")
		}
		fmt.Println("Restart the shell (or reload its completion system) to activate it.")
		return nil
	}

	shell := ""
	var err error
	if len(args) == 1 {
		shell = args[0]
	} else {
		shell, err = detectShell()
		if err != nil {
			return err
		}
	}
	script, err := completionScript(shell)
	if err != nil {
		return err
	}
	fmt.Print(script)
	return nil
}
