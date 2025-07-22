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
	if _, err := tea.NewProgram(initialApp(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
