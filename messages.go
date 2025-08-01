package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

type message struct {
	MsgID        string          `json:"id"`
	From         string          `json:"from"`
	Type         string          `json:"type"`
	GroupFrom    string          `json:"group_member_from"`
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
	inInput         bool         // true when buffer focused
	replyHighlights map[int]bool // tracks which messages have reply highlight
	replyingToMsg   int          // index of message being replied to (-1 if not replying)
	scrollOffset    int          // for scrolling through messages in select mode
	from_chat       *chat
	container       *pageContainer
}

func new_messages_page(chat chat, container *pageContainer) messages_page {
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
	var b strings.Builder

	var topbarText string
	if !mp.inInput {
		if mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
			// Check if the selected message has reply highlight
			if mp.replyHighlights[mp.selectedMsg] {
				msg := mp.messages[mp.selectedMsg]
				topbarText = fmt.Sprintf(" Replying to \"%s\" (ID: %s)", msg.Body, msg.MsgID)
			} else {
				msg := mp.messages[mp.selectedMsg]
				actionText := "R to reply"
				if msg.HasMedia {
					actionText += ", M to open media"
				}
				if msg.ResponseToID != "" {
					actionText += ", Enter to jump to quoted message"
				}
				topbarText = fmt.Sprintf(" Selected: %s (%s, Esc to return to input)", msg.MsgID, actionText)
			}
		} else {
			topbarText = " Selected: (Esc to return to input)"
		}
	} else {
		// Check if we're in reply mode
		if mp.replyingToMsg != -1 && mp.replyingToMsg < len(mp.messages) {
			msg := mp.messages[mp.replyingToMsg]
			topbarText = fmt.Sprintf(" Replying to \"%s\" (ID: %s, Esc to cancel reply)", msg.Body, msg.MsgID)
		} else {
			topbarText = " Messages (↑ ↓ to enter select mode, Enter to send, Esc to go back)"
		}
	}

	// shorten topbar text if it exceeds terminal container.app.width
	topbarTextLength := utf8.RuneCountInString(topbarText)
	if topbarTextLength > mp.container.app.width {
		if mp.container.app.width > 3 {
			topbarText = string([]rune(topbarText)[:mp.container.app.width-3]) + "..."
		} else {
			topbarText = string([]rune(topbarText)[:mp.container.app.width])
		}
	}

	topbarPadding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(topbarText)))
	topbar := styles["topbarStyle"].Width(mp.container.app.width).Render(topbarText + topbarPadding)
	b.WriteString(topbar + "\n")

	// Calculate available container.app.height for messages (total height - topbar - bottombar)
	availableHeight := mp.container.app.height - 2 // 1 for topbar, 1 for bottombar
	if availableHeight < 1 {
		availableHeight = 1
	}

	// Get all message lines
	messageLines := mp.calculateMessageLines()

	// Apply scrolling logic
	var displayLines []string
	if mp.inInput && mp.replyingToMsg == -1 {
		// In input mode and NOT replying, show the most recent messages
		if len(messageLines) > availableHeight {
			startIdx := len(messageLines) - availableHeight
			displayLines = messageLines[startIdx:]
		} else {
			displayLines = messageLines
		}
	} else {
		// In select mode OR in reply mode, use scroll offset
		startIdx := mp.scrollOffset
		endIdx := mp.scrollOffset + availableHeight

		// Ensure we don't scroll past the end
		if endIdx > len(messageLines) {
			endIdx = len(messageLines)
			startIdx = max(0, endIdx-availableHeight)
		}

		// Ensure we don't scroll before the beginning
		if startIdx < 0 {
			startIdx = 0
		}

		if startIdx < len(messageLines) {
			displayLines = messageLines[startIdx:endIdx]
		}
	}

	// Fill remaining space if needed
	for len(displayLines) < availableHeight {
		displayLines = append([]string{""}, displayLines...)
	}

	for _, line := range displayLines {
		b.WriteString(line + "\n")
	}

	// Fixed bottom input bar - full container.app.width
	var flashbar string
	if mp.container.app.flashCount > 0 {
		// Alternate between error and normal style for flashing effect
		msg := " " + mp.container.app.flashMsg + " : " + fmt.Sprintf("%v", mp.container.app.flashCount)
		bottombarPadding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(msg)))
		if mp.container.app.flashCount%2 == 1 {
			flashbar = styles["errorBarStyle"].Width(mp.container.app.width).Render(msg + bottombarPadding)
		} else {
			flashbar = styles["bottombarStyle"].Width(mp.container.app.width).Render(msg + bottombarPadding)
		}
	}
	if flashbar != "" {
		flashbar += "\n"
	}
	b.WriteString(flashbar)
	var bottombar string
	inputText := " Message: " + mp.input
	bottombarPadding := strings.Repeat(" ", max(0, mp.container.app.width-utf8.RuneCountInString(inputText)))
	bottombar = styles["bottombarStyle"].Width(mp.container.app.width).Render(inputText + bottombarPadding)
	b.WriteString(bottombar)
	return b.String()
}

func (mp messages_page) findMessageByID(msgID string) (message, int) {
	for i, msg := range mp.messages {
		if msg.MsgID == msgID {
			return msg, i
		}
	}
	return message{}, -1
}

func (mp messages_page) calculateMessageLines() []string {
	var messageLines []string
	for i, msg := range mp.messages {
		var linePrefix string

		ts := msg.Timestamp.Local().Format("15:04")

		var sender_id string
		if strings.Contains(mp.from_chat.ID, "@g.us") {
			sender_id = msg.GroupFrom
		} else {
			sender_id = msg.From
		}

		sender, ok := mp.container.app.id_to_name[sender_id]
		if !ok {
			if msg.FromMe {
				mp.container.app.id_to_name[sender_id] = "You"
				sender = "You"
			} else {
				sender = sender_id
			}
		}

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
		hasReplyHighlight := mp.replyHighlights[i] || (mp.replyingToMsg == i)
		selected := mp.selectedMsg == i && !mp.inInput

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

			messageLines = append(messageLines, completeLine)

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

				messageLines = append(messageLines, completeContLine)
			}
		}
	}
	return messageLines
}

func getMessages(chatId string) tea.Cmd {
	return func() tea.Msg {
		res, err := http.Get(fmt.Sprintf("%s/client/1/chat/%s/messages", baseURL, chatId))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		var msgs []message
		if err := json.NewDecoder(res.Body).Decode(&msgs); err != nil {
			msgs = append(msgs, message{Body: err.Error()})
		}
		return messagesLoadedMsg(msgs)
	}
}

func sendMessage(chatId, text string) tea.Cmd {
	return func() tea.Msg {
		data := map[string]string{"message": text}
		body, _ := json.Marshal(data)
		res, err := http.Post(
			fmt.Sprintf("%s/client/1/chat/%s/send", baseURL, chatId),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return getMessages(chatId)()
	}
}

func deleteMessage(chatId, msgId string) error {
	c := &http.Client{}
	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/client/1/message/%s", baseURL, msgId),
		nil,
	)

	if err != nil {
		return err
	}

	res, err := c.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

func sendReply(chatId, text, responseToId string) tea.Cmd {
	return func() tea.Msg {
		data := map[string]string{
			"message":        text,
			"response_to_id": responseToId,
		}
		body, _ := json.Marshal(data)
		res, err := http.Post(
			fmt.Sprintf("%s/client/1/chat/%s/send", baseURL, chatId),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return getMessages(chatId)()
	}
}

func sendMedia(chatId, mediaPath, caption, responseToId string) tea.Cmd {
	return func() tea.Msg {
		// Open file
		file, err := os.Open(mediaPath)
		if err != nil {
			return updateFlashMsg{msg: "File not found!", count: 6}
		}
		defer file.Close()

		// Read first 512 bytes to detect MIME type
		head := make([]byte, 512)
		n, _ := file.Read(head)
		mimeType := http.DetectContentType(head[:n])

		// Reset reader to beginning of file
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return error(err)
		}

		// Build multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add message
		_ = writer.WriteField("message", caption)

		// Create form part with detected Content-Type
		partHeader := textproto.MIMEHeader{}
		partHeader.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="media"; filename="%s"`, filepath.Base(mediaPath)))
		partHeader.Set("Content-Type", mimeType)

		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return error(err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return error(err)
		}

		// Add response_to_id
		_ = writer.WriteField("response_to_id", responseToId)

		// Close form
		if err := writer.Close(); err != nil {
			return error(err)
		}

		// Prepare and send request
		url := fmt.Sprintf("%s/client/1/chat/%s/send", baseURL, chatId)
		req, err := http.NewRequest("POST", url, body)
		if err != nil {
			return error(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return error(err)
		}
		defer res.Body.Close()

		if res.StatusCode >= 400 {
			return error(fmt.Errorf("server error: %d", res.StatusCode))
		}

		return getMessages(chatId)()
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

func (mp *messages_page)registerLuaFuncs() {
	L := mp.container.app.luaState

	L.SetGlobal("scroll_up", L.NewFunction(func(L *lua.LState) int {
		if mp.inInput {
			mp.inInput = false
			mp.selectedMsg = len(mp.messages) - 1
			mp.scrollOffset = len(mp.messages)-mp.container.app.height+4
		} else if mp.selectedMsg > 0 {
			mp.selectedMsg--
			if mp.selectedMsg < mp.scrollOffset {
				mp.scrollOffset = mp.selectedMsg
			}
		}

		return 0
	}))

	L.SetGlobal("scroll_down", L.NewFunction(func(L *lua.LState) int {
		if !mp.inInput {
			if mp.selectedMsg < len(mp.messages)-1 {
				mp.selectedMsg++
				avail := mp.container.app.height - 2
				if mp.selectedMsg >= mp.scrollOffset+avail {
					mp.scrollOffset = mp.selectedMsg - avail + 1
				}
			} else {
				mp.inInput = true
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
					height := mp.container.app.height - 2
					if height < 1 {
						height = 1
					}
					mp.scrollOffset = idx - height/2
					if mp.scrollOffset < 0 {
						mp.scrollOffset = 0
					}
					lines := mp.calculateMessageLines()
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
			mediaURL := fmt.Sprintf("%s/client/1/message/%s/media", baseURL, mp.messages[mp.selectedMsg].MsgID)
			openURL(mediaURL)
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

	L.SetGlobal("quit", L.NewFunction(func(L *lua.LState) int {
		mp.container.commands = append(mp.container.commands, tea.Quit)
		return 0
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
	case webhookMsg:
		if msg.Chat.ID != mp.from_chat.ID {
			mp.container.commands = append(mp.container.commands, flash(updateFlashMsg{msg: "MSG FROM " + msg.Chat.Name, count: 6}))
			return mp, nil
		}
		mp.container.commands = append(mp.container.commands, getMessages(msg.Chat.ID))
		return mp, nil
	}
	return mp, nil
}
