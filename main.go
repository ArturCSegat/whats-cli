package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const baseURL = "http://localhost:3000"

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hyperlink       = lipgloss.NewStyle().Background(lipgloss.Color("#0000FF")).Foreground(lipgloss.Color("#FFFFFF"))
	selfPrefix      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	selfBody        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7FFF7F"))
	topbarStyle     = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15")).Bold(true)
	bottombarStyle  = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15"))
	replyHighlight  = lipgloss.NewStyle().Background(lipgloss.Color("#FFFFFF")).Foreground(lipgloss.Color("#000000"))
)

type chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

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

type model struct {
	chats           []chat
	messages        []message
	selectedChat    int
	selectedMsg     int
	input           string
	inInput         bool   // true when buffer focused
	view            string // "loading", "chats", "messages"
	err             error
	width           int
	height          int
	replyHighlights map[int]bool // tracks which messages have reply highlight
	replyingToMsg   int          // index of message being replied to (-1 if not replying)
}

func initialModel() model {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	return model{
		view:            "loading",
		inInput:         true,
		width:           width,
		height:          height,
		replyHighlights: make(map[int]bool),
		replyingToMsg:   -1,
	}
}

type errMsg error

type chatsMsg []chat

type messagesMsg []message

func (m model) Init() tea.Cmd { return getChats }

func getChats() tea.Msg {
	res, err := http.Get(fmt.Sprintf("%s/client/1/chat", baseURL))
	if err != nil {
		return errMsg(err)
	}
	defer res.Body.Close()
	var chats []chat
	if err := json.NewDecoder(res.Body).Decode(&chats); err != nil {
		return errMsg(err)
	}
	return chatsMsg(chats)
}

func getMessages(chatId string) tea.Cmd {
	return func() tea.Msg {
		res, err := http.Get(fmt.Sprintf("%s/client/1/chat/%s/messages", baseURL, chatId))
		if err != nil {
			return errMsg(err)
		}
		defer res.Body.Close()
		var msgs []message
		if err := json.NewDecoder(res.Body).Decode(&msgs); err != nil {
			return errMsg(err)
		}
		return messagesMsg(msgs)
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
			return errMsg(err)
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
			return errMsg(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return getMessages(chatId)()
	}
}

func openURL(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", "", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}

// findMessageByID finds a message by its ID and returns its body with media indicator if applicable
func (m model) findMessageByID(msgID string) string {
	for _, msg := range m.messages {
		if msg.MsgID == msgID {
			body := msg.Body
			if msg.HasMedia {
				body = "[MEDIA] " + body
			}
			return body
		}
	}
	return ""
}

// wrapText wraps text to fit within the specified width, preserving words
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var currentLine []string
	currentLength := 0

	for _, word := range words {
		wordLength := utf8.RuneCountInString(word)

		// If adding this word would exceed the width, start a new line
		if currentLength > 0 && currentLength+1+wordLength > width {
			lines = append(lines, strings.Join(currentLine, " "))
			currentLine = []string{word}
			currentLength = wordLength
		} else {
			currentLine = append(currentLine, word)
			if currentLength > 0 {
				currentLength += 1 // space
			}
			currentLength += wordLength
		}
	}

	// Add the last line
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " "))
	}

	return lines
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.view == "chats" {
				if m.selectedChat > 0 {
					m.selectedChat--
				}
			} else if m.view == "messages" {
				if m.inInput {
					m.inInput = false
					m.selectedMsg = len(m.messages) - 1
				} else if m.selectedMsg > 0 {
					m.selectedMsg--
				}
			}
		case "down":
			if m.view == "chats" {
				if m.selectedChat < len(m.chats)-1 {
					m.selectedChat++
				}
			} else if m.view == "messages" {
				if !m.inInput {
					if m.selectedMsg < len(m.messages)-1 {
						m.selectedMsg++
					} else {
						m.inInput = true
					}
				}
			}
		case "enter":
			if m.view == "chats" && len(m.chats) > 0 {
				m.view = "messages"
				m.inInput = true
				// Clear reply highlights and replying state when switching chats
				m.replyHighlights = make(map[int]bool)
				m.replyingToMsg = -1
				return m, getMessages(m.chats[m.selectedChat].ID)
			} else if m.view == "messages" && m.inInput {
				// Check if we're replying to a message
				if m.replyingToMsg != -1 && m.replyingToMsg < len(m.messages) {
					cmd := sendReply(m.chats[m.selectedChat].ID, m.input, m.messages[m.replyingToMsg].MsgID)
					m.input = ""
					m.replyingToMsg = -1 // Clear reply state after sending
					return m, cmd
				} else {
					cmd := sendMessage(m.chats[m.selectedChat].ID, m.input)
					m.input = ""
					return m, cmd
				}
			}
		case "r", "R":
			if m.view == "messages" && !m.inInput && m.selectedMsg >= 0 && m.selectedMsg < len(m.messages) {
				// Toggle reply highlight for the selected message
				if m.replyHighlights[m.selectedMsg] {
					// If already highlighted, remove highlight and clear reply state
					m.replyHighlights[m.selectedMsg] = false
					m.replyingToMsg = -1
				} else {
					// Clear any existing highlights and set new one
					m.replyHighlights = make(map[int]bool)
					m.replyHighlights[m.selectedMsg] = true
					m.replyingToMsg = m.selectedMsg
					// Shift focus to input buffer
					m.inInput = true
				}
			} else if m.view == "messages" && m.inInput {
				// allow typing 'r' in buffer
				m.input += key
			}
		case "m", "M":
			if m.view == "messages" && !m.inInput {
				if m.messages[m.selectedMsg].HasMedia {
					mediaURL := fmt.Sprintf("%s/client/1/message/%s/media", baseURL, m.messages[m.selectedMsg].MsgID)
					openURL(mediaURL)
				}
			} else if m.view == "messages" && m.inInput {
				// allow typing 'm' in buffer
				m.input += key
			}
		case "esc":
			if m.view == "messages" {
				if m.inInput && m.replyingToMsg != -1 {
					// If replying and in input, clear reply state and go back to message selection
					m.replyHighlights = make(map[int]bool)
					m.replyingToMsg = -1
					m.inInput = false
				} else {
					m.view = "chats"
				}
			}
		default:
			if m.view == "messages" && m.inInput {
				switch msg.Type {
				case tea.KeyRunes:
					m.input += msg.String()
				case tea.KeySpace:
					m.input += " "
				case tea.KeyBackspace:
					if len(m.input) > 0 {
						m.input = m.input[:len(m.input)-1]
					}
				}
			}
		}
	case chatsMsg:
		m.chats = msg
		m.view = "chats"
	case messagesMsg:
		m.messages = msg
		m.inInput = true
		m.selectedMsg = len(msg) - 1
		// Clear reply highlights and replying state when messages are refreshed
		m.replyHighlights = make(map[int]bool)
		m.replyingToMsg = -1
	case errMsg:
		m.err = msg
		m.view = "error"
	}
	return m, nil
}

func (m model) View() string {
	switch m.view {
	case "loading":
		return "Loading chats..."
	case "error":
		return fmt.Sprintf("Error: %v", m.err)
	case "chats":
		var b strings.Builder
		b.WriteString("Chats (↑ ↓ to enter select mode, Enter to open, q to quit):\n\n")

		// Calculate available height for chats (total height - header - padding)
		availableHeight := m.height - 3 // 1 for header, 1 for empty line, 1 for padding
		if availableHeight < 1 {
			availableHeight = 1
		}

		// Calculate which chats to show based on selection and available height
		startIndex := 0
		endIndex := len(m.chats)

		// If we have more chats than available height, center the selection
		if len(m.chats) > availableHeight {
			startIndex = m.selectedChat - availableHeight/2
			if startIndex < 0 {
				startIndex = 0
			}
			endIndex = startIndex + availableHeight
			if endIndex > len(m.chats) {
				endIndex = len(m.chats)
				startIndex = endIndex - availableHeight
				if startIndex < 0 {
					startIndex = 0
				}
			}
		}

		for i := startIndex; i < endIndex; i++ {
			c := m.chats[i]
			name := c.Name
			if name == "" {
				name = c.ID
			}
			if i == m.selectedChat {
				b.WriteString(fmt.Sprintf("> %s\n", selectedStyle.Render(name)))
			} else {
				b.WriteString(fmt.Sprintf("  %s\n", unselectedStyle.Render(name)))
			}
		}
		return b.String()
	case "messages":
		var b strings.Builder

		// Fixed topbar - full width
		var topbarText string
		if !m.inInput {
			if m.selectedMsg >= 0 && m.selectedMsg < len(m.messages) {
				// Check if the selected message has reply highlight
				if m.replyHighlights[m.selectedMsg] {
					msg := m.messages[m.selectedMsg]
					topbarText = fmt.Sprintf(" Replying to \"%s\" (ID: %s)", msg.Body, msg.MsgID)
				} else {
					topbarText = fmt.Sprintf(" Selected: %s (R to toggle reply highlight)", m.messages[m.selectedMsg].MsgID)
				}
			} else {
				topbarText = " Selected: "
			}
		} else {
			// Check if we're in reply mode
			if m.replyingToMsg != -1 && m.replyingToMsg < len(m.messages) {
				msg := m.messages[m.replyingToMsg]
				topbarText = fmt.Sprintf(" Replying to \"%s\" (ID: %s)", msg.Body, msg.MsgID)
			} else {
				topbarText = " Messages (↑ ↓ to select, Enter to send, m to open media, R to toggle reply, Esc to go back)"
			}
		}

		// Truncate topbar text if it exceeds terminal width
		topbarTextLength := utf8.RuneCountInString(topbarText)
		if topbarTextLength > m.width {
			if m.width > 3 {
				topbarText = string([]rune(topbarText)[:m.width-3]) + "..."
			} else {
				topbarText = string([]rune(topbarText)[:m.width])
			}
		}

		topbarPadding := strings.Repeat(" ", max(0, m.width-utf8.RuneCountInString(topbarText)))
		topbar := topbarStyle.Width(m.width).Render(topbarText + topbarPadding)
		b.WriteString(topbar + "\n")

		// Calculate available height for messages (total height - topbar - bottombar)
		availableHeight := m.height - 2 // 1 for topbar, 1 for bottombar
		if availableHeight < 1 {
			availableHeight = 1
		}

		// Build messages content
		var messageLines []string
		for i, msg := range m.messages {
			var linePrefix string
			if !m.inInput {
				if i == m.selectedMsg {
					linePrefix = "> "
				} else {
					linePrefix = "  "
				}
			}

			ts := msg.Timestamp.Local().Format("15:04")
			sender := msg.From
			if strings.Contains(m.chats[m.selectedChat].ID, "@g.us") {
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
				originalBody := m.findMessageByID(msg.ResponseToID)
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

			// Calculate available width for message content
			availableWidth := m.width - fullPrefixLength

			// Wrap the message body BEFORE applying styling
			wrappedLines := wrapText(fullBody, availableWidth)

			// Check if this message has reply highlight or is being replied to
			hasReplyHighlight := m.replyHighlights[i] || (m.replyingToMsg == i)

			// Apply styling to msgPrefix if it's from me (unless reply highlighted)
			styledMsgPrefix := msgPrefix
			if !hasReplyHighlight && msg.FromMe {
				styledMsgPrefix = selfPrefix.Render(msgPrefix)
			}

			if len(wrappedLines) > 0 {
				firstLine := wrappedLines[0]

				// Apply styling to the first line content (unless reply highlighted)
				if !hasReplyHighlight {
					// Handle reply indicator styling
					if strings.HasPrefix(firstLine, "[REPLY:") {
						// Find the end of the reply indicator by looking for the last "] "
						replyEndIdx := strings.LastIndex(firstLine, "] ")
						if replyEndIdx != -1 {
							replyIndicatorText := firstLine[:replyEndIdx+1] // Include "]" but not the space
							rest := firstLine[replyEndIdx+2:]               // Skip "] "

							// Style the reply indicator without the trailing space
							styledReplyIndicator := replyHighlight.Render(replyIndicatorText) + " "

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
				if hasReplyHighlight {
					// Pad the line to full width and apply highlight
					padding := strings.Repeat(" ", max(0, m.width-utf8.RuneCountInString(completeLine)))
					completeLine = replyHighlight.Width(m.width).Render(completeLine + padding)
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
						linePadding := strings.Repeat(" ", max(0, m.width-utf8.RuneCountInString(completeContLine)))
						completeContLine = replyHighlight.Width(m.width).Render(completeContLine + linePadding)
					}

					messageLines = append(messageLines, completeContLine)
				}
			}
		}

		// Display messages with scrolling if needed
		if len(messageLines) > availableHeight {
			// Show the most recent messages that fit
			startIdx := len(messageLines) - availableHeight
			messageLines = messageLines[startIdx:]
		}

		// Fill remaining space if needed
		for len(messageLines) < availableHeight {
			messageLines = append([]string{""}, messageLines...)
		}

		for _, line := range messageLines {
			b.WriteString(line + "\n")
		}

		// Fixed bottom input bar - full width
		inputText := " Message: " + m.input
		bottombarPadding := strings.Repeat(" ", max(0, m.width-len(inputText)))
		bottombar := bottombarStyle.Width(m.width).Render(inputText + bottombarPadding)
		b.WriteString(bottombar)

		return b.String()
	default:
		return "Unknown state"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	if err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
