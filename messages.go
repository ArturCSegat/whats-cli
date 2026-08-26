package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
	"go.mau.fi/whatsmeow/proto/waE2E"

	tea "github.com/charmbracelet/bubbletea"
)

type message struct {
	MsgID        string          `json:"id"`
	ChatID       string          `json:"-"`
	From         string          `json:"from"`
	Type         string          `json:"type"`
	GroupFrom    string          `json:"groupMemberFrom"`
	GroupFromAlt string          `json:"groupMemberFromAlt,omitempty"`
	SenderName   string          `json:"groupMemberName,omitempty"`
	FromMe       bool            `json:"fromMe"`
	Body         string          `json:"body"`
	Timestamp    time.Time       `json:"timestamp"`
	HasMedia     bool            `json:"hasMedia"`
	GroupInvite  string          `json:"groupInvite"`
	IsResponse   bool            `json:"isQuote"`
	ResponseToID string          `json:"quoteId"`
	IsForwarded  bool            `json:"isForwarded"`
	MentionedIDs []string        `json:"mentionedIds"`
	Info         map[string]bool `json:"info"`
	RawMessage   *waE2E.Message  `json:"-"`
}

type messagesLoadedMsg []message
type flashTickMsg struct{}
type updateFlashMsg struct {
	count int
	msg   string
}
type messages_page struct {
	messages        []message
	selectedMsg     int
	input           string
	inInput         bool
	replyHighlights map[int]bool
	replyingToMsg   int
	scrollOffset    int
	from_chat       *Chat
	container       *pageContainer
	lines           []string
	curr_line       int
}

type messageChromeInfo struct {
	Chat         Chat   `json:"chat"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	MessageCount int    `json:"message_count"`
	Input        string `json:"input"`
	InInput      bool   `json:"in_input"`
	SelectedID   string `json:"selected_id"`
	Replying     bool   `json:"replying"`
	StatusText   string `json:"status_text"`
}

func new_messages_page(chat Chat, container *pageContainer) messages_page {
	if container == nil {
		panic("passed nil container")
	}

	mp := messages_page{}
	mp.inInput = true
	mp.scrollOffset = 0 // Reset scroll when switching chats
	mp.replyHighlights = make(map[int]bool)
	mp.replyingToMsg = -1
	mp.selectedMsg = -1
	mp.container = container
	mp.from_chat = &chat
	return mp
}

func (mp messages_page) Init() tea.Cmd {
	return nil
}

func (mp messages_page) View() string {
	var statusText string
	if !mp.inInput {
		if mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
			msg := mp.messages[mp.selectedMsg]
			if mp.replyHighlights[mp.selectedMsg] {
				statusText = fmt.Sprintf(" Replying to \"%s\" (ID: %s)", msg.Body, msg.MsgID)
			} else {
				actionText := "actions: reply"
				if msg.HasMedia {
					actionText += ", open media"
				}
				if msg.ResponseToID != "" {
					actionText += ", jump to quoted message"
				}
				statusText = fmt.Sprintf(" Selected: %s (%s, Esc to return to input)", msg.MsgID, actionText)
			}
		} else {
			statusText = " Selected: (Esc to return to input)"
		}
	} else if mp.replyingToMsg != -1 && mp.replyingToMsg < len(mp.messages) {
		msg := mp.messages[mp.replyingToMsg]
		statusText = fmt.Sprintf(" Replying to \"%s\" (ID: %s, Esc to cancel reply)", msg.Body, msg.MsgID)
	} else {
		statusText = " Messages "
	}

	selectedID := ""
	if mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
		selectedID = mp.messages[mp.selectedMsg].MsgID
	}
	chrome := messageChromeInfo{
		Chat:         *mp.from_chat,
		Width:        mp.container.app.width,
		Height:       mp.container.app.height,
		MessageCount: len(mp.messages),
		Input:        mp.input,
		InInput:      mp.inInput,
		SelectedID:   selectedID,
		Replying:     mp.replyingToMsg >= 0,
		StatusText:   statusText,
	}

	status := ""
	if rendered, handled, err := renderLua(mp.container.app.luaState, "status", chrome); err != nil {
		statusText = " " + err.Error()
	} else if handled {
		status = strings.TrimSuffix(rendered, "\n")
	}
	if status == "" {
		statusRunes := []rune(statusText)
		if len(statusRunes) > mp.container.app.width {
			if mp.container.app.width > 3 {
				statusText = string(statusRunes[:mp.container.app.width-3]) + "..."
			} else {
				statusText = string(statusRunes[:mp.container.app.width])
			}
		}
		padding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(statusText)))
		status = styles["topbarStyle"].Width(mp.container.app.width).Render(statusText + padding)
	}

	inputBar := ""
	if rendered, handled, err := renderLua(mp.container.app.luaState, "input", chrome); err != nil {
		inputBar = " " + err.Error()
	} else if handled {
		inputBar = strings.TrimSuffix(rendered, "\n")
	}
	if inputBar == "" {
		inputText := " Message: " + mp.input
		padding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(inputText)))
		inputBar = styles["bottombarStyle"].Width(mp.container.app.width).Render(inputText + padding)
	}

	flashbar := ""
	if mp.container.app.flashCount > 0 {
		text := fmt.Sprintf(" %s : %d", mp.container.app.flashMsg, mp.container.app.flashCount)
		padding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(text)))
		if mp.container.app.flashCount%2 == 1 {
			flashbar = styles["errorBarStyle"].Width(mp.container.app.width).Render(text + padding)
		} else {
			flashbar = styles["bottombarStyle"].Width(mp.container.app.width).Render(text + padding)
		}
	}

	availableHeight := mp.container.app.height - blockHeight(status) - blockHeight(inputBar) - blockHeight(flashbar)
	if availableHeight < 1 {
		availableHeight = 1
	}
	mp.calculateMessageLines()
	messageLines := mp.lines

	var displayLines []string
	if mp.inInput && mp.replyingToMsg == -1 {
		if len(messageLines) > availableHeight {
			displayLines = messageLines[len(messageLines)-availableHeight:]
		} else {
			displayLines = messageLines
		}
	} else {
		startIdx := mp.scrollOffset
		endIdx := startIdx + availableHeight
		if endIdx > len(messageLines) {
			endIdx = len(messageLines)
			startIdx = max(0, endIdx-availableHeight)
		}
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx < len(messageLines) {
			displayLines = messageLines[startIdx:endIdx]
		}
	}
	for len(displayLines) < availableHeight {
		displayLines = append([]string{""}, displayLines...)
	}

	var b strings.Builder
	statusAtBottom := lua.LVAsBool(mp.container.app.luaState.GetGlobal("status_at_bottom"))
	if !statusAtBottom {
		b.WriteString(status)
		b.WriteByte('\n')
	}
	for _, line := range displayLines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if flashbar != "" {
		b.WriteString(flashbar)
		b.WriteByte('\n')
	}
	if statusAtBottom {
		b.WriteString(status)
		b.WriteByte('\n')
	}
	b.WriteString(inputBar)
	return b.String()
}

func blockHeight(block string) int {
	if block == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

func (mp messages_page) viewportHeight() int {
	statusLines := 1
	if value := mp.container.app.luaState.GetGlobal("status_lines"); value.Type() == lua.LTNumber {
		statusLines = max(1, int(lua.LVAsNumber(value)))
	}
	return max(1, mp.container.app.height-statusLines-1)
}

func (mp messages_page) findMessageByID(msgID string) (message, int) {
	for i, msg := range mp.messages {
		if msg.MsgID == msgID {
			return msg, i
		}
	}
	return message{}, -1
}

func (mp *messages_page) calculateMessageLines() {
	var messageLines []string

	for i, msg := range mp.messages {

		var senderID string
		isGroup := strings.Contains(mp.from_chat.ID, "@g.us")
		if isGroup {
			senderID = msg.GroupFrom
		} else {
			senderID = msg.From
		}

		sender, ok := mp.container.app.id_to_name[senderID]
		if isGroup && whatsApp != nil {
			if contactName, found := whatsApp.contactNameForIDs(msg.GroupFrom, msg.GroupFromAlt); found {
				sender = contactName
				ok = true
			}
		}
		if !ok && msg.SenderName != "" {
			sender = msg.SenderName
			ok = true
		}
		if !ok {
			sender = senderID
		}
		if msg.FromMe {
			sender = "You"
		}

		renderedLine, _ := mp.renderMsg(msg, i, sender)
		lines := strings.Split(renderedLine, "\n")
		messageLines = append(messageLines, lines...)
	}

	mp.lines = messageLines
}

func getMessages(chatID string) tea.Cmd {
	return func() tea.Msg {
		if whatsApp == nil {
			return errors.New("WhatsApp client is not initialized")
		}
		return messagesLoadedMsg(whatsApp.getMessages(chatID))
	}
}

func sendMessage(chatID, text string) tea.Cmd {
	return func() tea.Msg {
		if err := whatsApp.sendText(context.Background(), chatID, text, ""); err != nil {
			return err
		}
		return getMessages(chatID)()
	}
}

func deleteMessage(chatID, messageID string) error {
	return whatsApp.deleteMessage(context.Background(), chatID, messageID)
}

func sendReply(chatID, text, responseToID string) tea.Cmd {
	return func() tea.Msg {
		if err := whatsApp.sendText(context.Background(), chatID, text, responseToID); err != nil {
			return err
		}
		return getMessages(chatID)()
	}
}

func sendMedia(chatID, mediaPath, caption, responseToID string) tea.Cmd {
	return func() tea.Msg {
		if _, err := os.Stat(mediaPath); err != nil {
			return updateFlashMsg{msg: "File not found!", count: 6}
		}
		if err := whatsApp.sendMedia(context.Background(), chatID, mediaPath, caption, responseToID); err != nil {
			return err
		}
		return getMessages(chatID)()
	}
}
func flashTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return flashTickMsg{}
	})
}

func flash(msg updateFlashMsg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func (msg *message) getMediaPrefix() string {
	if msg.Type == "revoked" {
		msg.HasMedia = true
		return "[DELETED]"
	} else if msg.Type == "ciphertext" {
		msg.HasMedia = true
		return "[VIS ONCE MEDIA]"
	} else if msg.HasMedia {
		return "[MEDIA]"
	}
	return ""
}
func (mp messages_page) getOfsset(idx int) int {
	offset := 0
	for i := 0; i < idx; i++ {
		str, _ := mp.renderMsg(mp.messages[i], i, "")
		offset += len(strings.Split(str, "\n"))
	}
	return offset
}

func (mp *messages_page) registerLuaFuncs() {
	L := mp.container.app.luaState

	L.SetGlobal("scroll_up", L.NewFunction(func(L *lua.LState) int {
		mp.calculateMessageLines()
		if mp.inInput {
			mp.inInput = false
			mp.selectedMsg = len(mp.messages) - 1
			mp.scrollOffset = len(mp.lines) - mp.viewportHeight()
			mp.curr_line = len(mp.lines) - 1
		} else if mp.selectedMsg > 0 {
			str, _ := mp.renderMsg(mp.messages[mp.selectedMsg], mp.selectedMsg, "sender")
			mp.curr_line -= (len(strings.Split(str, "\n")))
			mp.selectedMsg--
			if mp.curr_line < mp.scrollOffset {
				mp.scrollOffset = mp.curr_line - mp.viewportHeight()
			}
		}
		return 0
	}))

	L.SetGlobal("scroll_down", L.NewFunction(func(L *lua.LState) int {
		mp.calculateMessageLines()
		if !mp.inInput {
			if mp.selectedMsg < len(mp.messages)-1 {
				str, _ := mp.renderMsg(mp.messages[mp.selectedMsg], mp.selectedMsg, "sender")
				mp.curr_line += len(strings.Split(str, "\n"))
				mp.selectedMsg++
				if mp.curr_line >= mp.scrollOffset+mp.viewportHeight() {
					mp.scrollOffset = mp.curr_line - mp.viewportHeight() + 1
				}
			} else {
				mp.inInput = true
				mp.selectedMsg = -1
				if mp.replyingToMsg == -1 {
					mp.scrollOffset = 0
				}
			}
		}
		return 0
	}))

	L.SetGlobal("escape", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput || mp.replyingToMsg != -1 {
			mp.replyingToMsg = -1
			mp.replyHighlights = make(map[int]bool)
			mp.selectedMsg = -1
			mp.inInput = true
			return 0
		}

		mp.container.app.luaReturn = "go_chats"
		return 0
	}))

	L.SetGlobal("jump_to_quoted", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput && mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
			selected := mp.messages[mp.selectedMsg]
			if selected.ResponseToID != "" {
				_, idx := mp.findMessageByID(selected.ResponseToID)
				if idx != -1 {
					mp.selectedMsg = idx
					height := mp.viewportHeight()
					mp.scrollOffset = idx - height/2
					if mp.scrollOffset < 0 {
						mp.scrollOffset = 0
					}
					mp.calculateMessageLines()
					lines := mp.lines
					maxScroll := len(lines) - height
					if mp.scrollOffset > maxScroll {
						mp.scrollOffset = maxScroll
					}
					if mp.scrollOffset < 0 {
						mp.scrollOffset = 0
					}
				}
			}
		}
		return 0
	}))

	L.SetGlobal("toggle_reply", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput && mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
			if mp.replyHighlights[mp.selectedMsg] {
				mp.replyHighlights[mp.selectedMsg] = false
				mp.replyingToMsg = -1
			} else {
				mp.replyHighlights = make(map[int]bool)
				mp.replyHighlights[mp.selectedMsg] = true
				mp.replyingToMsg = mp.selectedMsg
				mp.inInput = true
			}
		} else {
			mp.container.app.luaReturn = "type"
		}
		return 0
	}))

	L.SetGlobal("open_media", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput && mp.selectedMsg >= 0 && mp.messages[mp.selectedMsg].HasMedia {
			messageID := mp.messages[mp.selectedMsg].MsgID
			mp.container.commands = append(mp.container.commands, func() tea.Msg {
				mediaPath, err := whatsApp.downloadMedia(context.Background(), messageID)
				if err != nil {
					return updateFlashMsg{msg: "Media: " + err.Error(), count: 6}
				}
				openURL(mediaPath)
				return nil
			})
		} else {
			mp.container.app.luaReturn = "type"
		}
		return 0
	}))

	L.SetGlobal("forward_selected", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput && mp.selectedMsg >= 0 {
			cp := new_chats_page(mp.container)
			cp.forwarding.isForwarding = true
			cp.forwarding.MsgID = mp.messages[mp.selectedMsg].MsgID
			mp.container.page = cp
			mp.container.commands = append(mp.container.commands, getChats())
		} else {
			mp.container.app.luaReturn = "type"
		}

		return 0
	}))
	L.SetGlobal("delete_selected", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput && mp.selectedMsg >= 0 {
			err := deleteMessage(mp.from_chat.ID, mp.messages[mp.selectedMsg].MsgID)
			if err != nil {
				mp.container.app.flashMsg = "ERROR when deleting msg"
				mp.container.app.flashCount = 6
			} else {
				time.Sleep(3 * time.Second)
				mp.container.commands = append(mp.container.commands, getMessages(mp.from_chat.ID))
			}
		} else {
			mp.container.app.luaReturn = "type"
		}

		return 0
	}))

	L.SetGlobal("append_input", L.NewFunction(func(L *lua.LState) int {
		str := L.ToString(1)
		mp.input += str
		return 0
	}))
	L.SetGlobal("backspace_input", L.NewFunction(func(L *lua.LState) int {
		if len(mp.input) > 0 {
			mp.input = mp.input[:len(mp.input)-1]
		}
		return 0
	}))

	L.SetGlobal("submit_input", L.NewFunction(func(L *lua.LState) int {
		input := strings.TrimSpace(mp.input)
		if input == "" {
			return 0
		}

		var cmd tea.Cmd

		// Clear input field
		mp.input = ""

		// Handle media syntax
		if strings.HasPrefix(input, `media:"`) {
			parts := strings.SplitN(input[len(`media:"`):], `"`, 2)
			if len(parts) < 1 {
				return 0
			}
			mediaPath := parts[0]
			caption := ""
			if len(parts) == 2 {
				caption = strings.TrimSpace(parts[1])
			}

			var replyToID string
			if mp.replyingToMsg != -1 {
				replyToID = mp.messages[mp.replyingToMsg].MsgID
				mp.replyHighlights = make(map[int]bool)
				mp.replyingToMsg = -1
			}

			mp.scrollOffset = 0

			cmd = sendMedia(mp.from_chat.ID, mediaPath, caption, replyToID)
			mp.container.commands = append(mp.container.commands, cmd)
			return 0
		}

		// Handle clipboard media
		if strings.HasPrefix(input, "media:clipboard") {
			caption := ""
			parts := strings.SplitN(input, " ", 2)
			if len(parts) > 1 {
				caption = strings.TrimSpace(parts[1])
			}

			var replyToID string
			if mp.replyingToMsg != -1 {
				replyToID = mp.messages[mp.replyingToMsg].MsgID
				mp.replyHighlights = make(map[int]bool)
				mp.replyingToMsg = -1
			}
			mp.scrollOffset = 0

			cmd = func() tea.Msg {
				mediaPath, err := getClipboardMediaFile()
				if err != nil {
					return updateFlashMsg{msg: "Clipboard: " + err.Error(), count: 6}
				}
				defer os.Remove(mediaPath)
				return sendMedia(mp.from_chat.ID, mediaPath, caption, replyToID)()
			}
			mp.container.commands = append(mp.container.commands, cmd)
			return 0
		}

		// Handle reply or plain message
		if mp.replyingToMsg != -1 && mp.replyingToMsg < len(mp.messages) {
			replyToID := mp.messages[mp.replyingToMsg].MsgID
			mp.replyHighlights = make(map[int]bool)
			mp.replyingToMsg = -1
			mp.scrollOffset = 0

			cmd = sendReply(mp.from_chat.ID, input, replyToID)
		} else {
			cmd = sendMessage(mp.from_chat.ID, input)
			mp.scrollOffset = 0
		}

		mp.container.commands = append(mp.container.commands, cmd)
		return 0
	}))

	L.SetGlobal("in_input", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(mp.inInput))
		return 1
	}))

	L.SetGlobal("input_content", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(mp.input))
		return 1
	}))

	L.SetGlobal("quit", L.NewFunction(func(L *lua.LState) int {
		mp.container.commands = append(mp.container.commands, tea.Quit)
		return 0
	}))

	L.SetGlobal("current_message_tbl", L.NewFunction(func(L *lua.LState) int {
		msg := mp.messages[mp.selectedMsg]
		tableStr, err := struct_to_lua_table(msg)
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

func (mp messages_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mp.registerLuaFuncs()
	mp.container.app.luaReturn = "" // Reset Lua return value
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Look for Lua keybind
		L := mp.container.app.luaState
		luaKeyHandled := false

		// Try to run keybinds[key]()
		err := L.DoString(fmt.Sprintf(`
			local key = %q
			local f = message_keybinds[key]
			if type(f) == "function" then
				f()
				handled = true
			end
		`, key))
		if err != nil {
			fmt.Println("Lua error:", err)
		}

		luaKeyHandled = L.GetGlobal("handled") == lua.LTrue
		L.SetGlobal("handled", lua.LBool(false)) // reset
		if !luaKeyHandled {
			mp.container.app.luaReturn = "type" // No Lua keybind handled, return to input mode
		}

		if mp.container.app.luaReturn != "" {
			switch mp.container.app.luaReturn {
			case "type":
				mp.input += key

			case "go_chats":
				cp := new_chats_page(mp.container)
				mp.container.commands = append(mp.container.commands, getChats())
				return cp, nil
			}
		}

		return mp, nil

	case messagesLoadedMsg:
		mp.container.app.luaState.OpenLibs()
		mp.messages = msg
		if strings.Contains(mp.from_chat.ID, "@g.us") {
			// Group chat
			chatTitle := mp.from_chat.Name
			if chatTitle == "" {
				chatTitle = mp.from_chat.ID
			}
			setTerminalTitle("Whats-CLI: " + chatTitle)
		} else if len(msg) > 0 {
			// Private chat — get the 'From' field of first non-self message
			for _, message := range msg {
				if !message.FromMe {
					displayName := mp.from_chat.Name
					if displayName == "" {
						displayName = mp.from_chat.ID
					}
					setTerminalTitle(fmt.Sprintf("Whats-CLI: %s (%s)", displayName, message.From))
					break
				}
			}
		}
	case whatsmeowEventMsg:
		L := mp.container.app.luaState

		str, err := struct_to_lua_table(msg.Message)
		if err != nil {
			panic(err.Error())
		}
		lua_str := fmt.Sprintf(`
			if not hooks then
				handled = false
				return
			end

			local key = %q
			local f = hooks[key]
			tbl = %v

			if type(f) == "function" then
				f(tbl)
				handled = true
			end
		`, "onMsg", str)

		err = mp.container.app.luaState.DoString(lua_str)
		if err != nil {
			panic("Lua error:" + err.Error() + "\n" + lua_str)
		}

		L.SetGlobal("handled", lua.LBool(false)) // reset

		if msg.Chat.ID != mp.from_chat.ID {
			mp.container.commands = append(mp.container.commands, flash(updateFlashMsg{msg: "MSG FROM " + msg.Chat.Name, count: 6}))
			return mp, nil
		}
		mp.container.commands = append(mp.container.commands, getMessages(msg.Chat.ID))
		return mp, nil
	}
	return mp, nil
}

func (mp messages_page) renderMsg(msg message, idx int, sender string) (string, bool) {
	L := mp.container.app.luaState

	type message_to_render_info struct {
		Is_selected bool   `json:"is_selected"`
		IsGroup     bool   `json:"is_group"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Name        string `json:"name"`
		ChatName    string `json:"chat_name"`
	}
	type message_to_render struct {
		Msg  message                `json:"message"`
		Info message_to_render_info `json:"info"`
	}
	luaHandled := false
	renderedLine := ""

	str, err := struct_to_lua_table(
		message_to_render{
			Msg: msg,
			Info: message_to_render_info{
				Is_selected: idx == mp.selectedMsg,
				IsGroup:     mp.from_chat.IsGroup,
				Width:       mp.container.app.width,
				Height:      mp.container.app.height,
				Name:        sender,
				ChatName:    mp.from_chat.Name,
			},
		})
	if err != nil {
		panic(err.Error())
	}

	luaScript := fmt.Sprintf(`
			tbl = %v
			local f = renders["message"]
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
		return renderedLine, true
	}

	var ret string
	var linePrefix string

	ts := msg.Timestamp.Local().Format("15:04")

	body := msg.Body
	msgPrefix := "[" + ts + "] <" + sender + ">: "

	// Calculate the full prefix length (line prefix + message prefix)
	fullPrefixLength := utf8.RuneCountInString(linePrefix + msgPrefix)

	// Handle reply indicator
	var replyPrefix string
	if msg.ResponseToID != "" {
		// Find the original message body (now includes media indicator)
		originalMsg, _ := mp.findMessageByID(msg.ResponseToID)
		originalBody := originalMsg.Body
		if originalMsg.HasMedia {
			originalBody = originalMsg.getMediaPrefix() + originalBody
		}
		// Truncate original body if too long (adjust as needed)
		if len(originalBody) > 30 {
			originalBody = originalBody[:27] + "..."
		}
		replyPrefix = fmt.Sprintf("[REPLY: %s] ", originalBody)
	}

	// Handle media indicator
	mediaPrefix := msg.getMediaPrefix()

	// Combine all prefixes with the body
	combinedPrefix := replyPrefix + mediaPrefix
	fullBody := combinedPrefix + body

	// Calculate available container.app.width for message content
	availableWidth := mp.container.app.width - fullPrefixLength

	// Wrap the message body BEFORE applying styling
	wrappedLines := wrapText(fullBody, availableWidth)

	// Check if this message has reply highlight or is being replied to
	hasReplyHighlight := mp.replyHighlights[idx] || (mp.replyingToMsg == idx)
	selected := mp.selectedMsg == idx && !mp.inInput

	// Apply styling to msgPrefix if it's from me (unless reply highlighted)
	styledMsgPrefix := msgPrefix
	if !hasReplyHighlight && !selected && msg.FromMe {
		styledMsgPrefix = styles["selfPrefix"].Render(msgPrefix)
	}

	if len(wrappedLines) > 0 {
		firstLine := wrappedLines[0]

		// Apply styling to the first line content (unless reply highlighted)
		if !hasReplyHighlight && !selected {
			// Handle reply indicator styling
			if strings.HasPrefix(firstLine, "[REPLY:") {
				// Find the end of the reply indicator by looking for the last "] "
				replyEndIdx := strings.LastIndex(firstLine, "] ")
				if replyEndIdx != -1 {
					replyIndicatorText := firstLine[:replyEndIdx+1] // Include "]" but not the space
					rest := firstLine[replyEndIdx+2:]               // Skip "] "

					// Style the reply indicator without the trailing space
					styledReplyIndicator := styles["replyHighlight"].Render(replyIndicatorText) + " "

					// Handle media in the reply indicator
					if msg.HasMedia {
						// Split the reply indicator to style media separately
						parts := strings.Split(replyIndicatorText, mediaPrefix)
						var styledParts []string
						for i, part := range parts {
							if i > 0 {
								styledParts = append(styledParts, styles["hyperlink"].Render(mediaPrefix))
							}
							styledParts = append(styledParts, styles["replyHighlight"].Render(part))
						}
						styledReplyIndicator = strings.Join(styledParts, "") + " "
					}

					// Handle media and self styling for the rest
					if strings.HasPrefix(rest, mediaPrefix) {
						mediaLabel := mediaPrefix
						afterMedia := strings.TrimPrefix(rest, mediaLabel)
						if msg.FromMe {
							firstLine = styledReplyIndicator + styles["hyperlink"].Render(mediaLabel) + styles["selfBody"].Render(afterMedia)
						} else {
							firstLine = styledReplyIndicator + styles["hyperlink"].Render(mediaLabel) + afterMedia
						}
					} else if msg.FromMe {
						firstLine = styledReplyIndicator + styles["selfBody"].Render(rest)
					} else {
						firstLine = styledReplyIndicator + rest
					}
				}
			} else if strings.HasPrefix(firstLine, mediaPrefix) {
				// Handle media without reply
				mediaLabel := mediaPrefix
				rest := strings.TrimPrefix(firstLine, mediaLabel)
				if msg.FromMe {
					firstLine = styles["hyperlink"].Render(mediaLabel) + styles["selfBody"].Render(rest)
				} else {
					firstLine = styles["hyperlink"].Render(mediaLabel) + rest
				}
			} else if msg.FromMe {
				// For self messages without media or reply, apply self styling
				firstLine = styles["selfBody"].Render(firstLine)
			}
		}

		// Build the complete first line
		var fowardedPrefix string
		if msg.IsForwarded {
			fowardedPrefix = "[FORWARDED] "
		}
		if msg.IsForwarded && !hasReplyHighlight && !selected {
			fowardedPrefix = styles["replyHighlight"].Render(fowardedPrefix)
		}
		completeLine := fmt.Sprintf("%s%s%s%s", linePrefix, styledMsgPrefix, fowardedPrefix, firstLine)

		// Apply reply highlight to the entire line if needed
		if hasReplyHighlight || selected {
			// Pad the line to full container.app.width and apply highlight
			padding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(completeLine)))
			completeLine = styles["replyHighlight"].Width(mp.container.app.width).Render(completeLine + padding)
		}

		ret += completeLine

		// Print continuation lines with padding and consistent styling
		padding := strings.Repeat(" ", fullPrefixLength)
		for j := 1; j < len(wrappedLines); j++ {
			continuationLine := wrappedLines[j]

			// Apply consistent styling to continuation lines (unless reply highlighted)
			if !hasReplyHighlight && msg.FromMe {
				continuationLine = styles["selfBody"].Render(continuationLine)
			}

			// Build complete continuation line
			completeContLine := fmt.Sprintf("%s%s", padding, continuationLine)

			// Apply reply highlight to continuation lines if needed
			if hasReplyHighlight || selected {
				linePadding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(completeContLine)))
				completeContLine = styles["replyHighlight"].Width(mp.container.app.width).Render(completeContLine + linePadding)
			}

			ret += "\n" + completeLine
		}
	}
	return ret, false
}
