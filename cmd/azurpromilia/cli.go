package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cliConfig struct {
	GamePath     string `json:"gamePath"`
	Workers      int    `json:"workers,omitempty"`
	InstallMode  string `json:"installMode,omitempty"`
}

func cliProgramName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." {
		return "azp"
	}
	return name
}

func cliConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "azurpromilia", "config.json"), nil
}

func loadCLIConfig() (cliConfig, error) {
	var cfg cliConfig
	path, err := cliConfigPath()
	if err != nil {
		return cfg, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid CLI config %s: %w", path, err)
	}
	return cfg, nil
}

func saveCLIConfig(cfg cliConfig) error {
	configPath, err := cliConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(configPath, b, 0600)
}

func rememberGamePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("game path cannot be empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	cfg.GamePath = abs
	if err := saveCLIConfig(cfg); err != nil {
		return "", err
	}
	return abs, nil
}

func preferredWorkers(cfg cliConfig) int {
	if cfg.Workers >= 1 && cfg.Workers <= 32 {
		return cfg.Workers
	}
	return 4
}

func preferredInstallMode(cfg cliConfig) string {
	switch cfg.InstallMode {
	case "auto", "reality", "slice":
		return cfg.InstallMode
	default:
		return "auto"
	}
}

func setPreferredWorkers(workers int) error {
	if workers < 1 || workers > 32 {
		return errors.New("workers must be 1..32")
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	cfg.Workers = workers
	return saveCLIConfig(cfg)
}

func setPreferredInstallMode(mode string) error {
	if mode != "auto" && mode != "reality" && mode != "slice" {
		return errors.New("install mode must be auto, reality or slice")
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	cfg.InstallMode = mode
	return saveCLIConfig(cfg)
}

func hasOption(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func configuredGamePath() (string, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.GamePath), nil
}

type commonCLIOptions struct {
	Path      string
	Workers   string
	Help      bool
	Version   bool
	Verbose   bool
	Quiet     bool
	NoColor   bool
	Rest      []string
}

func parseCommonCLIOptions(args []string) (commonCLIOptions, error) {
	var out commonCLIOptions
	commandSeen := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			out.Help = true
		case arg == "--version" && !commandSeen:
			out.Version = true
		case arg == "-v" || arg == "--verbose":
			out.Verbose = true
		case arg == "-q" || arg == "--quiet":
			out.Quiet = true
		case arg == "--no-color":
			out.NoColor = true
		case arg == "-p" || arg == "--path" || arg == "--root":
			if i+1 >= len(args) {
				return out, fmt.Errorf("%s requires a path", arg)
			}
			i++
			out.Path = args[i]
		case strings.HasPrefix(arg, "--path="):
			out.Path = strings.TrimPrefix(arg, "--path=")
		case strings.HasPrefix(arg, "--root="):
			out.Path = strings.TrimPrefix(arg, "--root=")
		case arg == "-j" || arg == "--workers":
			if i+1 >= len(args) {
				return out, fmt.Errorf("%s requires a number", arg)
			}
			i++
			out.Workers = args[i]
		case strings.HasPrefix(arg, "--workers="):
			out.Workers = strings.TrimPrefix(arg, "--workers=")
		default:
			out.Rest = append(out.Rest, arg)
			if !commandSeen && !strings.HasPrefix(arg, "-") {
				commandSeen = true
			}
		}
	}
	if out.Workers != "" {
		n, err := strconv.Atoi(out.Workers)
		if err != nil || n < 1 || n > 32 {
			return out, errors.New("workers must be 1..32")
		}
	}
	return out, nil
}

func canonicalCommand(command string) string {
	switch command {
	case "sync":
		return "repair"
	case "predownload":
		return "preload"
	default:
		return command
	}
}

func printTopLevelHelp(w io.Writer, prog string) {
	fmt.Fprintf(w, `Azur Promilia Linux Manager

Usage:
  %s [options] <command> [command options]

Commands:
  install       Download and install the game
  update        Update an existing installation
  repair        Verify and repair missing or damaged files
  sync          Alias for repair
  verify        Verify installed files without downloading
  status        Show the current official version and download mode
  preload       Pre-download the next release (preview)
  predownload   Alias for preload
  path          Show or set the remembered game directory
  upgrade       Update the azp tool from verified GitHub Releases
  completion    Generate or install shell completion
  help          Show this help or help for one command

Run without a command in a terminal to open the interactive menu.

Global options:
  -p, --path PATH       Game directory; remembered for future commands
  -j, --workers N       Parallel workers, 1..32; overrides remembered default
  -v, --verbose         Show per-file and diagnostic details
  -q, --quiet           Only show warnings, errors and command results
      --no-color        Disable ANSI colors
      --version         Show tool version
  -h, --help            Show help

First use:
  %s -p "$HOME/Games/AzurPromilia" install

After that:
  %s status
  %s verify
  %s sync

Compatibility:
  --root is still accepted as an alias for --path.
  The long executable name "azurpromilia" remains supported.
  Advanced legacy command "download" is kept for diagnostics.

`, prog, prog, prog, prog, prog)
}

func printCommandHelp(w io.Writer, prog, command string) {
	command = canonicalCommand(command)
	switch command {
	case "install":
		fmt.Fprintf(w, `Usage:
  %s install [options]

Download and install the current official Manjuu / SC000 release.

Common options:
  -p, --path PATH       Game directory; remembered for future commands
  -j, --workers N       Parallel downloads, 1..32 (default: 4)

Advanced options:
  --mode MODE           auto, reality, or slice (default: auto)
  --installer FILE      Override the automatically acquired SC000 launcher
  --packageconfig FILE  Use an existing official packageconfig JSON
  --python FILE         Python executable for metadata bootstrap
  --wine FILE           Wine executable for metadata bootstrap
  --refresh             Refresh cached official metadata

Example:
  %s -p "$HOME/Games/AzurPromilia" install

`, prog, prog)
	case "update":
		fmt.Fprintf(w, `Usage:
  %s update [options]

Update an existing installation to the current official release.

Common options:
  -p, --path PATH       Game directory; remembered for future commands
  -j, --workers N       Parallel downloads, 1..32 (default: 4)

Advanced metadata options:
  --installer FILE
  --packageconfig FILE
  --python FILE
  --wine FILE
  --refresh

`, prog)
	case "repair":
		fmt.Fprintf(w, `Usage:
  %s repair [options]
  %s sync [options]

Verify against the current official file list and re-download only missing or damaged files.

Options:
  -p, --path PATH       Game directory; remembered for future commands
  -j, --workers N       Parallel downloads, 1..32 (default: 4)

Advanced metadata options:
  --installer FILE
  --packageconfig FILE
  --python FILE
  --wine FILE
  --refresh

`, prog, prog)
	case "verify":
		fmt.Fprintf(w, `Usage:
  %s verify [options]

Verify installed files without downloading replacements.

Options:
  -p, --path PATH       Game directory; remembered for future commands
  -j, --workers N       Parallel verification workers, 1..32 (default: 4)

Advanced options:
  --target released|preload
  --version VERSION     Required when verifying a preload
  --no-extract          Skip archive extraction verification

`, prog)
	case "status":
		fmt.Fprintf(w, `Usage:
  %s status [options]

Show a concise player-facing status page: install state, local/official versions,
update availability, download mode, local manifest summary, launch entry,
pre-download state and free disk space.

Options:
  -p, --path PATH       Game directory; remembered for future commands

Advanced metadata options:
  --installer FILE
  --packageconfig FILE
  --python FILE
  --wine FILE
  --refresh

`, prog)
	case "info":
		fmt.Fprintf(w, `Usage:
  %s info [options]

Show detailed official package, CDN and launch metadata for diagnostics.

Options:
  -p, --path PATH       Game directory; remembered for future commands

Advanced metadata options:
  --installer FILE
  --packageconfig FILE
  --python FILE
  --wine FILE
  --refresh

`, prog)
	case "preload":
		fmt.Fprintf(w, `Usage:
  %s preload [options]
  %s predownload [options]

Pre-download files that differ in the next official release.
This workflow is still preview / not fully live-upgrade validated.

Options:
  -p, --path PATH       Game directory; remembered for future commands
  -j, --workers N       Parallel downloads, 1..32 (default: 4)

Advanced metadata options:
  --installer FILE
  --packageconfig FILE
  --python FILE
  --wine FILE
  --refresh

`, prog, prog)
	case "path":
		fmt.Fprintf(w, `Usage:
  %s path
  %s path /path/to/AzurPromilia

Show or change the remembered game directory.

`, prog, prog)
	case "upgrade":
		fmt.Fprintf(w, `Usage:
  %s upgrade [--check] [--stable] [--force]

Update the azp tool itself from this project's GitHub Releases.
The release archive is SHA-256 verified before extraction and replacement.

Options:
  --check               Check which Release would be selected without installing
  --stable              Ignore pre-releases and use the latest stable Release
  --force               Reinstall the selected Release; also permits replacing a dev build

`, prog)
	case "completion":
		fmt.Fprintf(w, `Usage:
  %s completion [bash|zsh|fish]
  %s completion install [bash|zsh|fish]

Print a completion script, or explicitly install it into the user completion directory.
With no shell argument, the shell is detected from $SHELL.

Examples:
  %s completion bash
  %s completion install
  %s completion install fish

`, prog, prog, prog, prog, prog)
	case "download":
		fmt.Fprintf(w, `Usage:
  %s download [advanced options]

Legacy low-level resource command kept for diagnostics and compatibility.
Normal players should use "%s install" instead.

Run the player guide for details on --packageconfig, --target, --mode and probe options.

`, prog, prog)
	default:
		fmt.Fprintf(w, "Unknown command %q.\n\n", command)
		printTopLevelHelp(w, prog)
	}
}

func runPathCommand(args []string) error {
	prog := cliProgramName()
	if len(args) > 1 {
		return fmt.Errorf("usage: %s path [GAME_DIRECTORY]", prog)
	}
	if len(args) == 1 {
		path, err := rememberGamePath(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Game path: %s\n", path)
		return nil
	}
	path, err := configuredGamePath()
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("no game path configured; set one with: %s path %q", prog, "$HOME/Games/AzurPromilia")
	}
	fmt.Println(path)
	return nil
}

func runCLI(args []string) error {
	prog := cliProgramName()
	opts, err := parseCommonCLIOptions(args)
	if err != nil {
		return err
	}
	if err := configureOutput(opts.Verbose, opts.Quiet, opts.NoColor); err != nil {
		return err
	}
	if opts.Version {
		fmt.Printf("Azur Promilia Linux Manager %s\n", toolVersion)
		return nil
	}
	if len(opts.Rest) == 0 {
		if opts.Help {
			printTopLevelHelp(os.Stdout, prog)
			return nil
		}
		if opts.Path != "" {
			path, err := rememberGamePath(opts.Path)
			if err != nil {
				return err
			}
			fmt.Printf("Game path: %s\n", path)
		}
		if terminalAvailable() {
			return runInteractiveMenu()
		}
		printTopLevelHelp(os.Stdout, prog)
		return nil
	}

	command := opts.Rest[0]
	commandArgs := append([]string(nil), opts.Rest[1:]...)
	if command == "help" {
		if len(commandArgs) == 0 {
			printTopLevelHelp(os.Stdout, prog)
			return nil
		}
		printCommandHelp(os.Stdout, prog, commandArgs[0])
		return nil
	}
	if opts.Help {
		printCommandHelp(os.Stdout, prog, command)
		return nil
	}
	if command == "path" {
		if opts.Path != "" {
			commandArgs = []string{opts.Path}
		}
		return runPathCommand(commandArgs)
	}
	if command == "completion" {
		return runCompletionCommand(commandArgs)
	}
	if command == "upgrade" {
		return runUpgradeCommand(commandArgs)
	}

	canonical := canonicalCommand(command)
	switch canonical {
	case "install", "update", "repair", "verify", "status", "info", "preload", "download":
	default:
		printTopLevelHelp(os.Stderr, prog)
		return fmt.Errorf("unknown command %q", command)
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	gamePath := opts.Path
	if gamePath != "" {
		gamePath, err = rememberGamePath(gamePath)
		if err != nil {
			return err
		}
		cfg.GamePath = gamePath
	} else {
		gamePath = strings.TrimSpace(cfg.GamePath)
	}
	if gamePath == "" {
		return fmt.Errorf("no game path configured; first run: %s -p %q %s", prog, "$HOME/Games/AzurPromilia", command)
	}
	commandArgs = append(commandArgs, "--root", gamePath)
	if opts.Workers != "" {
		commandArgs = append(commandArgs, "--workers", opts.Workers)
	} else {
		commandArgs = append(commandArgs, "--workers", strconv.Itoa(preferredWorkers(cfg)))
	}
	if canonical == "install" && !hasOption(commandArgs, "--mode") {
		commandArgs = append(commandArgs, "--mode", preferredInstallMode(cfg))
	}
	return runCommand(canonical, commandArgs)
}
