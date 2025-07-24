package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type pageContainer struct {
	page 	tea.Model
	app		*app
}

func new_page_container(page tea.Model, app *app) *pageContainer {
	pc := &pageContainer{}
	pc.page = page
	pc.app = app
	return pc
}

func (pc * pageContainer) update (msg tea.Msg) tea.Cmd {
	p, cmd := pc.page.Update(msg);
	pc.page = p
	return cmd
}

type app struct {
	page_conatiner	*pageContainer
	id_to_name		map[string]string
	width           int
	height          int
	flashMsg        string       // message to flash in bottom bar
	flashCount      int          // counter for flash animation
}

type updateAppMsg *app

func initialApp() *app {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	a  := &app{}
	pc := new_page_container(loading_page{}, a);
	lp := new_loading_page(pc)
	pc.page = lp
	a.page_conatiner = pc
	a.width = width
	a.height = height
	a.flashCount = 0
	a.flashMsg = ""
	a.id_to_name = make(map[string]string)
	return a
}

func (m app) Init() tea.Cmd {
	m.page_conatiner.app = &m
	return nil
}


func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.page_conatiner.app = &m;
	cmds := make([]tea.Cmd, 0)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case updateFlashMsg:
		m.flashCount = msg.count
		m.flashMsg = msg.msg
		cmds = append(cmds, flashTick())
	case flashTickMsg:
		if m.flashCount > 0 {
			m.flashCount--
			if m.flashCount == 0 {
				m.flashMsg = ""
			}
			cmds = append(cmds, flashTick())
		}
	}
	
	cmd := m.page_conatiner.update(msg);
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m app) View() string {
	m.page_conatiner.app = &m
	return m.page_conatiner.page.View()
}
