package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestLuaDirForPrefersWorkingDirectoryConfig(t *testing.T) {
	root := t.TempDir()
	workingDir := filepath.Join(root, "source")
	executableDir := filepath.Join(root, "bin")
	workingLuaDir := filepath.Join(workingDir, "lua")
	if err := os.MkdirAll(workingLuaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := luaDirFor(workingDir, executableDir); got != workingLuaDir {
		t.Fatalf("Lua directory = %q, want working-directory config %q", got, workingLuaDir)
	}
}

func TestLuaDirForFallsBackToExecutableDirectory(t *testing.T) {
	root := t.TempDir()
	workingDir := filepath.Join(root, "source")
	executableDir := filepath.Join(root, "bin")

	want := filepath.Join(executableDir, "lua")
	if got := luaDirFor(workingDir, executableDir); got != want {
		t.Fatalf("Lua directory = %q, want executable-directory config %q", got, want)
	}
}

func TestRepositoryLuaConfigurationLoads(t *testing.T) {
	state := lua.NewState()
	defer state.Close()
	lua.OpenIo(state)
	lua.OpenOs(state)

	luaDir := luaDirFor(".", t.TempDir())
	if err := state.DoFile(filepath.Join(luaDir, "init.lua")); err != nil {
		t.Fatalf("load init.lua: %v", err)
	}
	if err := state.DoFile(filepath.Join(luaDir, "colors.lua")); err != nil {
		t.Fatalf("load colors.lua: %v", err)
	}

	status, handled, err := renderLua(state, "status", map[string]any{
		"width": 80, "height": 25, "message_count": 42, "in_input": true,
		"chat": map[string]any{"id": "123@g.us", "name": "Test Channel"},
	})
	if err != nil || !handled {
		t.Fatalf("render BitchX status: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(status, "[=You]") || !strings.Contains(status, "Test Channel") || !strings.Contains(status, "[Lag 0]") || blockHeight(status) != 2 {
		t.Fatalf("unexpected BitchX status: %q", status)
	}

	messageLine, handled, err := renderLua(state, "message", map[string]any{
		"message": map[string]any{"fromMe": false, "body": "hello from IRC"},
		"info":    map[string]any{"width": 80, "name": "Alice", "is_selected": false},
	})
	if err != nil || !handled {
		t.Fatalf("render BitchX message: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(messageLine, "Alice") || !strings.Contains(messageLine, "hello from IRC") {
		t.Fatalf("unexpected BitchX message: %q", messageLine)
	}

	publicIncoming, handled, err := renderLua(state, "message", map[string]any{
		"message": map[string]any{"fromMe": false, "body": "hello"},
		"info":    map[string]any{"width": 80, "name": "Alice", "is_group": true},
	})
	if err != nil || !handled || !strings.HasPrefix(publicIncoming, "\x1b[34m<\x1b[37mAlice\x1b[34m>") {
		t.Fatalf("incoming FORMAT_PUBLIC mismatch: %q, handled=%v err=%v", publicIncoming, handled, err)
	}
	publicOutgoing, handled, err := renderLua(state, "message", map[string]any{
		"message": map[string]any{"fromMe": true, "body": "hello"},
		"info":    map[string]any{"width": 80, "name": "You", "is_group": true},
	})
	if err != nil || !handled || !strings.HasPrefix(publicOutgoing, "\x1b[35m<\x1b[37mYou\x1b[35m>") {
		t.Fatalf("outgoing FORMAT_SEND_PUBLIC mismatch: %q, handled=%v err=%v", publicOutgoing, handled, err)
	}
	input, handled, err := renderLua(state, "input", map[string]any{
		"width": 80, "input": "draft",
		"chat": map[string]any{"name": "Test Channel"},
	})
	if err != nil || !handled || !strings.Contains(input, "[Test Channel] draft") {
		t.Fatalf("DEFAULT_INPUT_PROMPT mismatch: %q, handled=%v err=%v", input, handled, err)
	}

	setup_styles(state)
	application := &app{
		width:      80,
		height:     24,
		id_to_name: map[string]string{"alice@lid": "Alice"},
		luaState:   state,
	}
	container := &pageContainer{app: application}
	page := new_messages_page(Chat{ID: "test@g.us", Name: "Test Channel", IsGroup: true}, container)
	page.messages = []message{{
		MsgID:     "sample",
		ChatID:    "test@g.us",
		GroupFrom: "alice@lid",
		Body:      "hello from IRC",
		Timestamp: time.Unix(1_700_000_000, 0),
	}}
	view := page.View()
	if !strings.Contains(view, "[=You]") || !strings.Contains(view, "[Test Channel] ") || !strings.Contains(view, "hello from IRC") {
		t.Fatalf("BitchX message surface is incomplete: %q", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines != application.height {
		t.Fatalf("BitchX message surface height = %d, want %d", lines, application.height)
	}

	chatPage := new_chats_page(container)
	chatPage.chats = []Chat{{ID: "test@g.us", Name: "Test Channel", IsGroup: true}}
	chatView := chatPage.View()
	if !strings.Contains(chatView, "Ref") || !strings.Contains(chatView, "WhatsApp") || !strings.Contains(chatView, "Test Channel") {
		t.Fatalf("BitchX chat surface is incomplete: %q", chatView)
	}
	if lines := strings.Count(chatView, "\n") + 1; lines != application.height {
		t.Fatalf("BitchX chat surface height = %d, want %d", lines, application.height)
	}
}
