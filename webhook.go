package main

import (
	"encoding/json"
	// "fmt"
	"log"
	"net/http"
	tea "github.com/charmbracelet/bubbletea"
)

type webhookMsg struct {
	Chat  		chat 		`json:"chat"`
	Message 	message 	`json:"message"`
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


