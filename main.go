package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	relink := flag.Bool("relink", false, "clear the local session and import chats from a new WhatsApp link")
	flag.Parse()

	cmdChan := make(chan tea.Msg, 64)
	client, err := connectWhatsmeow(cmdChan, *relink)
	if err != nil {
		return err
	}
	whatsApp = client
	defer whatsApp.Close()

	a := initialApp()
	p := tea.NewProgram(*a, tea.WithAltScreen())

	go func() {
		for msg := range cmdChan {
			p.Send(msg)
		}
	}()

	_, err = p.Run()
	return err
}
