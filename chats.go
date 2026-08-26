package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	lua "github.com/yuin/gopher-lua"
)

type Chat struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	namePriority uint8
	UnreadCount  int     `json:"unreadCount"`
	LastMessage  message `json:"lastMessage"`
	IsArchived   bool    `json:"isArchived"`
	IsGroup      bool    `json:"isGroup"`
	IsMuted      bool    `json:"isMuted"`
	IsReadOnly   bool    `json:"isReadOnly"`
	IsPinned     bool    `json:"isPinned"`
}
type chats_page struct {
	chats        []Chat
	selectedChat int
	scrollOffset int
	curr_line    int
	container    *pageContainer
	forwarding   struct {
		isForwarding bool
		MsgID        string
	}
	lines []string
}

type chatsLoadedMsg []Chat

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
			cp.curr_line -= len(strings.Split(cp.renderChat(cp.chats[cp.selectedChat], cp.selectedChat), "\n"))
			cp.selectedChat--
			if cp.curr_line < cp.scrollOffset {
				cp.scrollOffset = cp.curr_line - (cp.container.app.height - 6)
			}
		}
		return 0
	}))
	L.SetGlobal("chat_scroll_down", L.NewFunction(func(L *lua.LState) int {
		if cp.selectedChat < len(cp.chats)-1 {
			cp.curr_line += len(strings.Split(cp.renderChat(cp.chats[cp.selectedChat], cp.selectedChat), "\n"))
			cp.selectedChat++
			if cp.curr_line >= cp.scrollOffset+(cp.container.app.height-3) {
				cp.scrollOffset = cp.curr_line
			}

		}
		return 0
	}))

	L.SetGlobal("chat_select", L.NewFunction(func(L *lua.LState) int {
		if cp.forwarding.isForwarding {
			if err := forwardMsgToChat(cp.chats[cp.selectedChat].ID, cp.forwarding.MsgID); err != nil {
				cp.container.commands = append(cp.container.commands, flash(updateFlashMsg{msg: "Forward: " + err.Error(), count: 6}))
				return 0
			}
		}

		cp.container.app.luaReturn = "go_messages"
		cp.container.commands = append(cp.container.commands, getMessages(cp.chats[cp.selectedChat].ID))
		return 0
	}))

	L.SetGlobal("current_chat_tbl", L.NewFunction(func(L *lua.LState) int {
		chat := cp.chats[cp.selectedChat]
		tableStr, err := struct_to_lua_table(chat)
		if err != nil {
			panic(err)
		}
		if err := L.DoString("return " + tableStr); err != nil {
			panic(err)
		}
		tbl := L.Get(-1) // retrieve the table
		L.Pop(1)         // remove it from the stack

		L.Push(tbl)

		return 1
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

	case whatsmeowEventMsg:
		cp.container.app.flashMsg = "MSG FROM " + msg.Chat.Name
		cp.container.app.flashCount = 6 // 3 flashes (on/off cycles)
		cp.container.commands = append(cp.container.commands, tea.Batch(getChats(), flashTick()))
		return cp, nil
	}

	return cp, nil
}

func (cp chats_page) View() string {
	chrome := struct {
		Width         int `json:"width"`
		Height        int `json:"height"`
		ChatCount     int `json:"chat_count"`
		SelectedIndex int `json:"selected_index"`
	}{
		Width:         cp.container.app.width,
		Height:        cp.container.app.height,
		ChatCount:     len(cp.chats),
		SelectedIndex: cp.selectedChat + 1,
	}

	header := "Chats:\n"
	if rendered, handled, err := renderLua(cp.container.app.luaState, "chat_header", chrome); err == nil && handled {
		header = strings.TrimSuffix(rendered, "\n")
	} else if err != nil {
		header = "Lua chat header error: " + err.Error()
	}
	footer := ""
	if rendered, handled, err := renderLua(cp.container.app.luaState, "chat_footer", chrome); err == nil && handled {
		footer = strings.TrimSuffix(rendered, "\n")
	} else if err != nil {
		footer = "Lua chat footer error: " + err.Error()
	}

	availableHeight := cp.container.app.height - blockHeight(header) - blockHeight(footer)
	if availableHeight < 1 {
		availableHeight = 1
	}

	lines := make([]string, 0, len(cp.chats))
	for i, chat := range cp.chats {
		if chat.Name == "" {
			chat.Name = chat.ID
		}
		rendered := cp.renderChat(chat, i)
		lines = append(lines, strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")...)
	}
	if len(lines) == 0 {
		lines = append(lines, "No synchronized chats")
	}
	cp.lines = lines

	startIdx := cp.scrollOffset
	endIdx := startIdx + availableHeight
	if endIdx > len(lines) {
		endIdx = len(lines)
		startIdx = max(0, endIdx-availableHeight)
	}
	if startIdx < 0 {
		startIdx = 0
	}
	var displayLines []string
	if startIdx < len(lines) {
		displayLines = lines[startIdx:endIdx]
	}
	for len(displayLines) < availableHeight {
		displayLines = append(displayLines, "")
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	for _, line := range displayLines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if footer != "" {
		b.WriteString(footer)
	}
	return b.String()
}

func (cp *chats_page) renderChat(chat Chat, idx int) string {
	L := cp.container.app.luaState
	type chat_to_render_info struct {
		Is_selected bool `json:"is_selected"`
		Index       int  `json:"index"`
		Width       int  `json:"width"`
		Height      int  `json:"height"`
	}

	type chat_to_render struct {
		Info chat_to_render_info `json:"info"`
		Chat Chat                `json:"chat"`
	}

	str, err := struct_to_lua_table(
		chat_to_render{
			chat_to_render_info{Is_selected: idx == cp.selectedChat, Index: idx + 1, Width: cp.container.app.width, Height: cp.container.app.height},
			chat,
		},
	)
	var renderedLine string
	var luaHandled bool
	if err != nil {
		panic(err.Error())
	}
	luaScript := fmt.Sprintf(`
			tbl = %v
			local f = renders["chat"]
			if type(f) == "function" then
				local ok, result = pcall(f, tbl)
				if ok and type(result) == "string" then
					_rendered = result
					_handled = true
				end
			end
		`, str)

	err = L.DoString(luaScript)
	if err != nil {
		panic("Lua error: " + err.Error() + "\n" + luaScript)
	}

	if L.GetGlobal("_handled") == lua.LTrue {
		renderedLine = L.GetGlobal("_rendered").String()
		luaHandled = true
	}

	// Clean up Lua globals
	L.SetGlobal("_rendered", lua.LNil)
	L.SetGlobal("_handled", lua.LBool(false))

	if luaHandled {
		return renderedLine
	}
	if idx == cp.selectedChat {
		return fmt.Sprintf("> %s\n", styles["selectedStyle"].Render(chat.Name))
	}
	return fmt.Sprintf("  %s\n", styles["unselectedStyle"].Render(chat.Name))
}

func forwardMsgToChat(chatID, messageID string) error {
	return whatsApp.forwardMessage(context.Background(), chatID, messageID)
}

func getChats() tea.Cmd {
	return func() tea.Msg {
		if whatsApp == nil {
			return fmt.Errorf("WhatsApp client is not initialized")
		}
		return chatsLoadedMsg(whatsApp.getChats())
	}
}
