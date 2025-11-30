package myrient_browser

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
)

const (
	baseURL    = "https://myrient.erista.me/files/"
	numWorkers = 10
)

type Model struct {
	entries         []fileEntry
	filtered        []int
	cursor          int
	currentPath     string
	pathStack       []string
	downloading     bool
	paused          bool
	status          string
	downloadStats   *downloadStats
	startTime       time.Time
	pausedTime      time.Duration
	pauseStart      time.Time
	viewport        struct{ offset, height int }
	filterInput     textinput.Model
	filtering       bool
	progress        progress.Model
	skipScan        bool
	autoExtract     bool
	extractToFolder bool
	deleteZip       bool
	ctx             context.Context
	cancel          context.CancelFunc
	lastError       string
}

type fileEntry struct {
	Name string
	Path string
}

type fileInfo struct {
	url       string
	filename  string
	path      string
	size      int64
	resumable bool
}

type downloadStats struct {
	completed     int32
	bytesDownload int64
	bytesTotal    int64
	total         int32
	scanning      bool
	scanProgress  int32
	lastBytes     int64
	lastTime      time.Time
	currentSpeed  float64
	extracting    bool
	extracted     int32
	paused        int32
}

type (
	dirLoadedMsg        []fileEntry
	downloadCompleteMsg struct{}
	tickMsg             time.Time
	scanCompleteMsg     struct {
		totalBytes int64
		files      []fileInfo
	}
	errMsg struct {
		err error
	}
)
