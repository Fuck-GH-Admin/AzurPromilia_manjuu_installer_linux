package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type outputLevel int32

const (
	levelQuiet outputLevel = iota
	levelNormal
	levelVerbose
)

var currentOutputLevel atomic.Int32
var outputColor atomic.Bool
var activeProgress atomic.Bool

func init() {
	currentOutputLevel.Store(int32(levelNormal))
	outputColor.Store(stderrLooksInteractive() && os.Getenv("NO_COLOR") == "")
}

func configureOutput(verbose, quiet, noColor bool) error {
	if verbose && quiet {
		return fmt.Errorf("--verbose and --quiet cannot be used together")
	}
	switch {
	case verbose:
		currentOutputLevel.Store(int32(levelVerbose))
	case quiet:
		currentOutputLevel.Store(int32(levelQuiet))
	default:
		currentOutputLevel.Store(int32(levelNormal))
	}
	outputColor.Store(!noColor && os.Getenv("NO_COLOR") == "" && stderrLooksInteractive())
	return nil
}

func stderrLooksInteractive() bool {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	st, err := os.Stderr.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func isVerboseOutput() bool {
	return outputLevel(currentOutputLevel.Load()) >= levelVerbose
}

func isQuietOutput() bool {
	return outputLevel(currentOutputLevel.Load()) == levelQuiet
}

func progressIsInteractive() bool {
	return stderrLooksInteractive() && !isQuietOutput()
}

func progressInterval() time.Duration {
	if progressIsInteractive() {
		return time.Second
	}
	return 5 * time.Second
}

func styledText(text, style string) string {
	if !outputColor.Load() || style == "" {
		return text
	}
	return style + text + "\x1b[0m"
}

func clearProgressLocked() {
	if activeProgress.Load() && progressIsInteractive() {
		fmt.Fprint(os.Stderr, "\r\x1b[2K")
	}
}

func writeLogLine(name, color string, min outputLevel, format string, args ...any) {
	if outputLevel(currentOutputLevel.Load()) < min {
		return
	}
	output.Lock()
	defer output.Unlock()
	clearProgressLocked()
	fmt.Fprintf(os.Stderr, "%s %s\n", styledText("["+name+"]", color), fmt.Sprintf(format, args...))
}

func debugf(format string, args ...any) {
	writeLogLine("DEBUG", "\x1b[90m", levelVerbose, format, args...)
}

func infof(format string, args ...any) {
	writeLogLine("INFO", "\x1b[36m", levelNormal, format, args...)
}

func warnf(format string, args ...any) {
	writeLogLine("WARN", "\x1b[33m", levelQuiet, format, args...)
}

func errorf(format string, args ...any) {
	writeLogLine("ERROR", "\x1b[31m", levelQuiet, format, args...)
}

func progressBar(done, total int64, width int) string {
	if width < 4 {
		width = 4
	}
	if total <= 0 {
		return strings.Repeat("░", width)
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func renderTransferProgress(label string, done, total int64, verified, failed, totalFiles int64, bytesPerSecond float64) {
	if isQuietOutput() {
		return
	}
	if total < 0 {
		total = 0
	}
	pct := 0.0
	if total > 0 {
		pct = float64(done) * 100 / float64(total)
		if pct > 100 {
			pct = 100
		}
	}
	speed := fmt.Sprintf("%.2f MiB/s", bytesPerSecond/(1<<20))
	if progressIsInteractive() {
		output.Lock()
		defer output.Unlock()
		activeProgress.Store(true)
		fmt.Fprintf(os.Stderr,
			"\r\x1b[2K%s [%s] %5.1f%%  %s / %s  %s  文件 %d/%d",
			label, progressBar(done, total, 24), pct,
			formatBytes(uint64(max64(done, 0))), formatBytes(uint64(max64(total, 0))),
			speed, verified, totalFiles,
		)
		if failed > 0 {
			fmt.Fprintf(os.Stderr, "  失败 %d", failed)
		}
		return
	}
	infof("%s %.1f%% | %s/%s | %s | files %d/%d failed=%d",
		label, pct,
		formatBytes(uint64(max64(done, 0))), formatBytes(uint64(max64(total, 0))),
		speed, verified, totalFiles, failed)
}

func finishTransferProgress(label string, verified, failed, totalFiles, transferred int64, err error) {
	if isQuietOutput() {
		activeProgress.Store(false)
		return
	}
	status, statusEN := "完成", "complete"
	switch {
	case failed == 0 && errors.Is(err, context.Canceled):
		status, statusEN = "已中断", "interrupted"
	case err != nil || failed > 0:
		status, statusEN = "失败", "failed"
	}
	if progressIsInteractive() {
		output.Lock()
		clearProgressLocked()
		activeProgress.Store(false)
		fmt.Fprintf(os.Stderr, "%s %s：文件 %d/%d，失败 %d，传输 %s\n",
			label, status, verified, totalFiles, failed, formatBytes(uint64(max64(transferred, 0))))
		output.Unlock()
		return
	}
	infof("%s %s: files %d/%d failed=%d transferred=%s",
		label, statusEN, verified, totalFiles, failed, formatBytes(uint64(max64(transferred, 0))))
}

func max64(v, floor int64) int64 {
	if v < floor {
		return floor
	}
	return v
}
