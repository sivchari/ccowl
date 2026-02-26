# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CCOwl is a macOS/Linux status bar application that monitors Claude Code usage in real-time. It displays session costs in the system tray using the `systray` library and fetches data from the `ccusage` CLI tool.

## Build and Development Commands

```bash
# Build
go build -o ccowl

# Run tests
go test -v ./...

# Run a single test
go test -v -run TestFormatCompactTitle ./...

# Lint
go vet ./...

# Run the app (requires ccusage in PATH)
./ccowl
```

## Architecture

Single-file Go application (`main.go`) with these key components:

- **Data structures**: `CCUsageResponse`, `Block`, `TokenCounts`, `BurnRate`, `Projection` - mirror the JSON output from `ccusage blocks --live --json --active`
- **systray integration**: Uses `github.com/getlantern/systray` for cross-platform system tray functionality
- **Update loop**: Polls `ccusage` every 5 seconds via `updateStatus()` → `getCCUsageData()`
- **i18n**: Simple locale detection (`LANG`/`LC_ALL`/`LC_MESSAGES`) with English/Japanese support via `t()` helper

## Dependencies

- Requires `ccusage` CLI tool in PATH (part of Claude Code)
- Linux: needs `libayatana-appindicator3-dev` and `libgtk-3-dev`
