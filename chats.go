package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type chats_page struct {
	chats           []chat
	selectedChat    int
	scrollOffset 	int
	from_app		*app
}
type chatsLoadedMsg []chat

func new_chats_page(app *app) chats_page {
	cp := chats_page{}
	cp.from_app = app
	cp.selectedChat = 0 
	cp.scrollOffset = 0
	return cp
}




func (cp chats_page) Init() tea.Cmd {
	return nil
}

func (cp chats_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatsLoadedMsg:
		cp.chats = msg
		cp.scrollOffset = 0 // Reset scroll when loading chats
		setTerminalTitle("Whats-CLI")
		return cp, nil
	case tea.KeyMsg:
		if cp.from_app.flashCount > 0 {
			cp.from_app.flashCount = 0
			cp.from_app.flashMsg = ""
		}
		key := msg.String();
		switch key {
		case "ctrl+c":
		case "esc":
			return cp, tea.Quit
		case "up":
			if cp.selectedChat > 0 {
				cp.selectedChat--
			}
		case "down":
			if cp.selectedChat < len(cp.chats)-1 {
				cp.selectedChat++
			}
		case "enter":
			mp := new_messages_page(cp.chats[cp.selectedChat], cp.from_app)
			return mp, getMessages(cp.chats[cp.selectedChat].ID)
		}
		
	case webhookMsg:
		cp.from_app.flashMsg = "MSG FROM " + msg.Chat.Name
		cp.from_app.flashCount = 6 // 3 flashes (on/off cycles)
		return cp, tea.Batch(getChats(), flashTick())
	}

	return cp, nil
}


func (cp chats_page) View() string {
		var b strings.Builder
		b.WriteString("Chats (↑ ↓ to navigate, Enter to open, Ctrl+C to quit):\n\n")

		if len(cp.chats) < 1 {
			b.WriteString("Loading chats...");
			return b.String()
		}
		availableHeight := cp.from_app.height - 3 // 1 for header, 1 for empty line, 1 for padding
		if availableHeight < 1 {
			availableHeight = 1
		}

		// Calculate which chats to show based on selection and available from_app.height
		startIndex := 0
		endIndex := len(cp.chats)

		// If we have more chats than available from_app.height, center the selection
		if len(cp.chats) > availableHeight {
			startIndex = cp.selectedChat - availableHeight/2
			if startIndex < 0 {
				startIndex = 0
			}
			endIndex = startIndex + availableHeight
			if endIndex > len(cp.chats) {
				endIndex = len(cp.chats)
				startIndex = endIndex - availableHeight
				if startIndex < 0 {
					startIndex = 0
				}
			}
		}

		for i := startIndex; i < endIndex; i++ {
			c := cp.chats[i]
			name := c.Name
			if name == "" {
				name = c.ID
			}
			if i == cp.selectedChat {
				b.WriteString(fmt.Sprintf("> %s\n", selectedStyle.Render(name)))
			} else {
				b.WriteString(fmt.Sprintf("  %s\n", unselectedStyle.Render(name)))
			}
		}
		return b.String()
}

func getChats() tea.Cmd {
	return func() tea.Msg {
		res, err := http.Get(fmt.Sprintf("%s/client/1/chat", baseURL))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		var chats []chat
		if err := json.NewDecoder(res.Body).Decode(&chats); err != nil {
			return err
		}
		return chatsLoadedMsg(chats)
	}
}
