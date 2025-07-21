package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const baseURL = "http://localhost:3000"

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type message struct {
	From string `json:"from"`
	GroupFrom string `json:"group_member_from"`
	Body   string `json:"body"`
	Type   string `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

type model struct {
	chats    []chat
	messages []message
	selected int
	input    string
	view     string // "loading", "chats", "messages"
	err      error
}

func initialModel() model {
	return model{view: "loading"}
}

// --- API Messages ---

type errMsg error
type chatsMsg []chat
type messagesMsg []message

// --- Initialization ---

func (m model) Init() tea.Cmd {
	return getChats
}

// --- API Calls ---

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

// --- Update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c":
			return m, tea.Quit

		case "up":
			if m.view == "chats" && m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.view == "chats" && m.selected < len(m.chats)-1 {
				m.selected++
			}
		case "enter":
			if m.view == "chats" && len(m.chats) > 0 {
				m.view = "messages"
				return m, getMessages(m.chats[m.selected].ID)
			} else if m.view == "messages" && m.input != "" {
				cmd := sendMessage(m.chats[m.selected].ID, m.input)
				m.input = ""
				return m, cmd
			}
		case "esc":
			if m.view == "messages" {
				m.view = "chats"
			}
		default:
			if m.view == "messages" {
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
	case errMsg:
		m.err = msg
		m.view = "error"
	}

	return m, nil
}

// --- View ---

func (m model) View() string {
	switch m.view {
	case "loading":
		return "Loading chats..."
	case "error":
		return fmt.Sprintf("Error: %v", m.err)
	case "chats":
		var b strings.Builder
		b.WriteString("Chats (↑ ↓ to navigate, Enter to open, q to quit):\n\n")
		for i, chat := range m.chats {
			if i > 10 {
				break;
			}
			name := chat.Name
			if name == "" {
				name = chat.ID
			}
			if i == m.selected {
				b.WriteString(fmt.Sprintf("> %s\n", selectedStyle.Render(name)))
			} else {
				b.WriteString(fmt.Sprintf("%s\n", unselectedStyle.Render(name)))
			}
		}
		return b.String()

	case "messages":
		var b strings.Builder
		b.WriteString("Messages (Esc to go back, Enter to send):\n\n")
		for _, msg := range m.messages {
			chat_id := m.chats[m.selected].ID
			sender := ""
			if strings.Contains(chat_id, "@g.us") {
				sender = msg.GroupFrom;
			} else {
				sender = msg.From;
			}
			ts := msg.Timestamp.Local().Format("15:04")	// normalize timezone to the system's time and then format it to 24hr format
			b.WriteString(fmt.Sprintf("[%s] <%s>: %s\n", ts, sender, msg.Body))	// [TI:ME] <Author>: Message
		}
		b.WriteString("\nMessage: " + m.input)
		return b.String()

	default:
		return "Unknown state"
	}
}

// --- Main ---

func main() {
	if err := tea.NewProgram(initialModel()).Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

