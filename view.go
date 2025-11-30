package myrient_browser

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	// Show error if present
	if m.lastError != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("red")).Bold(true)
		return errorStyle.Render("❌ Error: "+m.lastError) + "\n\n" +
			"Press any key to continue..."
	}

	if m.downloading && m.downloadStats != nil {
		s := strings.Builder{}

		if m.downloadStats.extracting {
			extracted := atomic.LoadInt32(&m.downloadStats.extracted)
			total := m.downloadStats.total
			s.WriteString(fmt.Sprintf("\nExtracting files: %d/%d\n\n", extracted, total))
			percent := float64(extracted) / float64(total)
			s.WriteString(m.progress.ViewAs(percent) + "\n\n")
			s.WriteString("Almost done...\n")
			return s.String()
		}

		if m.downloadStats.scanning {
			scanned := atomic.LoadInt32(&m.downloadStats.scanProgress)
			total := m.downloadStats.total
			s.WriteString(fmt.Sprintf("\nScanning files: %d/%d\n\n", scanned, total))
			percent := float64(scanned) / float64(total)
			s.WriteString(m.progress.ViewAs(percent) + "\n\n")
			s.WriteString("[Esc] Cancel scan [Ctrl+C] Quit\n")
			return s.String()
		}

		completed := atomic.LoadInt32(&m.downloadStats.completed)
		total := m.downloadStats.total
		bytesDownload := atomic.LoadInt64(&m.downloadStats.bytesDownload)
		bytesTotal := atomic.LoadInt64(&m.downloadStats.bytesTotal)

		var percent float64
		if bytesTotal > 0 {
			percent = float64(bytesDownload) / float64(bytesTotal)
		} else {
			percent = float64(completed) / float64(total)
		}

		var elapsed time.Duration
		if m.paused {
			elapsed = m.pausedTime + time.Since(m.pauseStart)
		} else {
			elapsed = m.pausedTime + time.Since(m.startTime)
		}

		speed := 0.0
		eta := "calculating..."

		if !m.paused {
			now := time.Now()
			if m.downloadStats.lastTime.IsZero() {
				m.downloadStats.lastTime = m.startTime
				m.downloadStats.lastBytes = 0
			}

			timeDiff := now.Sub(m.downloadStats.lastTime).Seconds()
			if timeDiff >= 0.1 {
				bytesDiff := bytesDownload - m.downloadStats.lastBytes
				instantSpeed := float64(bytesDiff) / timeDiff / 1024 / 1024

				if m.downloadStats.currentSpeed == 0 {
					m.downloadStats.currentSpeed = instantSpeed
				} else {
					m.downloadStats.currentSpeed = 0.7*m.downloadStats.currentSpeed + 0.3*instantSpeed
				}

				m.downloadStats.lastTime = now
				m.downloadStats.lastBytes = bytesDownload
			}

			speed = m.downloadStats.currentSpeed
			if speed > 0.1 && bytesTotal > 0 {
				remainingBytes := bytesTotal - bytesDownload
				etaSeconds := float64(remainingBytes) / (speed * 1024 * 1024)
				if etaSeconds > 0 {
					eta = time.Duration(etaSeconds * float64(time.Second)).Round(time.Second).String()
				}
			}
		} else {
			speed = 0
			eta = "paused"
		}

		statusText := "Downloading"
		if m.paused {
			statusText = lipgloss.NewStyle().Foreground(lipgloss.Color("yellow")).Render("PAUSED")
		}

		s.WriteString(fmt.Sprintf("\n%s: %d/%d files (%.1f%%)\n", statusText, completed, total, percent*100))
		if bytesTotal > 0 {
			s.WriteString(fmt.Sprintf("Progress: %.2f MB / %.2f MB\n\n",
				float64(bytesDownload)/1024/1024, float64(bytesTotal)/1024/1024))
		} else {
			s.WriteString("\n")
		}

		s.WriteString(m.progress.ViewAs(percent) + "\n\n")
		s.WriteString(fmt.Sprintf("Speed: %.2f MB/s | Elapsed: %s", speed, elapsed.Round(time.Second)))
		if bytesTotal > 0 {
			s.WriteString(fmt.Sprintf(" | ETA: %s", eta))
		}
		s.WriteString("\n\n")

		if m.paused {
			s.WriteString("[r] Resume [Esc] Cancel [Ctrl+C] Quit\n")
		} else {
			s.WriteString("[p] Pause [Ctrl+C] Quit\n")
		}

		if m.status != "" {
			s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("green")).Render(m.status))
		}

		return s.String()
	}

	s := strings.Builder{}

	displayPath := "Root"
	if m.currentPath != "" {
		decodedPath, err := url.QueryUnescape(m.currentPath)
		if err != nil {
			decodedPath = m.currentPath
		}
		displayPath = decodedPath
	}

	title := fmt.Sprintf("Myrient Browser - %s", displayPath)
	s.WriteString(lipgloss.NewStyle().Bold(true).Render(title) + "\n\n")

	if m.filtering {
		s.WriteString("Filter: " + m.filterInput.View() + "\n")
	} else if m.filterInput.Value() != "" {
		s.WriteString(fmt.Sprintf("Filter: %s (press / to edit, Esc to clear)\n", m.filterInput.Value()))
	}

	s.WriteString("\n")

	start := m.viewport.offset
	end := m.viewport.offset + m.viewport.height
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	if start > 0 {
		s.WriteString(" ↑ More items above...\n")
	}

	for i := start; i < end; i++ {
		entry := m.entries[m.filtered[i]]
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}

		icon := "📄"
		if strings.HasSuffix(entry.Path, "/") || entry.Path == "../" {
			icon = "📁"
		}

		style := lipgloss.NewStyle()
		if i == m.cursor {
			style = style.Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
		}

		s.WriteString(style.Render(fmt.Sprintf("%s %s %s", cursor, icon, entry.Name)) + "\n")
	}

	if end < len(m.filtered) {
		s.WriteString(" ↓ More items below...\n")
	}

	totalEntries := len(m.entries)
	filteredCount := len(m.filtered)
	filterInfo := ""
	if filteredCount < totalEntries {
		filterInfo = fmt.Sprintf(" [%d/%d filtered]", filteredCount, totalEntries)
	}

	// Styles for ON/OFF
	greenOn := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("ON")
	grayOff := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("OFF")

	// Build options list
	preScanStatus := grayOff
	if !m.skipScan {
		preScanStatus = greenOn
	}

	extractStatus := grayOff
	if m.autoExtract {
		extractStatus = greenOn
	}

	folderStatus := grayOff
	if m.extractToFolder {
		folderStatus = greenOn
	}

	deleteStatus := grayOff
	if m.deleteZip {
		deleteStatus = greenOn
	}

	// Build help text
	help := fmt.Sprintf("\n[%d/%d]%s\n\n", m.cursor+1, filteredCount, filterInfo)
	help += fmt.Sprintf("PreScan: %s Extract: %s Folder: %s Delete: %s\n\n",
		preScanStatus, extractStatus, folderStatus, deleteStatus)
	help += "Navigation: [↑↓] Move [PgUp/PgDn] Scroll [Home/End] Jump [/] Filter\n"
	help += "Actions: [→/Enter] Open [d] Download All [←] Back [q] Quit\n"
	help += "Options: [s] PreScan [x] Extract [f] Folder [z] Delete Zip"

	if m.status != "" {
		help = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("green")).Render(m.status) + help
	}

	return s.String() + help
}
