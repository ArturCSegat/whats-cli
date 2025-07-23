package main

import (
	"encoding/json"
	// "fmt"
	"log"
	"net/http"
	"time"
	tea "github.com/charmbracelet/bubbletea"
)

type webhookMsg struct {
	Chat struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		UnreadCount    int    `json:"unreadCount"`
		LastMessage    string `json:"lastMessageBody"`
		IsArchived     bool   `json:"isArchived"`
		IsGroup        bool   `json:"isGroup"`
		IsMuted        bool   `json:"isMuted"`
		IsReadOnly     bool   `json:"isReadOnly"`
		IsPinned       bool   `json:"isPinned"`
	} `json:"chat"`

	Message struct {
		ID              string    `json:"id"`
		From            string    `json:"from"`
		GroupMemberFrom *string   `json:"group_member_from"` // use *string for possible `undefined` (null)
		FromMe          bool      `json:"fromMe"`
		Body            string    `json:"body"`
		Timestamp       time.Time `json:"timestamp"`
		HasMedia        bool      `json:"hasMedia"`
		IsQuote         bool      `json:"isQuote"`
		QuoteID         string    `json:"quoteId"`
		IsForwarded     bool      `json:"isForwarded"`
		MentionedIDs    []string  `json:"mentionedIds"`
		Info            map[string]any `json:"info"`
	} `json:"message"`
}

func startWebhookListener(cmdChan chan tea.Msg) {
	http.HandleFunc("/whatshttp/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "must POST", http.StatusNotFound)
			return
		}

		var hook webhookMsg
		if err := json.NewDecoder(r.Body).Decode(&hook); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		
		cmdChan <- hook;

		w.WriteHeader(http.StatusOK)
	})

	go func() {
		log.Println("Starting webhook server on :4000")
		if err := http.ListenAndServe(":4000", nil); err != nil {
			log.Fatalf("Webhook listener failed: %v", err)
		}
	}()
}


