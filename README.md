# myrient-browser

A terminal-based file browser and downloader for [Myrient](https://myrient.erista.me/files/), featuring concurrent downloads, resume support, automatic extraction, and a clean TUI interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Interactive TUI** - Browse Myrient's file directory structure with keyboard navigation
- **Concurrent Downloads** - Download up to 10 files simultaneously
- **Resume Support** - Automatically resume interrupted downloads where they left off
- **Pre-scan Option** - Check file sizes before downloading (can be disabled for faster starts)
- **Auto-extraction** - Automatically unzip downloaded files
- **Flexible Extraction** - Extract to individual folders or current directory
- **Pause/Resume** - Pause and resume downloads on the fly
- **Real-time Progress** - Live download speed, ETA, and progress tracking
- **Filtering** - Quick filter to find files in large directories
- **Error Handling** - Proper error display with context

## Installation

```shell
go install github.com/alexferl/myrient_browser@latest
```

Or build from source:

```shell
git clone https://github.com/alexferl/myrient-browser.git
cd myrient-browser
go build -o myrient_browser ./cmd/myrient_browser
```

## Usage

Simply run the executable:
```shell
myrient_browser
```

Downloads will be saved to `./downloads/` in the current directory.

## Keyboard Controls

### Navigation
- `↑`/`↓` - Move cursor up/down
- `←` - Go back to parent directory
- `→`/`Enter` - Open directory or download file
- `PgUp`/`PgDn` - Scroll page up/down
- `Home`/`End` - Jump to first/last item
- `/` - Filter current directory
- `Esc` - Clear filter (when filtering)

### Actions
- `d` - Download all files in current view (respects filters)
- `Enter` - Download single file (when on a file)

### Download Controls
- `p` - Pause download
- `r` - Resume paused download
- `Esc` - Cancel scan (during scanning) or cancel download (when paused)

### Options (Toggle)
- `s` - **PreScan**: Check file sizes before downloading (ON by default)
- `x` - **Auto-extract**: Automatically unzip downloaded files (OFF by default)
- `f` - **Extract to Folder**: Create separate folder per zip file (OFF by default)
- `z` - **Delete Zip**: Delete zip files after extraction (OFF by default)

### Exit
- `q` or `Ctrl+C` - Quit application

## How It Works

### Browsing
The application uses [colly](https://github.com/gocolly/colly) to scrape the Myrient file directory, parsing the HTML tables to display directories and files in a navigable interface.

### Downloading
Downloads use Go's standard `http` package with the following features:

1. **Resume Support**: Uses HTTP Range headers to resume partial downloads from `.part` files
2. **Concurrent Downloads**: Spawns up to 10 worker goroutines to download files in parallel
3. **Pre-scanning**: Optionally checks file sizes via HEAD requests before downloading to calculate total download size and show accurate progress
4. **Progress Tracking**: Uses atomic operations to safely track bytes downloaded across concurrent workers

### Extraction
ZIP files are extracted using Go's `archive/zip` package with path traversal protection to prevent zip-slip vulnerabilities.

## Architecture

- **Model** (`types.go`) - Application state including files, download stats, UI state
- **Update** (`update.go`) - Handles all user input and state transitions
- **View** (`view.go`) - Renders the current state to the terminal
- **Commands** (`download.go`, `model.go`) - Async operations that return messages

Key components:
- `download.go` - Download orchestration, file info fetching, concurrent workers
- `extract.go` - ZIP extraction logic
- `model.go` - Directory loading and filtering
- `view.go` - TUI rendering with progress bars and status

## Configuration

Currently hardcoded in `types.go`:
- `baseURL` - Myrient base URL (default: `https://myrient.erista.me/files/`)
- `numWorkers` - Concurrent download workers (default: 10)

## Requirements

- Go 1.25 or later

## License

[MIT License](LICENSE)

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.
