package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	lua "github.com/yuin/gopher-lua"
)

type chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type chats_page struct {
	chats        []chat
	selectedChat int
	scrollOffset int
	container    *pageContainer
	forwarding   struct {
		isForwarding bool
		MsgID        string
	}
}

type chatsLoadedMsg []chat

func new_chats_page(container *pageContainer) chats_page {
	if container == nil {
		panic("passed nil container")
	}

	cp := chats_page{}
	cp.container = container
	cp.selectedChat = 0
	cp.scrollOffset = 0
	return cp
}

func (cp chats_page) Init() tea.Cmd {
	return nil
}

func (cp *chats_page) registerLuaFuncs() {
	L := cp.container.app.luaState
	L.SetGlobal("chat_escape", L.NewFunction(func(L *lua.LState) int {
		cp.container.commands = append(cp.container.commands, tea.Quit)
		return 0
	}))

	L.SetGlobal("chat_scroll_up", L.NewFunction(func(L *lua.LState) int {
		if cp.selectedChat > 0 {
			cp.selectedChat--
		}
		return 0
	}))
	L.SetGlobal("chat_scroll_down", L.NewFunction(func(L *lua.LState) int {
		if cp.selectedChat < len(cp.chats)-1 {
			cp.selectedChat++
		}
		return 0
	}))

	L.SetGlobal("chat_select", L.NewFunction(func(L *lua.LState) int {
		if cp.forwarding.isForwarding {
			forwardMsgToChat(cp.chats[cp.selectedChat].ID, cp.forwarding.MsgID)
			time.Sleep(2 * time.Second)
		}

		cp.container.app.luaReturn = "go_messages"
		cp.container.commands = append(cp.container.commands, getMessages(cp.chats[cp.selectedChat].ID))
		return 0
	}))
}

func (cp chats_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatsLoadedMsg:
		for _, c := range msg {
			cp.container.app.id_to_name[c.ID] = c.Name
		}

		cp.chats = msg
		cp.scrollOffset = 0 // Reset scroll when loading chats
		setTerminalTitle("Whats-CLI")
		return cp, nil
	case tea.KeyMsg:
		cp.registerLuaFuncs()
		cp.container.app.luaReturn = "" // Reset lua return
		key := msg.String()

		// Look for Lua keybind
		L := cp.container.app.luaState

		// Try to run keybinds[key]()
		err := L.DoString(fmt.Sprintf(`
			local key = %q
			local f = chat_keybinds[key]
			if type(f) == "function" then
				f()
				handled = true
			end
		`, key))
		if err != nil {
			fmt.Println("Lua error:", err)
		}


		if cp.container.app.luaReturn != "" {
			switch cp.container.app.luaReturn {
			case "go_messages":
				mp := new_messages_page(cp.chats[cp.selectedChat], cp.container)
				cp.container.commands = append(mp.container.commands, getMessages(cp.chats[cp.selectedChat].ID))
				return mp, nil
			}
		}

		return cp, nil

	case webhookMsg:
		cp.container.app.flashMsg = "MSG FROM " + msg.Chat.Name
		cp.container.app.flashCount = 6 // 3 flashes (on/off cycles)
		cp.container.commands = append(cp.container.commands, tea.Batch(getChats(), flashTick()))
		return cp, nil
	}

	return cp, nil
}

func (cp chats_page) View() string {
	var b strings.Builder
	b.WriteString("Chats (↑ ↓ to navigate, Enter to open, Ctrl+C to quit):\n\n")

	if len(cp.chats) < 1 {
		b.WriteString("Loading chats...")
		return b.String()
	}
	availableHeight := cp.container.app.height - 3 // 1 for header, 1 for empty line, 1 for padding
	if availableHeight < 1 {
		availableHeight = 1
	}

	// Calculate which chats to show based on selection and available container.app.height
	startIndex := 0
	endIndex := len(cp.chats)

	// If we have more chats than available container.app.height, center the selection
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
			b.WriteString(fmt.Sprintf("> %s\n", styles["selectedStyle"].Render(name)))
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", styles["unselectedStyle"].Render(name)))
		}
	}
	return b.String()
}

func forwardMsgToChat(chatID string, msgID string) {
	res, err := http.Post(fmt.Sprintf("%s/client/1/message/%s/forward/%s", baseURL, msgID, chatID), "application/json", nil)
	if err != nil {
		panic(fmt.Errorf("failed to forward message: %w", err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		panic(fmt.Errorf("failed to forward message: %s", res.Status))
	}
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
