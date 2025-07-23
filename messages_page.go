package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

type message struct {
	MsgID        string    `json:"id"`
	From         string    `json:"from"`
	FromMe       bool      `json:"fromMe"`
	HasMedia     bool      `json:"hasMedia"`
	GroupFrom    string    `json:"group_member_from"`
	Body         string    `json:"body"`
	Type         string    `json:"type"`
	Timestamp    time.Time `json:"timestamp"`
	ResponseToID string    `json:"quoteId"`
}

type messagesLoadedMsg []message
type messsageFlashMsg string
type flashTickMsg struct{}
type messages_page struct {
	messages        []message
	selectedMsg     int
	input           string
	inInput         bool         // true when buffer focused
	replyHighlights map[int]bool // tracks which messages have reply highlight
	replyingToMsg   int          // index of message being replied to (-1 if not replying)
	scrollOffset    int          // for scrolling through messages in select mode
	from_chat       *chat
	from_app        *app
}

func new_messages_page(chat chat, app *app) messages_page {
	mp := messages_page{}
	mp.inInput = true
	mp.scrollOffset = 0 // Reset scroll when switching chats
	mp.replyHighlights = make(map[int]bool)
	mp.replyingToMsg = -1
	mp.selectedMsg = -1
	mp.from_app = app
	mp.from_chat = &chat
	return mp
}

func (mp messages_page) Init() tea.Cmd {
	return nil
}

func (mp messages_page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if mp.from_app.flashCount > 0 {
			mp.from_app.flashCount = 0
			mp.from_app.flashMsg = ""
		}
		key := msg.String()
		switch key {
		case "ctrl+c":
			return mp, tea.Quit
		case "esc":
			if !mp.inInput {
				// If in selection mode, return to input mode and reset scroll to latest messages
				mp.inInput = true
				mp.selectedMsg = -1
				mp.scrollOffset = 0
				return mp, nil
			}
			cp := new_chats_page(mp.from_app)
			return cp, getChats()
		case "up":
			if mp.inInput {
				// Entering select mode
				mp.inInput = false

				// If we're in reply mode, return to the message selected for reply
				if mp.replyingToMsg != -1 {
					mp.selectedMsg = mp.replyingToMsg
					// Keep the current scroll offset when returning to reply message
				} else {
					// Not in reply mode - start from the last message
					mp.selectedMsg = len(mp.messages) - 1

					// Calculate available height and set scroll offset to show bottom messages
					availableHeight := mp.from_app.height - 2 // 1 for topbar, 1 for bottombar
					if availableHeight < 1 {
						availableHeight = 1
					}

					// Get all message lines to calculate proper scroll offset
					messageLines := mp.calculateMessageLines()

					// Set scroll offset to show the last messages (like input mode does)
					if len(messageLines) > availableHeight {
						mp.scrollOffset = len(messageLines) - availableHeight
					} else {
						mp.scrollOffset = 0
					}
				}
			} else if mp.selectedMsg > 0 {
				mp.selectedMsg--
				// Adjust scroll offset if needed to keep selected message visible
				if mp.selectedMsg < mp.scrollOffset {
					mp.scrollOffset = mp.selectedMsg
				}
			}
			return mp, nil
		case "down":
			if !mp.inInput {
				if mp.selectedMsg < len(mp.messages)-1 {
					mp.selectedMsg++
					// Adjust scroll offset if message goes off screen
					availableHeight := mp.from_app.height - 2
					if mp.selectedMsg >= mp.scrollOffset+availableHeight {
						mp.scrollOffset = mp.selectedMsg - availableHeight + 1
					}
				} else {
					// Return to input mode
					mp.inInput = true
					// Only reset scroll if we're NOT in reply mode
					if mp.replyingToMsg == -1 {
						mp.scrollOffset = 0
					}
				}
			}
		case "enter":
			if !mp.inInput {
				// In select mode - check if selected message has a quote
				if mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
					selectedMessage := mp.messages[mp.selectedMsg]
					if selectedMessage.ResponseToID != "" {
						// Find the quoted message index
						_, quotedMsgIndex := mp.findMessageByID(selectedMessage.ResponseToID)
						if quotedMsgIndex != -1 {
							// Jump to the quoted message
							mp.selectedMsg = quotedMsgIndex

							// Adjust scroll offset to show the quoted message
							availableHeight := mp.from_app.height - 2
							if availableHeight < 1 {
								availableHeight = 1
							}

							// Center the quoted message in the view if possible
							mp.scrollOffset = quotedMsgIndex - availableHeight/2
							if mp.scrollOffset < 0 {
								mp.scrollOffset = 0
							}

							// Make sure we don't scroll past the end
							messageLines := mp.calculateMessageLines()
							maxScroll := len(messageLines) - availableHeight
							if mp.scrollOffset > maxScroll {
								mp.scrollOffset = maxScroll
							}
							if mp.scrollOffset < 0 {
								mp.scrollOffset = 0
							}
						}
					}
				}
			} else {
				// Check for media upload syntax
				if strings.HasPrefix(mp.input, "media:\"") {
					parts := strings.SplitN(mp.input[len("media:\""):], "\"", 2)
					if len(parts) < 2 {
						return mp, nil
					}

					mediaPath := parts[0]
					caption := ""
					if len(parts) > 1 {
						caption = strings.TrimSpace(parts[1])
					}

					// Clear input buffer immediately
					mp.input = ""

					// Capture reply ID if we're in reply mode
					var replyToID string
					if mp.replyingToMsg != -1 {
						replyToID = mp.messages[mp.replyingToMsg].MsgID
						// Clear reply state
						mp.replyHighlights = make(map[int]bool)
						mp.replyingToMsg = -1
					}

					// Reset scroll to show latest messages
					mp.scrollOffset = 0

					return mp, sendMedia(
						mp.from_chat.ID,
						mediaPath,
						caption,
						replyToID,
					)
				}

				// --- Clipboard media support ---
				if strings.HasPrefix(mp.input, "media:clipboard") {
					// Optionally allow caption after a space
					caption := ""
					parts := strings.SplitN(mp.input, " ", 2)
					if len(parts) > 1 {
						caption = strings.TrimSpace(parts[1])
					}
					mp.input = ""

					// Capture reply ID if we're in reply mode
					var replyToID string
					if mp.replyingToMsg != -1 {
						replyToID = mp.messages[mp.replyingToMsg].MsgID
						mp.replyHighlights = make(map[int]bool)
						mp.replyingToMsg = -1
					}
					mp.scrollOffset = 0

					return mp, func() tea.Msg {
						mediaPath, err := getClipboardMediaFile()
						if err != nil {
							return messsageFlashMsg("Clipboard: " + err.Error())
						}
						defer os.Remove(mediaPath)
						return sendMedia(
							mp.from_chat.ID,
							mediaPath,
							caption,
							replyToID,
						)()
					}
				}

				// Check if we're replying to a message
				if mp.replyingToMsg != -1 && mp.replyingToMsg < len(mp.messages) {
					cmd := sendReply(mp.from_chat.ID, mp.input, mp.messages[mp.replyingToMsg].MsgID)
					mp.input = ""
					mp.replyHighlights = make(map[int]bool)
					mp.replyingToMsg = -1 // Clear reply state after sending
					mp.scrollOffset = 0   // Reset scroll to show latest messages
					return mp, cmd
				} else {
					cmd := sendMessage(mp.from_chat.ID, mp.input)
					mp.input = ""
					mp.scrollOffset = 0 // Reset scroll to show latest messages
					return mp, cmd
				}
			}
		case "r", "R":
			if !mp.inInput && mp.selectedMsg >= 0 && mp.selectedMsg < len(mp.messages) {
				// Toggle reply highlight for the selected message
				if mp.replyHighlights[mp.selectedMsg] {
					// If already highlighted, remove highlight and clear reply state
					mp.replyHighlights[mp.selectedMsg] = false
					mp.replyingToMsg = -1
				} else {
					// Clear any existing highlights and set new one
					mp.replyHighlights = make(map[int]bool)
					mp.replyHighlights[mp.selectedMsg] = true
					mp.replyingToMsg = mp.selectedMsg
					// Shift focus to input buffer but keep current scroll offset
					mp.inInput = true
					// DON'T reset scroll offset here - keep the current position
				}
			} else if mp.inInput {
				// allow typing 'r' in buffer
				mp.input += key
			}
		case "m", "M":
			if !mp.inInput {
				if mp.messages[mp.selectedMsg].HasMedia {
					mediaURL := fmt.Sprintf("%s/client/1/message/%s/media", baseURL, mp.messages[mp.selectedMsg].MsgID)
					openURL(mediaURL)
				}
			} else {
				// allow typing 'm' in buffer
				mp.input += key
			}
		default:
			if mp.inInput {
				switch msg.Type {
				case tea.KeyRunes:
					mp.input += msg.String()
				case tea.KeySpace:
					mp.input += " "
				case tea.KeyBackspace:
					if len(mp.input) > 0 {
						mp.input = mp.input[:len(mp.input)-1]
					}
				}
			}
		}
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
	case messsageFlashMsg:
		mp.from_app.flashMsg = string(msg)
		mp.from_app.flashCount = 6 // 3 flashes (on/off cycles)
		return mp, flashTick()
	case webhookMsg:
		if msg.Chat.ID != mp.from_chat.ID {
			mp.from_app.flashMsg = "MSG FROM " + msg.Chat.Name
			mp.from_app.flashCount = 6 // 3 flashes (on/off cycles)
			return mp, flashTick()
		}
		return mp, getMessages(msg.Chat.ID)
	}
	return mp, nil
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

	// shorten topbar text if it exceeds terminal from_app.width
	topbarTextLength := utf8.RuneCountInString(topbarText)
	if topbarTextLength > mp.from_app.width {
		if mp.from_app.width > 3 {
			topbarText = string([]rune(topbarText)[:mp.from_app.width-3]) + "..."
		} else {
			topbarText = string([]rune(topbarText)[:mp.from_app.width])
		}
	}

	topbarPadding := strings.Repeat(" ", max(0, mp.from_app.width-utf8.RuneCountInString(topbarText)))
	topbar := topbarStyle.Width(mp.from_app.width).Render(topbarText + topbarPadding)
	b.WriteString(topbar + "\n")

	// Calculate available from_app.height for messages (total height - topbar - bottombar)
	availableHeight := mp.from_app.height - 2 // 1 for topbar, 1 for bottombar
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

	// Fixed bottom input bar - full from_app.width
	var bottombar string
	if mp.from_app.flashCount > 0 {
		// Alternate between error and normal style for flashing effect
		if mp.from_app.flashCount%2 == 1 {
			bottombar = errorBarStyle.Width(mp.from_app.width).Render(" " + mp.from_app.flashMsg)
		} else {
			bottombar = bottombarStyle.Width(mp.from_app.width).Render(" " + mp.from_app.flashMsg)
		}
	} else {
		inputText := " Message: " + mp.input
		if mp.from_app.flashMsg != "" {
			inputText = " " + mp.from_app.flashMsg
		}
		bottombarPadding := strings.Repeat(" ", max(0, mp.from_app.width-utf8.RuneCountInString(inputText)))
		bottombar = bottombarStyle.Width(mp.from_app.width).Render(inputText + bottombarPadding)
	}
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
		sender := msg.From
		if strings.Contains(mp.from_chat.ID, "@g.us") {
			sender = msg.GroupFrom
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
				originalBody = "[MEDIA] " + originalBody
			}
			// Truncate original body if too long (adjust as needed)
			if len(originalBody) > 30 {
				originalBody = originalBody[:27] + "..."
			}
			replyPrefix = fmt.Sprintf("[REPLY: %s] ", originalBody)
		}

		// Handle media indicator
		var mediaPrefix string
		if msg.HasMedia {
			mediaPrefix = "[MEDIA] "
		}

		// Combine all prefixes with the body
		combinedPrefix := replyPrefix + mediaPrefix
		fullBody := combinedPrefix + body

		// Calculate available from_app.width for message content
		availableWidth := mp.from_app.width - fullPrefixLength

		// Wrap the message body BEFORE applying styling
		wrappedLines := wrapText(fullBody, availableWidth)

		// Check if this message has reply highlight or is being replied to
		hasReplyHighlight := mp.replyHighlights[i] || (mp.replyingToMsg == i)
		selected := mp.selectedMsg == i && !mp.inInput

		// Apply styling to msgPrefix if it's from me (unless reply highlighted)
		styledMsgPrefix := msgPrefix
		if !hasReplyHighlight && !selected && msg.FromMe {
			styledMsgPrefix = selfPrefix.Render(msgPrefix)
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
						styledReplyIndicator := replyHighlight.Render(replyIndicatorText) + " "

						// Handle media in the reply indicator
						if strings.Contains(replyIndicatorText, "[MEDIA]") {
							// Split the reply indicator to style media separately
							parts := strings.Split(replyIndicatorText, "[MEDIA]")
							var styledParts []string
							for i, part := range parts {
								if i > 0 {
									styledParts = append(styledParts, hyperlink.Render("[MEDIA]"))
								}
								styledParts = append(styledParts, replyHighlight.Render(part))
							}
							styledReplyIndicator = strings.Join(styledParts, "") + " "
						}

						// Handle media and self styling for the rest
						if strings.HasPrefix(rest, "[MEDIA]") {
							mediaLabel := "[MEDIA]"
							afterMedia := strings.TrimPrefix(rest, mediaLabel)
							if msg.FromMe {
								firstLine = styledReplyIndicator + hyperlink.Render(mediaLabel) + selfBody.Render(afterMedia)
							} else {
								firstLine = styledReplyIndicator + hyperlink.Render(mediaLabel) + afterMedia
							}
						} else if msg.FromMe {
							firstLine = styledReplyIndicator + selfBody.Render(rest)
						} else {
							firstLine = styledReplyIndicator + rest
						}
					}
				} else if strings.HasPrefix(firstLine, "[MEDIA]") {
					// Handle media without reply
					mediaLabel := "[MEDIA]"
					rest := strings.TrimPrefix(firstLine, mediaLabel)
					if msg.FromMe {
						firstLine = hyperlink.Render(mediaLabel) + selfBody.Render(rest)
					} else {
						firstLine = hyperlink.Render(mediaLabel) + rest
					}
				} else if msg.FromMe {
					// For self messages without media or reply, apply self styling
					firstLine = selfBody.Render(firstLine)
				}
			}

			// Build the complete first line
			completeLine := fmt.Sprintf("%s%s%s", linePrefix, styledMsgPrefix, firstLine)

			// Apply reply highlight to the entire line if needed
			if hasReplyHighlight || selected {
				// Pad the line to full from_app.width and apply highlight
				padding := strings.Repeat(" ", max(0, mp.from_app.width-utf8.RuneCountInString(completeLine)))
				completeLine = replyHighlight.Width(mp.from_app.width).Render(completeLine + padding)
			}

			messageLines = append(messageLines, completeLine)

			// Print continuation lines with padding and consistent styling
			padding := strings.Repeat(" ", fullPrefixLength)
			for j := 1; j < len(wrappedLines); j++ {
				continuationLine := wrappedLines[j]

				// Apply consistent styling to continuation lines (unless reply highlighted)
				if !hasReplyHighlight && msg.FromMe {
					continuationLine = selfBody.Render(continuationLine)
				}

				// Build complete continuation line
				completeContLine := fmt.Sprintf("%s%s", padding, continuationLine)

				// Apply reply highlight to continuation lines if needed
				if hasReplyHighlight {
					linePadding := strings.Repeat(" ", max(0, mp.from_app.width-utf8.RuneCountInString(completeContLine)))
					completeContLine = replyHighlight.Width(mp.from_app.width).Render(completeContLine + linePadding)
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
			return err
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
			return messsageFlashMsg("File not found!")
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
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return flashTickMsg{}
	})
}

// --- Clipboard media helper ---
// Returns a temp file path containing clipboard image or file.
// Supports PNG/JPEG images and file paths (if clipboard contains a file).
func getClipboardMediaFile() (string, error) {
	// Try platform-specific clipboard image extraction
	switch runtime.GOOS {
	case "windows":
		// Try PowerShell to get image from clipboard as PNG
		tmpfile, err := ioutil.TempFile("", "clipimg-*.png")
		if err != nil {
			return "", err
		}
		tmpfile.Close()
		psScript := `[void][System.Reflection.Assembly]::LoadWithPartialName("System.Windows.Forms")
if ([Windows.Forms.Clipboard]::ContainsImage()) {
  $img = [Windows.Forms.Clipboard]::GetImage()
  $img.Save("` + tmpfile.Name() + `", [System.Drawing.Imaging.ImageFormat]::Png)
  Write-Output "OK"
} else {
  Write-Output "NOIMG"
}`
		cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
		out, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(out), "OK") {
			return tmpfile.Name(), nil
		}
		os.Remove(tmpfile.Name())
		// Try file path from clipboard (for drag-drop)
		psScript2 := `Add-Type -AssemblyName PresentationCore; $f=[Windows.Clipboard]::GetFileDropList(); if ($f.Count -gt 0) { Write-Output $f[0] }`
		cmd2 := exec.Command("powershell", "-NoProfile", "-Command", psScript2)
		out2, err2 := cmd2.Output()
		if err2 == nil && len(strings.TrimSpace(string(out2))) > 0 {
			return strings.TrimSpace(string(out2)), nil
		}
		return "", fmt.Errorf("no image or file in clipboard")
	case "darwin":
		// Try pbpaste for PNG
		tmpfile, err := ioutil.TempFile("", "clipimg-*.png")
		if err != nil {
			return "", err
		}
		tmpfile.Close()
		cmd := exec.Command("bash", "-c", "pngpaste "+tmpfile.Name())
		if err := cmd.Run(); err == nil {
			// pngpaste succeeded
			return tmpfile.Name(), nil
		}
		os.Remove(tmpfile.Name())
		// Try pbpaste for file path (from Finder)
		cmd2 := exec.Command("osascript", "-e", `try
set theFiles to the clipboard as «class furl»
set thePath to POSIX path of (theFiles as text)
on error
return ""
end try`)
		out2, err2 := cmd2.Output()
		if err2 == nil && len(strings.TrimSpace(string(out2))) > 0 {
			return strings.TrimSpace(string(out2)), nil
		}
		return "", fmt.Errorf("no image or file in clipboard (install pngpaste for images)")
	default:
		// Linux: try wl-paste (Wayland) or xclip/xsel (X11)
		// Try wl-paste --type image/png
		tmpfile, err := ioutil.TempFile("", "clipimg-*.png")
		if err != nil {
			return "", err
		}
		tmpfile.Close()
		cmd := exec.Command("bash", "-c", "wl-paste --type image/png > "+tmpfile.Name())
		if err := cmd.Run(); err == nil {
			fi, _ := os.Stat(tmpfile.Name())
			if fi != nil && fi.Size() > 0 {
				return tmpfile.Name(), nil
			}
		}
		os.Remove(tmpfile.Name())
		// Try xclip -selection clipboard -t image/png
		tmpfile2, err := ioutil.TempFile("", "clipimg-*.png")
		if err == nil {
			tmpfile2.Close()
			cmd2 := exec.Command("bash", "-c", "xclip -selection clipboard -t image/png -o > "+tmpfile2.Name())
			if err := cmd2.Run(); err == nil {
				fi, _ := os.Stat(tmpfile2.Name())
				if fi != nil && fi.Size() > 0 {
					return tmpfile2.Name(), nil
				}
			}
			os.Remove(tmpfile2.Name())
		}
		// Try file path from clipboard (Nautilus, etc)
		cmd3 := exec.Command("xclip", "-selection", "clipboard", "-o")
		out3, err3 := cmd3.Output()
		if err3 == nil {
			path := strings.TrimSpace(string(out3))
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
		return "", fmt.Errorf("no image or file in clipboard (try wl-paste/xclip/xsel)")
	}
}
