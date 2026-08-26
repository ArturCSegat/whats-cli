package main

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-colorable"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waCompanionReg "go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

var whatsApp *whatsmeowClient

const (
	namePriorityUnknown uint8 = iota
	namePriorityHistory
	namePriorityPush
	namePriorityAddressBook
	namePriorityGroup
)

type whatsmeowEventMsg struct {
	Chat    Chat    `json:"chat"`
	Message message `json:"message"`
}

type resolvedName struct {
	name     string
	priority uint8
}

type whatsmeowClient struct {
	client          *whatsmeow.Client
	container       *sqlstore.Container
	history         *historyStore
	cmdChan         chan<- tea.Msg
	connectionReady chan struct{}

	mu           sync.RWMutex
	chats        map[string]Chat
	messages     map[string][]message
	messagesByID map[string]message
	contactNames map[string]resolvedName
	groupNames   map[string]resolvedName
}

func connectWhatsmeow(cmdChan chan<- tea.Msg, relink bool) (*whatsmeowClient, error) {
	ctx := context.Background()
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find user config directory: %w", err)
	}
	configDir = filepath.Join(configDir, "whats-cli")
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	history, err := openHistoryStore(configDir)
	if err != nil {
		return nil, fmt.Errorf("open chat history: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", filepath.ToSlash(filepath.Join(configDir, "session.db")))
	container, err := sqlstore.New(ctx, "sqlite", dsn, waLog.Noop)
	if err != nil {
		_ = history.Close()
		return nil, fmt.Errorf("open WhatsApp session: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		_ = container.Close()
		_ = history.Close()
		return nil, fmt.Errorf("load WhatsApp session: %w", err)
	}

	if relink {
		if device.ID != nil {
			if err = device.Delete(ctx); err != nil {
				_ = container.Close()
				_ = history.Close()
				return nil, fmt.Errorf("clear linked WhatsApp session: %w", err)
			}
		}
		if err = history.clear(); err != nil {
			_ = container.Close()
			_ = history.Close()
			return nil, fmt.Errorf("clear chat history: %w", err)
		}
		device, err = container.GetFirstDevice(ctx)
		if err != nil {
			_ = container.Close()
			_ = history.Close()
			return nil, fmt.Errorf("create WhatsApp session: %w", err)
		}
	} else if device.ID != nil {
		complete, syncErr := history.hasInitialSync()
		if syncErr != nil {
			_ = container.Close()
			_ = history.Close()
			return nil, fmt.Errorf("check chat history state: %w", syncErr)
		}
		if !complete {
			var hasChats bool
			hasChats, syncErr = history.hasChats()
			if syncErr != nil {
				_ = container.Close()
				_ = history.Close()
				return nil, fmt.Errorf("check persisted chats: %w", syncErr)
			}
			if hasChats {
				fmt.Fprintln(os.Stderr, "Warning: persisted chats predate full-history sync configuration; run with --relink once to import the complete recent conversation list.")
			} else {
				fmt.Fprintln(os.Stderr, "Warning: no synchronized chat list is stored yet; run with --relink to import recent WhatsApp chats.")
			}
		}
	}

	service := &whatsmeowClient{
		container:       container,
		history:         history,
		cmdChan:         cmdChan,
		connectionReady: make(chan struct{}, 1),
		chats:           make(map[string]Chat),
		messages:        make(map[string][]message),
		messagesByID:    make(map[string]message),
		contactNames:    make(map[string]resolvedName),
		groupNames:      make(map[string]resolvedName),
	}
	if err = history.load(service); err != nil {
		service.Close()
		return nil, fmt.Errorf("load chat history: %w", err)
	}
	if device.ID == nil {
		configureFullHistorySync()
	}
	service.client = whatsmeow.NewClient(device, waLog.Noop)
	service.client.AddEventHandler(service.handleEvent)

	if device.ID == nil {
		if err = service.loginWithQRCode(ctx); err != nil {
			service.Close()
			return nil, err
		}
	} else {
		fmt.Println("Connecting to WhatsApp...")
		if err = service.client.Connect(); err != nil {
			service.Close()
			return nil, fmt.Errorf("connect to WhatsApp: %w", err)
		}
		if !service.client.WaitForConnection(20 * time.Second) {
			service.Close()
			return nil, errors.New("timed out connecting to WhatsApp")
		}
	}

	service.loadContactsAndGroups(ctx)
	return service, nil
}

func configureFullHistorySync() {
	store.DeviceProps.Os = proto.String("whats-cli")
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_DESKTOP.Enum()
	store.DeviceProps.RequireFullSync = proto.Bool(true)

	config := store.DeviceProps.HistorySyncConfig
	if config == nil {
		config = &waCompanionReg.DeviceProps_HistorySyncConfig{}
		store.DeviceProps.HistorySyncConfig = config
	}
	config.FullSyncDaysLimit = proto.Uint32(3650)
	config.RecentSyncDaysLimit = proto.Uint32(365)
	config.FullSyncSizeMbLimit = proto.Uint32(10240)
	config.StorageQuotaMb = proto.Uint32(10240)
	config.InitialSyncMaxMessagesPerChat = proto.Uint32(100)
	config.InlineInitialPayloadInE2EeMsg = proto.Bool(true)
	config.SupportRecentSyncChunkMessageCountTuning = proto.Bool(true)
	config.SupportGroupHistory = proto.Bool(true)
}

func (wc *whatsmeowClient) loginWithQRCode(ctx context.Context) error {
	qrChan, err := wc.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("initialize QR login: %w", err)
	}
	if err = wc.client.Connect(); err != nil {
		return fmt.Errorf("connect for QR login: %w", err)
	}

	fmt.Println("Scan this QR code in WhatsApp: Settings > Linked devices > Link a device")
	for evt := range qrChan {
		switch evt.Event {
		case "code":
			if err = printQRCode(evt.Code); err != nil {
				return err
			}
		case "success":
			fmt.Println("WhatsApp linked successfully. Waiting for the authenticated connection...")
			select {
			case <-wc.connectionReady:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(60 * time.Second):
				return errors.New("timed out waiting for WhatsApp to reconnect after linking")
			}
		case "timeout":
			return errors.New("QR code expired; restart whats-cli to try again")
		default:
			fmt.Println("QR login status:", evt.Event)
		}
	}
	return errors.New("QR login ended before the device was linked")
}

func printQRCode(content string) error {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("render QR code: %w", err)
	}

	output := colorable.NewColorableStdout()
	for _, line := range terminalQRLines(qr.Bitmap()) {
		_, _ = fmt.Fprintf(output, "\x1b[30;47m%s\x1b[0m\n", line)
	}
	return nil
}

func terminalQRLines(bitmap [][]bool) []string {
	if len(bitmap) == 0 {
		return nil
	}

	// A terminal cell is approximately twice as tall as it is wide. Packing
	// two vertical modules into one cell keeps the QR code square while using
	// only the block characters supported by Windows raster fonts.
	lines := make([]string, 0, (len(bitmap)+1)/2)
	for y := 0; y < len(bitmap); y += 2 {
		line := make([]rune, len(bitmap[y]))
		for x, top := range bitmap[y] {
			bottom := y+1 < len(bitmap) && x < len(bitmap[y+1]) && bitmap[y+1][x]
			switch {
			case top && bottom:
				line[x] = '█'
			case top:
				line[x] = '▀'
			case bottom:
				line[x] = '▄'
			default:
				line[x] = ' '
			}
		}
		lines = append(lines, string(line))
	}
	return lines
}

func (wc *whatsmeowClient) Close() {
	if wc == nil {
		return
	}
	if wc.client != nil {
		wc.client.Disconnect()
	}
	if wc.container != nil {
		_ = wc.container.Close()
	}
	if wc.history != nil {
		_ = wc.history.Close()
	}
}

func (wc *whatsmeowClient) connected() error {
	if wc == nil || wc.client == nil || !wc.client.IsConnected() || !wc.client.IsLoggedIn() {
		return errors.New("not connected to WhatsApp")
	}
	return nil
}

func (wc *whatsmeowClient) loadContactsAndGroups(ctx context.Context) {
	wc.loadContacts(ctx)

	groups, err := wc.client.GetJoinedGroups(ctx)
	if err != nil {
		return
	}
	wc.mu.Lock()
	for _, group := range groups {
		resolved := resolvedName{name: group.Name, priority: namePriorityGroup}
		wc.groupNames[group.JID.String()] = resolved
		if _, exists := wc.chats[group.JID.String()]; exists {
			wc.upsertChatLocked(Chat{ID: group.JID.String(), Name: resolved.name, namePriority: resolved.priority, IsGroup: true})
		}
	}
	wc.mu.Unlock()
	wc.persistAllChats()
}

func (wc *whatsmeowClient) loadContacts(ctx context.Context) {
	if wc.client.Store == nil || wc.client.Store.Contacts == nil {
		return
	}
	contacts, err := wc.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return
	}
	resolvedContacts := make(map[string]resolvedName, len(contacts))
	for jid, contact := range contacts {
		if !isDirectContact(jid) {
			continue
		}
		name, priority := contactDisplayName(contact)
		resolved := resolvedName{name: name, priority: priority}
		mergeResolvedName(resolvedContacts, jid.String(), resolved)

		var alternate types.JID
		if wc.client.Store.LIDs != nil {
			if jid.Server == types.DefaultUserServer {
				alternate, _ = wc.client.Store.LIDs.GetLIDForPN(ctx, jid)
			} else {
				alternate, _ = wc.client.Store.LIDs.GetPNForLID(ctx, jid)
			}
		}
		if !alternate.IsEmpty() {
			mergeResolvedName(resolvedContacts, alternate.String(), resolved)
		}
	}

	wc.mu.Lock()
	for chatID, resolved := range resolvedContacts {
		wc.applyContactNameLocked(chatID, resolved)
	}
	wc.mu.Unlock()
	wc.persistAllChats()
}

func mergeResolvedName(names map[string]resolvedName, id string, incoming resolvedName) {
	existing, ok := names[id]
	if !ok || incoming.priority >= existing.priority {
		names[id] = incoming
	}
}

func (wc *whatsmeowClient) applyContactNameLocked(chatID string, resolved resolvedName) {
	mergeResolvedName(wc.contactNames, chatID, resolved)
	resolved = wc.contactNames[chatID]
	if _, exists := wc.chats[chatID]; exists {
		wc.upsertChatLocked(Chat{ID: chatID, Name: resolved.name, namePriority: resolved.priority})
	}
}

func (wc *whatsmeowClient) refreshContacts() {
	wc.loadContacts(context.Background())
	wc.sendToUI(chatsLoadedMsg(wc.getChats()))
}

func (wc *whatsmeowClient) updateGroupName(jid types.JID, name string, create bool) {
	if name == "" {
		return
	}
	wc.mu.Lock()
	resolved := resolvedName{name: name, priority: namePriorityGroup}
	wc.groupNames[jid.String()] = resolved
	if _, exists := wc.chats[jid.String()]; create || exists {
		wc.upsertChatLocked(Chat{ID: jid.String(), Name: name, namePriority: namePriorityGroup, IsGroup: true})
	}
	wc.mu.Unlock()
	wc.persistChat(jid.String())
	wc.sendToUI(chatsLoadedMsg(wc.getChats()))
}

func (wc *whatsmeowClient) persistAllChats() {
	if wc.history == nil {
		return
	}
	for _, chat := range wc.getChats() {
		if err := wc.history.saveChat(chat); err != nil {
			wc.sendToUI(fmt.Errorf("persist chat history: %w", err))
			return
		}
	}
}

func (wc *whatsmeowClient) persistChat(chatID string) {
	if wc.history == nil {
		return
	}
	wc.mu.RLock()
	chat, ok := wc.chats[chatID]
	wc.mu.RUnlock()
	if ok {
		if err := wc.history.saveChat(chat); err != nil {
			wc.sendToUI(fmt.Errorf("persist chat history: %w", err))
		}
	}
}

func (wc *whatsmeowClient) handleEvent(rawEvent any) {
	switch evt := rawEvent.(type) {
	case *events.Message:
		wc.handleMessage(evt)
	case *events.HistorySync:
		wc.handleHistorySync(evt)
	case *events.AppStateSyncComplete:
		go wc.refreshContacts()
	case *events.Contact:
		if !evt.FromFullSync {
			go wc.refreshContacts()
		}
	case *events.PushName, *events.BusinessName:
		go wc.refreshContacts()
	case *events.JoinedGroup:
		wc.updateGroupName(evt.JID, evt.Name, true)
	case *events.GroupInfo:
		if evt.Name != nil {
			wc.updateGroupName(evt.JID, evt.Name.Name, false)
		}
	case *events.Connected:
		select {
		case wc.connectionReady <- struct{}{}:
		default:
		}
		go func() {
			wc.loadContactsAndGroups(context.Background())
			wc.sendToUI(chatsLoadedMsg(wc.getChats()))
		}()
	case *events.LoggedOut:
		wc.sendToUI(updateFlashMsg{msg: "WhatsApp logged out: " + evt.Reason.String(), count: 6})
	}
}

func (wc *whatsmeowClient) handleMessage(evt *events.Message) {
	if evt == nil || evt.Message == nil {
		return
	}
	if protocol := evt.Message.GetProtocolMessage(); protocol != nil {
		if protocol.GetType() == waE2E.ProtocolMessage_REVOKE && protocol.GetKey() != nil {
			wc.revokeStoredMessage(evt.Info.Chat.String(), protocol.GetKey().GetID())
		}
		return
	}

	msg, ok := wc.messageFromEvent(evt.Info, evt.Message)
	if !ok {
		return
	}
	chat := wc.storeMessage(msg, !msg.FromMe)
	wc.sendToUI(whatsmeowEventMsg{Chat: chat, Message: msg})
}

func (wc *whatsmeowClient) handleHistorySync(evt *events.HistorySync) {
	if evt == nil || evt.Data == nil {
		return
	}
	for _, conversation := range evt.Data.GetConversations() {
		chatID := conversation.GetID()
		if chatID == "" {
			chatID = conversation.GetNewJID()
		}
		chatJID, err := types.ParseJID(chatID)
		if err != nil || chatID == "" {
			continue
		}

		isGroup := chatJID.Server == types.GroupServer
		name := conversation.GetName()
		priority := namePriorityHistory
		if name == "" {
			name = conversation.GetDisplayName()
		}
		if name == chatID || name == chatJID.User {
			name = ""
		}
		if name == "" && !isGroup {
			name, priority = wc.contactName(chatJID)
		}
		if name == "" {
			priority = namePriorityUnknown
		}
		wc.mu.Lock()
		wc.upsertChatLocked(Chat{
			ID:           chatID,
			Name:         name,
			namePriority: priority,
			UnreadCount:  int(conversation.GetUnreadCount()),
			IsGroup:      isGroup,
		})
		wc.mu.Unlock()
		wc.persistChat(chatID)

		for _, historyMessage := range conversation.GetMessages() {
			webMessage := historyMessage.GetMessage()
			if webMessage == nil {
				continue
			}
			parsed, err := wc.client.ParseWebMessage(chatJID, webMessage)
			if err != nil {
				continue
			}
			msg, ok := wc.messageFromEvent(parsed.Info, parsed.Message)
			if ok {
				wc.storeMessage(msg, false)
			}
		}
	}
	syncType := evt.Data.GetSyncType()
	if wc.history != nil && evt.Data.GetProgress() >= 100 && (syncType == waHistorySync.HistorySync_FULL || syncType == waHistorySync.HistorySync_RECENT) {
		if err := wc.history.markInitialSync(); err != nil {
			wc.sendToUI(fmt.Errorf("mark initial chat sync complete: %w", err))
		}
	}
	wc.sendToUI(chatsLoadedMsg(wc.getChats()))
}

func (wc *whatsmeowClient) messageFromEvent(info types.MessageInfo, raw *waE2E.Message) (message, bool) {
	if raw == nil || info.Chat.IsEmpty() {
		return message{}, false
	}
	msg := message{
		MsgID:      string(info.ID),
		ChatID:     info.Chat.String(),
		From:       info.Sender.String(),
		GroupFrom:  info.Sender.String(),
		FromMe:     info.IsFromMe,
		Timestamp:  info.Timestamp,
		RawMessage: raw,
	}
	if !info.SenderAlt.IsEmpty() {
		msg.GroupFromAlt = info.SenderAlt.String()
	}
	if info.PushName != "-" {
		msg.SenderName = info.PushName
	}

	var contextInfo *waE2E.ContextInfo
	switch {
	case raw.GetConversation() != "":
		msg.Type = "chat"
		msg.Body = raw.GetConversation()
	case raw.GetExtendedTextMessage() != nil:
		text := raw.GetExtendedTextMessage()
		msg.Type = "chat"
		msg.Body = text.GetText()
		contextInfo = text.GetContextInfo()
	case raw.GetImageMessage() != nil:
		media := raw.GetImageMessage()
		msg.Type = "image"
		msg.Body = media.GetCaption()
		msg.HasMedia = true
		contextInfo = media.GetContextInfo()
	case raw.GetVideoMessage() != nil:
		media := raw.GetVideoMessage()
		msg.Type = "video"
		msg.Body = media.GetCaption()
		msg.HasMedia = true
		contextInfo = media.GetContextInfo()
	case raw.GetAudioMessage() != nil:
		media := raw.GetAudioMessage()
		msg.Type = "audio"
		msg.HasMedia = true
		contextInfo = media.GetContextInfo()
	case raw.GetDocumentMessage() != nil:
		media := raw.GetDocumentMessage()
		msg.Type = "document"
		msg.Body = media.GetCaption()
		if msg.Body == "" {
			msg.Body = media.GetFileName()
		}
		msg.HasMedia = true
		contextInfo = media.GetContextInfo()
	case raw.GetStickerMessage() != nil:
		media := raw.GetStickerMessage()
		msg.Type = "sticker"
		msg.HasMedia = true
		contextInfo = media.GetContextInfo()
	default:
		return message{}, false
	}

	if contextInfo != nil {
		msg.IsResponse = contextInfo.GetStanzaID() != ""
		msg.ResponseToID = contextInfo.GetStanzaID()
		msg.IsForwarded = contextInfo.GetIsForwarded()
		msg.MentionedIDs = append([]string(nil), contextInfo.GetMentionedJID()...)
	}
	return msg, true
}
func (wc *whatsmeowClient) contactName(jid types.JID) (string, uint8) {
	wc.mu.RLock()
	resolved, ok := wc.contactNames[jid.String()]
	wc.mu.RUnlock()
	if ok {
		return resolved.name, resolved.priority
	}
	if wc.client != nil && wc.client.Store != nil && wc.client.Store.Contacts != nil {
		contact, err := wc.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.Found {
			return contactDisplayName(contact)
		}
	}
	return "", namePriorityUnknown
}

func isDirectContact(jid types.JID) bool {
	return jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer
}

func contactDisplayName(contact types.ContactInfo) (string, uint8) {
	if contact.FullName != "" {
		return contact.FullName, namePriorityAddressBook
	}
	if contact.FirstName != "" {
		return contact.FirstName, namePriorityAddressBook
	}
	if contact.PushName != "" {
		return contact.PushName, namePriorityPush
	}
	if contact.BusinessName != "" {
		return contact.BusinessName, namePriorityPush
	}
	return "", namePriorityUnknown
}

func (wc *whatsmeowClient) contactNameForIDs(ids ...string) (string, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()

	var best resolvedName
	for _, id := range ids {
		if resolved, ok := wc.contactNames[id]; ok && resolved.name != "" && resolved.priority >= best.priority {
			best = resolved
		}
	}
	return best.name, best.name != ""
}

func (wc *whatsmeowClient) storeMessage(msg message, unread bool) Chat {
	chatID := msg.ChatID
	wc.mu.Lock()

	_, exists := wc.messagesByID[msg.MsgID]
	if exists {
		for index := range wc.messages[chatID] {
			if wc.messages[chatID][index].MsgID == msg.MsgID {
				wc.messages[chatID][index] = msg
				break
			}
		}
	} else {
		wc.messages[chatID] = append(wc.messages[chatID], msg)
	}
	wc.messagesByID[msg.MsgID] = msg

	chat := wc.chats[chatID]
	chat.ID = chatID
	chat.IsGroup = strings.HasSuffix(chatID, "@g.us")
	if chat.Name == "" {
		var resolved resolvedName
		if chat.IsGroup {
			resolved = wc.groupNames[chatID]
		} else {
			resolved = wc.contactNames[chatID]
		}
		chat.Name = resolved.name
		chat.namePriority = resolved.priority
	}
	if !msg.Timestamp.Before(chat.LastMessage.Timestamp) {
		chat.LastMessage = msg
	}
	if unread && !exists {
		chat.UnreadCount++
	}
	wc.chats[chatID] = chat
	wc.mu.Unlock()

	if wc.history != nil {
		if err := wc.history.saveChat(chat); err != nil {
			wc.sendToUI(fmt.Errorf("persist chat history: %w", err))
		} else if err = wc.history.saveMessage(msg); err != nil {
			wc.sendToUI(fmt.Errorf("persist message history: %w", err))
		}
	}
	return chat
}

func (wc *whatsmeowClient) upsertChatLocked(chat Chat) {
	existing := wc.chats[chat.ID]
	if chat.Name == "" || chat.namePriority < existing.namePriority {
		chat.Name = existing.Name
		chat.namePriority = existing.namePriority
	}
	if chat.LastMessage.Timestamp.Before(existing.LastMessage.Timestamp) {
		chat.LastMessage = existing.LastMessage
	}
	if chat.UnreadCount < existing.UnreadCount {
		chat.UnreadCount = existing.UnreadCount
	}
	chat.IsGroup = chat.IsGroup || existing.IsGroup
	wc.chats[chat.ID] = chat
}

func (wc *whatsmeowClient) getChats() []Chat {
	wc.mu.RLock()
	chats := make([]Chat, 0, len(wc.chats))
	for _, chat := range wc.chats {
		chats = append(chats, chat)
	}
	wc.mu.RUnlock()
	sort.Slice(chats, func(i, j int) bool {
		left := chats[i].LastMessage.Timestamp
		right := chats[j].LastMessage.Timestamp
		if left.Equal(right) {
			return strings.ToLower(chats[i].Name) < strings.ToLower(chats[j].Name)
		}
		return left.After(right)
	})
	return chats
}

func (wc *whatsmeowClient) getMessages(chatID string) []message {
	wc.mu.RLock()
	messages := append([]message(nil), wc.messages[chatID]...)
	wc.mu.RUnlock()
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].Timestamp.Equal(messages[j].Timestamp) {
			return messages[i].MsgID < messages[j].MsgID
		}
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})
	return messages
}

func (wc *whatsmeowClient) sendText(ctx context.Context, chatID, text, responseToID string) error {
	if err := wc.connected(); err != nil {
		return err
	}
	jid, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	raw := &waE2E.Message{}
	if responseToID == "" {
		raw.Conversation = proto.String(text)
	} else {
		raw.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text:        proto.String(text),
			ContextInfo: wc.quoteContext(responseToID),
		}
	}
	response, err := wc.client.SendMessage(ctx, jid, raw)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	wc.storeOutgoing(chatID, response, raw)
	return nil
}

func (wc *whatsmeowClient) sendMedia(ctx context.Context, chatID, mediaPath, caption, responseToID string) error {
	if err := wc.connected(); err != nil {
		return err
	}
	data, err := os.ReadFile(mediaPath)
	if err != nil {
		return err
	}
	mimeType := http.DetectContentType(data)
	jid, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	mediaType := whatsmeow.MediaDocument
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		mediaType = whatsmeow.MediaImage
	case strings.HasPrefix(mimeType, "video/"):
		mediaType = whatsmeow.MediaVideo
	case strings.HasPrefix(mimeType, "audio/"):
		mediaType = whatsmeow.MediaAudio
	}
	upload, err := wc.client.Upload(ctx, data, mediaType)
	if err != nil {
		return fmt.Errorf("upload media: %w", err)
	}

	fileLength := upload.FileLength
	contextInfo := wc.quoteContext(responseToID)
	raw := &waE2E.Message{}
	switch mediaType {
	case whatsmeow.MediaImage:
		raw.ImageMessage = &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &fileLength,
			ContextInfo:   contextInfo,
		}
	case whatsmeow.MediaVideo:
		raw.VideoMessage = &waE2E.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &fileLength,
			ContextInfo:   contextInfo,
		}
	case whatsmeow.MediaAudio:
		raw.AudioMessage = &waE2E.AudioMessage{
			Mimetype:      proto.String(mimeType),
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &fileLength,
			ContextInfo:   contextInfo,
		}
	default:
		fileName := filepath.Base(mediaPath)
		raw.DocumentMessage = &waE2E.DocumentMessage{
			Caption:       proto.String(caption),
			FileName:      proto.String(fileName),
			Title:         proto.String(fileName),
			Mimetype:      proto.String(mimeType),
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &fileLength,
			ContextInfo:   contextInfo,
		}
	}

	response, err := wc.client.SendMessage(ctx, jid, raw)
	if err != nil {
		return fmt.Errorf("send media: %w", err)
	}
	wc.storeOutgoing(chatID, response, raw)
	return nil
}

func (wc *whatsmeowClient) storeOutgoing(chatID string, response whatsmeow.SendResponse, raw *waE2E.Message) {
	self := ""
	if wc.client.Store != nil && wc.client.Store.ID != nil {
		self = wc.client.Store.ID.String()
	}
	info := types.MessageInfo{
		MessageSource: types.MessageSource{IsFromMe: true},
		ID:            response.ID,
		Timestamp:     response.Timestamp,
	}
	if jid, err := types.ParseJID(chatID); err == nil {
		info.Chat = jid
	}
	if jid, err := types.ParseJID(self); err == nil {
		info.Sender = jid
	}
	msg, ok := wc.messageFromEvent(info, raw)
	if ok {
		wc.storeMessage(msg, false)
	}
}

func (wc *whatsmeowClient) quoteContext(messageID string) *waE2E.ContextInfo {
	if messageID == "" {
		return nil
	}
	wc.mu.RLock()
	quoted, ok := wc.messagesByID[messageID]
	wc.mu.RUnlock()
	if !ok || quoted.RawMessage == nil {
		return &waE2E.ContextInfo{StanzaID: proto.String(messageID)}
	}
	return &waE2E.ContextInfo{
		StanzaID:      proto.String(messageID),
		Participant:   proto.String(quoted.From),
		QuotedMessage: proto.Clone(quoted.RawMessage).(*waE2E.Message),
	}
}

func (wc *whatsmeowClient) deleteMessage(ctx context.Context, chatID, messageID string) error {
	if err := wc.connected(); err != nil {
		return err
	}
	jid, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}
	if _, err = wc.client.RevokeMessage(ctx, jid, types.MessageID(messageID)); err != nil {
		return err
	}
	wc.revokeStoredMessage(chatID, messageID)
	return nil
}

func (wc *whatsmeowClient) revokeStoredMessage(chatID, messageID string) {
	wc.mu.Lock()
	msg, ok := wc.messagesByID[messageID]
	if ok {
		msg.Type = "revoked"
		msg.Body = ""
		msg.HasMedia = false
		msg.RawMessage = nil
		wc.messagesByID[messageID] = msg
		messages := wc.messages[chatID]
		for index := range messages {
			if messages[index].MsgID == messageID {
				messages[index] = msg
				break
			}
		}
		wc.messages[chatID] = messages
	}
	chat := wc.chats[chatID]
	wc.mu.Unlock()
	if ok && wc.history != nil {
		if err := wc.history.saveChat(chat); err != nil {
			wc.sendToUI(fmt.Errorf("persist chat history: %w", err))
		} else if err = wc.history.saveMessage(msg); err != nil {
			wc.sendToUI(fmt.Errorf("persist message history: %w", err))
		}
	}
	if ok {
		wc.sendToUI(whatsmeowEventMsg{Chat: chat, Message: msg})
	}
}

func (wc *whatsmeowClient) forwardMessage(ctx context.Context, chatID, messageID string) error {
	if err := wc.connected(); err != nil {
		return err
	}
	jid, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}
	wc.mu.RLock()
	original, ok := wc.messagesByID[messageID]
	wc.mu.RUnlock()
	if !ok || original.RawMessage == nil {
		return errors.New("message is not available to forward")
	}
	forwarded, err := cloneAsForwarded(original.RawMessage)
	if err != nil {
		return err
	}
	response, err := wc.client.SendMessage(ctx, jid, forwarded)
	if err != nil {
		return fmt.Errorf("forward message: %w", err)
	}
	wc.storeOutgoing(chatID, response, forwarded)
	return nil
}

func cloneAsForwarded(original *waE2E.Message) (*waE2E.Message, error) {
	forwarded := proto.Clone(original).(*waE2E.Message)
	var contextInfo **waE2E.ContextInfo
	switch {
	case forwarded.GetConversation() != "":
		text := forwarded.GetConversation()
		forwarded.Conversation = nil
		forwarded.ExtendedTextMessage = &waE2E.ExtendedTextMessage{Text: proto.String(text)}
		contextInfo = &forwarded.ExtendedTextMessage.ContextInfo
	case forwarded.ExtendedTextMessage != nil:
		contextInfo = &forwarded.ExtendedTextMessage.ContextInfo
	case forwarded.ImageMessage != nil:
		contextInfo = &forwarded.ImageMessage.ContextInfo
	case forwarded.VideoMessage != nil:
		contextInfo = &forwarded.VideoMessage.ContextInfo
	case forwarded.AudioMessage != nil:
		contextInfo = &forwarded.AudioMessage.ContextInfo
	case forwarded.DocumentMessage != nil:
		contextInfo = &forwarded.DocumentMessage.ContextInfo
	case forwarded.StickerMessage != nil:
		contextInfo = &forwarded.StickerMessage.ContextInfo
	default:
		return nil, errors.New("unsupported message type for forwarding")
	}
	if *contextInfo == nil {
		*contextInfo = &waE2E.ContextInfo{}
	}
	score := (*contextInfo).GetForwardingScore() + 1
	(*contextInfo).IsForwarded = proto.Bool(true)
	(*contextInfo).ForwardingScore = proto.Uint32(score)
	return forwarded, nil
}

func (wc *whatsmeowClient) downloadMedia(ctx context.Context, messageID string) (string, error) {
	if err := wc.connected(); err != nil {
		return "", err
	}
	wc.mu.RLock()
	msg, ok := wc.messagesByID[messageID]
	wc.mu.RUnlock()
	if !ok || msg.RawMessage == nil {
		return "", errors.New("media message is no longer available")
	}

	var downloadable whatsmeow.DownloadableMessage
	mimeType := "application/octet-stream"
	fileName := messageID
	switch {
	case msg.RawMessage.GetImageMessage() != nil:
		downloadable = msg.RawMessage.GetImageMessage()
		mimeType = msg.RawMessage.GetImageMessage().GetMimetype()
	case msg.RawMessage.GetVideoMessage() != nil:
		downloadable = msg.RawMessage.GetVideoMessage()
		mimeType = msg.RawMessage.GetVideoMessage().GetMimetype()
	case msg.RawMessage.GetAudioMessage() != nil:
		downloadable = msg.RawMessage.GetAudioMessage()
		mimeType = msg.RawMessage.GetAudioMessage().GetMimetype()
	case msg.RawMessage.GetDocumentMessage() != nil:
		downloadable = msg.RawMessage.GetDocumentMessage()
		mimeType = msg.RawMessage.GetDocumentMessage().GetMimetype()
		if name := msg.RawMessage.GetDocumentMessage().GetFileName(); name != "" {
			fileName = filepath.Base(name)
		}
	case msg.RawMessage.GetStickerMessage() != nil:
		downloadable = msg.RawMessage.GetStickerMessage()
		mimeType = msg.RawMessage.GetStickerMessage().GetMimetype()
	default:
		return "", errors.New("message has no downloadable media")
	}

	if filepath.Ext(fileName) == "" {
		if extensions, _ := mime.ExtensionsByType(mimeType); len(extensions) > 0 {
			fileName += extensions[0]
		}
	}
	cacheDir := filepath.Join(os.TempDir(), "whats-cli-media")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(cacheDir, fileName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	data, err := wc.client.Download(ctx, downloadable)
	if err != nil {
		return "", fmt.Errorf("download media: %w", err)
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (wc *whatsmeowClient) sendToUI(msg tea.Msg) {
	if wc != nil && wc.cmdChan != nil {
		wc.cmdChan <- msg
	}
}
