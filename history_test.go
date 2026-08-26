package main

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestContactMetadataDoesNotCreateChat(t *testing.T) {
	service := &whatsmeowClient{
		chats:        make(map[string]Chat),
		contactNames: make(map[string]resolvedName),
	}
	chatID := "1234567890@lid"
	resolved := resolvedName{name: "Alice", priority: namePriorityAddressBook}

	service.applyContactNameLocked(chatID, resolved)
	if len(service.chats) != 0 {
		t.Fatalf("contact metadata created non-chat entries: %#v", service.chats)
	}
	if service.contactNames[chatID] != resolved {
		t.Fatalf("contact metadata was not retained: %#v", service.contactNames)
	}

	service.chats[chatID] = Chat{ID: chatID}
	service.applyContactNameLocked(chatID, resolved)
	if service.chats[chatID].Name != "Alice" {
		t.Fatalf("actual chat did not receive contact name: %#v", service.chats[chatID])
	}
}

func TestHistoryStoreTracksInitialSyncAndClear(t *testing.T) {
	store, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	complete, err := store.hasInitialSync()
	if err != nil || complete {
		t.Fatalf("new history sync state = (%v, %v)", complete, err)
	}
	hasChats, err := store.hasChats()
	if err != nil || hasChats {
		t.Fatalf("new history chat state = (%v, %v)", hasChats, err)
	}
	if err = store.markInitialSync(); err != nil {
		t.Fatal(err)
	}
	if complete, err = store.hasInitialSync(); err != nil || !complete {
		t.Fatalf("marked history sync state = (%v, %v)", complete, err)
	}
	if err = store.clear(); err != nil {
		t.Fatal(err)
	}
	if complete, err = store.hasInitialSync(); err != nil || complete {
		t.Fatalf("cleared history sync state = (%v, %v)", complete, err)
	}
}

func TestHistoryStoreRestoresActualChatsAndMessages(t *testing.T) {
	dir := t.TempDir()
	store, err := openHistoryStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	chatID := "1234567890@lid"
	service := &whatsmeowClient{
		history:      store,
		chats:        make(map[string]Chat),
		messages:     make(map[string][]message),
		messagesByID: make(map[string]message),
		contactNames: map[string]resolvedName{
			chatID: {name: "Alice", priority: namePriorityAddressBook},
		},
		groupNames: make(map[string]resolvedName),
	}
	msg := message{
		MsgID:      "persisted-message",
		ChatID:     chatID,
		From:       chatID,
		Body:       "persist me",
		Timestamp:  time.Unix(1_700_000_000, 0),
		RawMessage: &waE2E.Message{Conversation: proto.String("persist me")},
	}
	service.storeMessage(msg, false)
	hasChats, err := store.hasChats()
	if err != nil || !hasChats {
		t.Fatalf("populated history chat state = (%v, %v)", hasChats, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := openHistoryStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored := &whatsmeowClient{
		chats:        make(map[string]Chat),
		messages:     make(map[string][]message),
		messagesByID: make(map[string]message),
	}
	if err = reopened.load(restored); err != nil {
		t.Fatal(err)
	}

	chat, ok := restored.chats[chatID]
	if !ok || chat.Name != "Alice" || chat.LastMessage.MsgID != msg.MsgID {
		t.Fatalf("restored chat = %#v", chat)
	}
	messages := restored.getMessages(chatID)
	if len(messages) != 1 || messages[0].Body != msg.Body || messages[0].RawMessage.GetConversation() != msg.Body {
		t.Fatalf("restored messages = %#v", messages)
	}
}
