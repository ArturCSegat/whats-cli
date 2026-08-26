package main

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	waCompanionReg "go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestConfigureFullHistorySync(t *testing.T) {
	original := proto.Clone(store.DeviceProps)
	defer func() {
		store.DeviceProps = original.(*waCompanionReg.DeviceProps)
	}()

	configureFullHistorySync()
	if !store.DeviceProps.GetRequireFullSync() {
		t.Fatal("full history sync is not required")
	}
	config := store.DeviceProps.GetHistorySyncConfig()
	if config.GetFullSyncDaysLimit() != 3650 || config.GetInitialSyncMaxMessagesPerChat() != 100 {
		t.Fatalf("unexpected history sync configuration: %#v", config)
	}
}

func TestTerminalQRLinesPackVerticalModules(t *testing.T) {
	bitmap := [][]bool{
		{true, true, false, false},
		{false, false, false, false},
		{true, false, false, true},
		{true, false, true, true},
	}

	lines := terminalQRLines(bitmap)
	if len(lines) != 2 || lines[0] != "▀▀  " || lines[1] != "█ ▄█" {
		t.Fatalf("terminal QR lines = %#v", lines)
	}
	if odd := terminalQRLines([][]bool{{true}}); len(odd) != 1 || odd[0] != "▀" {
		t.Fatalf("odd-sized QR line = %#v", odd)
	}
}

func TestDirectContactsIncludePhoneAndLIDJIDs(t *testing.T) {
	phone := types.NewJID("15551234567", types.DefaultUserServer)
	lid := types.NewJID("1234567890", types.HiddenUserServer)
	group := types.NewJID("120363000000000000", types.GroupServer)

	if !isDirectContact(phone) || !isDirectContact(lid) {
		t.Fatal("phone-number and LID contacts must both be visible")
	}
	if isDirectContact(group) {
		t.Fatal("group JID must not be loaded as an individual contact")
	}
}

func TestContactDisplayNameUsesSyncedNames(t *testing.T) {
	contact := types.ContactInfo{FirstName: "Alice", PushName: "Ignored push name"}
	name, priority := contactDisplayName(contact)
	if name != "Alice" || priority != namePriorityAddressBook {
		t.Fatalf("contact identity = (%q, %d), want (Alice, %d)", name, priority, namePriorityAddressBook)
	}
	name, priority = contactDisplayName(types.ContactInfo{})
	if name != "" || priority != namePriorityUnknown {
		t.Fatalf("fallback contact identity = (%q, %d)", name, priority)
	}
}

func TestLowerPriorityNameCannotReplaceSyncedName(t *testing.T) {
	chatID := "1234567890@lid"
	service := &whatsmeowClient{chats: map[string]Chat{
		chatID: {ID: chatID, Name: "Alice", namePriority: namePriorityAddressBook},
	}}

	service.upsertChatLocked(Chat{ID: chatID, Name: "1234567890", namePriority: namePriorityHistory})
	if name := service.chats[chatID].Name; name != "Alice" {
		t.Fatalf("history fallback replaced synced name: %q", name)
	}

	service.upsertChatLocked(Chat{ID: chatID, Name: "Alice Cooper", namePriority: namePriorityAddressBook})
	if name := service.chats[chatID].Name; name != "Alice Cooper" {
		t.Fatalf("same-priority contact update was ignored: %q", name)
	}
}

func TestContactNameForGroupSenderAliases(t *testing.T) {
	service := &whatsmeowClient{contactNames: map[string]resolvedName{
		"1234567890@lid":             {name: "Push name", priority: namePriorityPush},
		"15551234567@s.whatsapp.net": {name: "Alice", priority: namePriorityAddressBook},
	}}

	name, ok := service.contactNameForIDs("1234567890@lid", "15551234567@s.whatsapp.net")
	if !ok || name != "Alice" {
		t.Fatalf("resolved group sender = (%q, %v), want Alice", name, ok)
	}
}

func TestMessageFromEventPreservesChatAndReplyMetadata(t *testing.T) {
	service := &whatsmeowClient{}
	chat := types.NewJID("120363000000000000", types.GroupServer)
	sender := types.NewJID("15551234567", types.DefaultUserServer)
	timestamp := time.Unix(1_700_000_000, 0)
	info := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:      chat,
			Sender:    sender,
			SenderAlt: types.NewJID("1234567890", types.HiddenUserServer),
			IsGroup:   true,
		},
		ID:        types.MessageID("message-id"),
		Timestamp: timestamp,
		PushName:  "Alice",
	}
	raw := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("hello"),
		ContextInfo: &waE2E.ContextInfo{
			StanzaID:     proto.String("quoted-id"),
			IsForwarded:  proto.Bool(true),
			MentionedJID: []string{"15557654321@s.whatsapp.net"},
		},
	}}

	msg, ok := service.messageFromEvent(info, raw)
	if !ok {
		t.Fatal("expected supported text message")
	}
	if msg.ChatID != chat.String() {
		t.Fatalf("chat ID = %q, want %q", msg.ChatID, chat.String())
	}
	if msg.GroupFrom != sender.String() || msg.GroupFromAlt != info.SenderAlt.String() || msg.SenderName != "Alice" || msg.Body != "hello" {
		t.Fatalf("unexpected normalized message: %#v", msg)
	}
	if !msg.IsResponse || msg.ResponseToID != "quoted-id" || !msg.IsForwarded {
		t.Fatalf("reply/forward metadata was not preserved: %#v", msg)
	}
	if len(msg.MentionedIDs) != 1 || msg.MentionedIDs[0] != "15557654321@s.whatsapp.net" {
		t.Fatalf("mentions = %#v", msg.MentionedIDs)
	}
}

func TestLiveMessagePublishesRealtimeUpdate(t *testing.T) {
	updates := make(chan tea.Msg, 1)
	chat := types.NewJID("120363000000000000", types.GroupServer)
	sender := types.NewJID("15551234567", types.DefaultUserServer)
	service := &whatsmeowClient{
		cmdChan:      updates,
		chats:        map[string]Chat{chat.String(): {ID: chat.String(), Name: "Family", IsGroup: true}},
		messages:     make(map[string][]message),
		messagesByID: make(map[string]message),
	}
	event := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsGroup: true},
			ID:            types.MessageID("live-message"),
			Timestamp:     time.Unix(1_700_000_001, 0),
		},
		Message: &waE2E.Message{Conversation: proto.String("live text")},
	}

	service.handleEvent(event)

	select {
	case rawUpdate := <-updates:
		update, ok := rawUpdate.(whatsmeowEventMsg)
		if !ok {
			t.Fatalf("update type = %T, want whatsmeowEventMsg", rawUpdate)
		}
		if update.Chat.Name != "Family" || update.Message.Body != "live text" {
			t.Fatalf("unexpected realtime update: %#v", update)
		}
		if stored := service.getMessages(chat.String()); len(stored) != 1 || stored[0].MsgID != "live-message" {
			t.Fatalf("live message was not stored: %#v", stored)
		}
	case <-time.After(time.Second):
		t.Fatal("live message update was not published")
	}
}

func TestCloneAsForwardedDoesNotMutateOriginal(t *testing.T) {
	original := &waE2E.Message{Conversation: proto.String("forward me")}

	forwarded, err := cloneAsForwarded(original)
	if err != nil {
		t.Fatal(err)
	}
	if original.GetConversation() != "forward me" || original.GetExtendedTextMessage() != nil {
		t.Fatalf("original message was mutated: %#v", original)
	}
	if forwarded.GetConversation() != "" || forwarded.GetExtendedTextMessage().GetText() != "forward me" {
		t.Fatalf("forwarded text was not converted correctly: %#v", forwarded)
	}
	contextInfo := forwarded.GetExtendedTextMessage().GetContextInfo()
	if !contextInfo.GetIsForwarded() || contextInfo.GetForwardingScore() != 1 {
		t.Fatalf("forward metadata = %#v", contextInfo)
	}
}

func TestStoreMessageUsesEventChatID(t *testing.T) {
	service := &whatsmeowClient{
		chats:        make(map[string]Chat),
		messages:     make(map[string][]message),
		messagesByID: make(map[string]message),
	}
	chatID := "120363000000000000@g.us"
	msg := message{
		MsgID:     "message-id",
		ChatID:    chatID,
		From:      "15551234567@s.whatsapp.net",
		GroupFrom: "15551234567@s.whatsapp.net",
		Timestamp: time.Unix(1_700_000_000, 0),
		Body:      "group message",
	}

	chat := service.storeMessage(msg, true)
	if chat.ID != chatID || !chat.IsGroup {
		t.Fatalf("stored under wrong chat: %#v", chat)
	}
	if chat.UnreadCount != 1 || len(service.messages[chatID]) != 1 {
		t.Fatalf("message state was not updated: chat=%#v messages=%#v", chat, service.messages)
	}

	duplicate := msg
	duplicate.Body = "updated group message"
	chat = service.storeMessage(duplicate, true)
	if chat.UnreadCount != 1 || len(service.messages[chatID]) != 1 {
		t.Fatalf("duplicate delivery changed unread or message counts: chat=%#v messages=%#v", chat, service.messages)
	}
	if service.messages[chatID][0].Body != duplicate.Body {
		t.Fatalf("duplicate delivery did not refresh message metadata: %#v", service.messages[chatID][0])
	}
}
