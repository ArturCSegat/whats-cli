package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

type historyStore struct {
	db *sql.DB
}

func openHistoryStore(configDir string) (*historyStore, error) {
	path := filepath.ToSlash(filepath.Join(configDir, "history.db"))
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	statements := []string{
		`CREATE TABLE IF NOT EXISTS chats (
			id TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			name_priority INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			data BLOB NOT NULL,
			raw_message BLOB,
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS messages_chat_id ON messages(chat_id)`,
	}
	for _, statement := range statements {
		if _, err = db.ExecContext(context.Background(), statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize history database: %w", err)
		}
	}
	return &historyStore{db: db}, nil
}

func (hs *historyStore) Close() error {
	if hs == nil || hs.db == nil {
		return nil
	}
	return hs.db.Close()
}

func (hs *historyStore) hasInitialSync() (bool, error) {
	var value string
	err := hs.db.QueryRowContext(context.Background(), `SELECT value FROM metadata WHERE key='full_sync_v1'`).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return value == "complete", err
}

func (hs *historyStore) hasChats() (bool, error) {
	var exists bool
	err := hs.db.QueryRowContext(context.Background(), `SELECT EXISTS(SELECT 1 FROM chats)`).Scan(&exists)
	return exists, err
}

func (hs *historyStore) markInitialSync() error {
	_, err := hs.db.ExecContext(
		context.Background(),
		`INSERT INTO metadata (key, value) VALUES ('full_sync_v1', 'complete')
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
	)
	return err
}

func (hs *historyStore) clear() error {
	tx, err := hs.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(context.Background(), `DELETE FROM messages`); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.ExecContext(context.Background(), `DELETE FROM chats`); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.ExecContext(context.Background(), `DELETE FROM metadata`); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (hs *historyStore) load(wc *whatsmeowClient) error {
	rows, err := hs.db.QueryContext(context.Background(), `SELECT data, name_priority FROM chats`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var data []byte
		var priority uint8
		if err = rows.Scan(&data, &priority); err != nil {
			_ = rows.Close()
			return err
		}
		var chat Chat
		if err = json.Unmarshal(data, &chat); err != nil {
			_ = rows.Close()
			return err
		}
		chat.namePriority = priority
		wc.chats[chat.ID] = chat
	}
	if err = rows.Close(); err != nil {
		return err
	}

	rows, err = hs.db.QueryContext(context.Background(), `SELECT chat_id, data, raw_message FROM messages`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var chatID string
		var data, raw []byte
		if err = rows.Scan(&chatID, &data, &raw); err != nil {
			return err
		}
		var msg message
		if err = json.Unmarshal(data, &msg); err != nil {
			return err
		}
		msg.ChatID = chatID
		if len(raw) > 0 {
			msg.RawMessage = &waE2E.Message{}
			if err = proto.Unmarshal(raw, msg.RawMessage); err != nil {
				return err
			}
		}
		wc.messages[chatID] = append(wc.messages[chatID], msg)
		wc.messagesByID[msg.MsgID] = msg
	}
	return rows.Err()
}

func (hs *historyStore) saveChat(chat Chat) error {
	if hs == nil {
		return nil
	}
	data, err := json.Marshal(chat)
	if err != nil {
		return err
	}
	_, err = hs.db.ExecContext(
		context.Background(),
		`INSERT INTO chats (id, data, name_priority) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data=excluded.data, name_priority=excluded.name_priority`,
		chat.ID, data, chat.namePriority,
	)
	return err
}

func (hs *historyStore) saveMessage(msg message) error {
	if hs == nil {
		return nil
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var raw []byte
	if msg.RawMessage != nil {
		raw, err = proto.Marshal(msg.RawMessage)
		if err != nil {
			return err
		}
	}
	_, err = hs.db.ExecContext(
		context.Background(),
		`INSERT INTO messages (id, chat_id, data, raw_message) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET chat_id=excluded.chat_id, data=excluded.data, raw_message=excluded.raw_message`,
		msg.MsgID, msg.ChatID, data, raw,
	)
	return err
}
