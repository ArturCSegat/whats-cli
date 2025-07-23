package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

type error_page struct {
	err error
}
type errMsg error

func (ep error_page) View() string {
	return "Error: " + ep.err.Error()
}
func (ep error_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case error:
		ep.err = m
	}
	return ep, nil
}
func (ep error_page) Init() tea.Cmd {
	return nil
}

type loading_page struct {
	from_app *app
}

func (lp loading_page) View() string {
	return "loading chats"
}
func (lp loading_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cp := new_chats_page(lp.from_app)
	return cp, getChats()
}
func (lp loading_page) Init() tea.Cmd {
	return nil
}
