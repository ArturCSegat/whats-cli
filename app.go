package main

import (
	"os"
	"golang.org/x/term"
	tea "github.com/charmbracelet/bubbletea"
)

type app struct {
	page            tea.Model
	width           int
	height          int
	flashMsg        string       // message to flash in bottom bar
	flashCount      int          // counter for flash animation
}




func initialApp() app {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	var a app
	a = app{
		page:            loading_page{from_app: &a},
		width:           width,
		height:          height,
		flashMsg: 		 "",
		flashCount: 	 0,
	}
	return a
}

func (m app) Init() tea.Cmd {
	return nil
}


func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case flashTickMsg:
		if m.flashCount > 0 {
			m.flashCount--
			if m.flashCount == 0 {
				m.flashMsg = ""
			}
			return m, flashTick()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.page, cmd = m.page.Update(msg);
	return m, cmd
}

func (m app) View() string {
	return m.page.View()
}
