package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const baseURL = "http://localhost:3000"


func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	cmdChan := make(chan tea.Msg, 10)
	startWebhookListener(cmdChan)


	a := initialApp()
	p := tea.NewProgram(*a, tea.WithAltScreen())

	go func() {

		for msg := range cmdChan {
			p.Send(msg)
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
