package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexferl/myrient_browser"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	p := tea.NewProgram(myrient_browser.InitialModel(), tea.WithAltScreen())

	go func() {
		<-sigChan
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
