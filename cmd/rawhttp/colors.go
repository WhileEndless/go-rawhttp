package main

import (
	"os"

	"golang.org/x/term"
)

// This file holds the CLI-side colour/beautify policy (TTY detection and the
// --color/--beautify/NO_COLOR/FORCE_COLOR precedence). The actual rendering
// (painter, beautify, highlight) lives in the reusable render package.

// colorEnabledFor is the shared color-gating policy.
// Precedence: --no-color / NO_COLOR off > --color / FORCE_COLOR on > auto.
// In auto mode color is emitted only to an interactive TTY (not a file/pipe).
func colorEnabledFor(cfg *Config, isTTY, toFile bool) bool {
	if cfg.NoColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if cfg.Color || os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if toFile {
		return false
	}
	return isTTY
}

// resolveColor decides whether ANSI color should be emitted to stdout.
func resolveColor(cfg *Config, toFile bool) bool {
	return colorEnabledFor(cfg, stdoutIsTTY(), toFile)
}

// resolveBeautify decides whether to reindent/pretty-print the body.
// Precedence: --no-beautify off > --beautify on > auto (TTY, not a file).
func resolveBeautify(cfg *Config, toFile bool) bool {
	if cfg.NoBeautify {
		return false
	}
	if cfg.Beautify {
		return true
	}
	if toFile {
		return false
	}
	return stdoutIsTTY()
}

func stdoutIsTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
func stderrIsTTY() bool { return term.IsTerminal(int(os.Stderr.Fd())) }
