package main

import (
	"os"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/term"
)



type pageContainer struct {
	page 	tea.Model
	commands []tea.Cmd
	app		*app
}

func new_page_container(page tea.Model, app *app) *pageContainer {
	pc := &pageContainer{}
	pc.commands = make([]tea.Cmd, 0)
	pc.page = page
	pc.app = app
	return pc
}

func (pc * pageContainer) update (msg tea.Msg) {
	pc.commands = make([]tea.Cmd, 0)
	p, _ := pc.page.Update(msg);
	pc.page = p
	pc.page.Init()
}

type app struct {
	page_conatiner	*pageContainer
	id_to_name		map[string]string
	width           int
	height          int
	flashMsg        string       // message to flash in bottom bar
	flashCount      int          // counter for flash animation
	luaState	*lua.LState 
	luaReturn	string
}

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
	a.luaState = lua.NewState()
	lua.OpenIo(a.luaState)
	lua.OpenOs(a.luaState)
	luaPath, err := ensureLuaPath()
	if err != err {
		panic("could not find lua path")
	}

	if err := a.luaState.DoFile(luaPath + "/init.lua"); err != nil {
		panic(fmt.Errorf("error loading init.lua: %w", err))
	}
	if err := a.luaState.DoFile(luaPath + "/colors.lua"); err != nil {
		panic(fmt.Errorf("error loading colors.lua: %w", err))
	}
	setup_styles(a.luaState)

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
	
	m.page_conatiner.update(msg);
	cmds = append(cmds, m.page_conatiner.commands...)
	return m, tea.Batch(cmds...)
}

func (m app) View() string {
	m.page_conatiner.app = &m
	return m.page_conatiner.page.View()
}

