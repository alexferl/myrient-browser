package myrient_browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gocolly/colly"
)

func InitialModel() *Model {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 156

	ctx, cancel := context.WithCancel(context.Background())

	m := &Model{
		entries:         []fileEntry{},
		filtered:        []int{},
		currentPath:     "",
		pathStack:       []string{},
		filterInput:     ti,
		filtering:       false,
		progress:        progress.New(progress.WithDefaultGradient()),
		skipScan:        false,
		autoExtract:     false,
		extractToFolder: false,
		deleteZip:       false,
		ctx:             ctx,
		cancel:          cancel,
	}

	return m
}

func (m *Model) Init() tea.Cmd {
	return loadDirectory("")
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadDirectory(path string) tea.Cmd {
	return func() tea.Msg {
		var entries []fileEntry
		c := colly.NewCollector()

		c.OnHTML("table tr", func(e *colly.HTMLElement) {
			fileName := strings.TrimSpace(e.ChildText("td:nth-child(1)"))
			href := e.ChildAttr("td:nth-child(1) a", "href")

			if fileName == "" || fileName == "File Name" ||
				fileName == "./" || fileName == "Parent directory/" ||
				(path == "" && fileName == "../") {
				return
			}

			if fileName == "../" {
				entries = append(entries, fileEntry{Name: "..", Path: "../"})
				return
			}

			decodedName, err := url.QueryUnescape(fileName)
			if err != nil {
				decodedName = fileName
			}

			entries = append(entries, fileEntry{
				Name: decodedName,
				Path: href,
			})
		})

		if err := c.Visit(baseURL + path); err != nil {
			return errMsg{err: fmt.Errorf("failed to load directory: %w", err)}
		}

		return dirLoadedMsg(entries)
	}
}

func (m *Model) updateFilter() {
	filterText := strings.ToLower(m.filterInput.Value())
	m.filtered = []int{}

	if filterText == "" {
		for i := range m.entries {
			m.filtered = append(m.filtered, i)
		}
	} else {
		for i, entry := range m.entries {
			if strings.Contains(strings.ToLower(entry.Name), filterText) {
				m.filtered = append(m.filtered, i)
			}
		}
	}

	if len(m.filtered) > 0 {
		m.cursor = 0
		m.viewport.offset = 0
	}
}
