package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type loading_page struct {
	container *pageContainer
}

func new_loading_page(container *pageContainer) loading_page {
	if container == nil {
		panic("passed nil cotainer")
	}
	return loading_page{container: container}
}

func (lp loading_page) View() string {
	return "loading chats" + fmt.Sprintf("count: %v\n", lp.container)
}
func (lp loading_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cp := new_chats_page(lp.container)
	lp.container.commands = append(lp.container.commands, getChats())
	return cp, nil
}
func (lp loading_page) Init() tea.Cmd {
	return nil
}
