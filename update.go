package myrient_browser

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.height = msg.Height - 12
		return m, nil

	case errMsg:
		m.lastError = msg.err.Error()
		m.downloading = false
		m.status = ""
		return m, nil

	case dirLoadedMsg:
		m.entries = msg
		m.cursor = 0
		m.viewport.offset = 0
		m.filtering = false
		m.filterInput.SetValue("")
		m.updateFilter()
		m.status = ""
		return m, nil

	case scanCompleteMsg:
		if m.downloadStats != nil {
			m.downloadStats.scanning = false
			m.downloadStats.bytesTotal = msg.totalBytes
			return m, tea.Batch(startDownloadWithFiles(msg.files, m.downloadStats, m.ctx, m.autoExtract, m.extractToFolder, m.deleteZip), tickCmd())
		}
		return m, nil

	case tickMsg:
		if m.downloading && m.downloadStats != nil {
			if m.paused {
				return m, tickCmd()
			}

			if m.downloadStats.extracting {
				extracted := atomic.LoadInt32(&m.downloadStats.extracted)
				if extracted >= m.downloadStats.total {
					m.downloading = false
					elapsed := m.pausedTime + time.Since(m.startTime)
					m.status = fmt.Sprintf("✓ Downloaded and extracted %d files in %s",
						m.downloadStats.total, elapsed.Round(time.Second))
					return m, nil
				}
				return m, tickCmd()
			}

			completed := atomic.LoadInt32(&m.downloadStats.completed)
			if completed >= m.downloadStats.total {
				if m.autoExtract {
					m.downloadStats.extracting = true
					return m, tickCmd()
				}

				m.downloading = false
				elapsed := m.pausedTime + time.Since(m.startTime)
				bytesDownload := atomic.LoadInt64(&m.downloadStats.bytesDownload)
				avgSpeed := float64(bytesDownload) / elapsed.Seconds() / 1024 / 1024
				m.status = fmt.Sprintf("✓ Downloaded %d files in %s (avg %.2f MB/s)",
					m.downloadStats.total, elapsed.Round(time.Second), avgSpeed)
				return m, nil
			}

			return m, tickCmd()
		}
		return m, nil

	case downloadCompleteMsg:
		m.downloading = false
		if m.downloadStats != nil {
			elapsed := m.pausedTime + time.Since(m.startTime)
			bytesDownload := atomic.LoadInt64(&m.downloadStats.bytesDownload)
			avgSpeed := float64(bytesDownload) / elapsed.Seconds() / 1024 / 1024
			m.status = fmt.Sprintf("✓ Downloaded %d files in %s (avg %.2f MB/s)",
				m.downloadStats.total, elapsed.Round(time.Second), avgSpeed)
		}
		return m, nil

	case tea.KeyMsg:
		// Handle error dismissal
		if m.lastError != "" {
			m.lastError = ""
			return m, nil
		}

		if m.downloading {
			// Check if we're scanning
			if m.downloadStats != nil && m.downloadStats.scanning {
				switch msg.String() {
				case "ctrl+c":
					m.cancel()
					return m, tea.Quit
				case "esc":
					m.cancel()
					m.downloading = false
					m.status = "Scan cancelled"
					return m, nil
				}
				return m, nil
			}

			switch msg.String() {
			case "ctrl+c":
				m.cancel()
				return m, tea.Quit
			case "p":
				if !m.paused {
					m.paused = true
					atomic.StoreInt32(&m.downloadStats.paused, 1)
					m.pauseStart = time.Now()
					m.status = "Paused - Press [r] to resume or [Esc] to cancel"
				}
				return m, nil
			case "r":
				if m.paused {
					m.paused = false
					atomic.StoreInt32(&m.downloadStats.paused, 0)
					m.pausedTime += time.Since(m.pauseStart)
					m.startTime = time.Now()
					m.downloadStats.lastTime = time.Time{}
					m.status = "Resumed downloading..."
				}
				return m, nil
			case "esc":
				if m.paused {
					m.cancel()
					m.downloading = false
					m.paused = false
					m.status = "Download cancelled"
					return m, nil
				}
				return m, nil
			}
			return m, nil
		}

		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterInput.SetValue("")
				m.updateFilter()
				return m, nil
			case "enter":
				m.filtering = false
				return m, nil
			default:
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.updateFilter()
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "s":
			m.skipScan = !m.skipScan
			if m.skipScan {
				m.status = "Scan disabled - downloads will start immediately"
			} else {
				m.status = "Scan enabled - will check file sizes before downloading"
			}

		case "x":
			m.autoExtract = !m.autoExtract
			if m.autoExtract {
				m.status = "Auto-extract enabled - will unzip files after download"
			} else {
				m.status = "Auto-extract disabled"
			}

		case "f":
			m.extractToFolder = !m.extractToFolder
			if m.extractToFolder {
				m.status = "Extract to folder: ON - creates folder per zip file"
			} else {
				m.status = "Extract to folder: OFF - extracts directly to current directory"
			}

		case "z":
			m.deleteZip = !m.deleteZip
			if m.deleteZip {
				m.status = "Delete zip: ON - will delete zip files after extraction"
			} else {
				m.status = "Delete zip: OFF - keeps zip files after extraction"
			}

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.viewport.offset {
					m.viewport.offset = m.cursor
				}
			}

		case "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.viewport.offset+m.viewport.height {
					m.viewport.offset = m.cursor - m.viewport.height + 1
				}
			}

		case "pgup":
			m.cursor -= m.viewport.height
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.viewport.offset = m.cursor

		case "pgdown":
			m.cursor += m.viewport.height
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor >= m.viewport.offset+m.viewport.height {
				m.viewport.offset = m.cursor - m.viewport.height + 1
			}

		case "home":
			m.cursor = 0
			m.viewport.offset = 0

		case "end":
			m.cursor = len(m.filtered) - 1
			if m.cursor >= m.viewport.height {
				m.viewport.offset = m.cursor - m.viewport.height + 1
			}

		case "d":
			var files []fileEntry
			for _, idx := range m.filtered {
				entry := m.entries[idx]
				if !strings.HasSuffix(entry.Path, "/") && entry.Path != "../" {
					files = append(files, entry)
				}
			}

			if len(files) == 0 {
				m.status = "No files to download in current view"
				return m, nil
			}

			m.downloading = true
			m.paused = false
			m.pausedTime = 0
			m.downloadStats = &downloadStats{
				total:    int32(len(files)),
				scanning: !m.skipScan,
			}
			m.startTime = time.Now()

			if m.skipScan {
				m.status = fmt.Sprintf("Starting download of %d files...", len(files))
				return m, tea.Batch(downloadAllFiles(m.currentPath, files, m.downloadStats, m.ctx, m.autoExtract, m.extractToFolder, m.deleteZip), tickCmd())
			} else {
				m.status = fmt.Sprintf("Scanning %d files...", len(files))
				return m, tea.Batch(scanAndDownload(m.currentPath, files, m.downloadStats, m.ctx), tickCmd())
			}

		case "right", "enter":
			if m.cursor >= len(m.filtered) {
				return m, nil
			}

			entry := m.entries[m.filtered[m.cursor]]
			isDir := strings.HasSuffix(entry.Path, "/")

			if isDir {
				if entry.Path == "../" {
					if len(m.pathStack) > 0 {
						m.currentPath = m.pathStack[len(m.pathStack)-1]
						m.pathStack = m.pathStack[:len(m.pathStack)-1]
					} else {
						m.currentPath = ""
					}
				} else {
					m.pathStack = append(m.pathStack, m.currentPath)
					if m.currentPath == "" {
						m.currentPath = entry.Path
					} else {
						m.currentPath = m.currentPath + entry.Path
					}
				}
				return m, loadDirectory(m.currentPath)
			} else {
				m.downloading = true
				m.paused = false
				m.pausedTime = 0
				m.downloadStats = &downloadStats{
					total:    1,
					scanning: !m.skipScan,
				}
				m.startTime = time.Now()
				m.status = fmt.Sprintf("Downloading %s...", entry.Name)
				files := []fileEntry{entry}

				if m.skipScan {
					return m, tea.Batch(downloadAllFiles(m.currentPath, files, m.downloadStats, m.ctx, m.autoExtract, m.extractToFolder, m.deleteZip), tickCmd())
				} else {
					return m, tea.Batch(scanAndDownload(m.currentPath, files, m.downloadStats, m.ctx), tickCmd())
				}
			}

		case "left":
			if len(m.pathStack) > 0 {
				m.currentPath = m.pathStack[len(m.pathStack)-1]
				m.pathStack = m.pathStack[:len(m.pathStack)-1]
				m.status = ""
				return m, loadDirectory(m.currentPath)
			} else if m.currentPath != "" {
				m.currentPath = ""
				m.status = ""
				return m, loadDirectory("")
			}
		}
	}

	return m, nil
}
