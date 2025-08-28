package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"
)

func setTerminalTitle(title string) {
	if runtime.GOOS == "windows" {
		_ = exec.Command("cmd", "/c", "title "+title).Run()
	} else {
		fmt.Printf("\033]0;%s\007", title)
	}
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

func ensureLuaPath() (string, error) {
	// Ensure the Lua scripts directory exists on the same folder as the binary (lua/init.lua)
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	luaDir := filepath.Join(exeDir, "lua")

	if err := os.MkdirAll(luaDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lua directory: %w", err)
	}

	initLuaPath := filepath.Join(luaDir, "init.lua")
	if _, err := os.Stat(initLuaPath); os.IsNotExist(err) {
		if err := os.WriteFile(initLuaPath, []byte(defaultInitLua), 0644); err != nil {
			return "", fmt.Errorf("failed to write default init.lua: %w", err)
		}
	}

	colorsLuaPath := filepath.Join(luaDir, "colors.lua")
	if _, err := os.Stat(colorsLuaPath); os.IsNotExist(err) {
		if err := os.WriteFile(colorsLuaPath, []byte(defaultColorsLua), 0644); err != nil {
			return "", fmt.Errorf("failed to write default colors.lua: %w", err)
		}
	}

	return luaDir, nil
}


var defaultInitLua = `
message_keybinds = {
	["up"] = function() scroll_up() end,
	["down"] = function() scroll_down() end,
	["ctrl+c"] = function() quit() end,
	["esc"] = function() escape() end,
	["enter"] = function()
		if in_input() then
			submit_input()
		else
			jump_to_quoted()
		end
	end,
	["backspace"] = function() backspace_input() end,
	["r"] = function() toggle_reply() end,
	["m"] = function() open_media() end,
	["f"] = function() forward_selected() end,
	["d"] = function() delete_selected() end,
}

chat_keybinds = {
	["up"] = function() chat_scroll_up() end,
	["down"] = function() chat_scroll_down() end,
	["k"] = function() chat_scroll_up() end,
	["j"] = function() chat_scroll_down() end,
	["ctrl+c"] = function() chat_escape() end,
	["esc"] = function() chat_escape() end,
	["enter"] = function() chat_select() end,
}

renders = {
	["message"] = function(msg_table)
		local msg = msg_table["message"]
		local info = msg_table["info"] or {}
		local fromMe = msg["fromMe"]
		local styles = styles or {}
		local termWidth = tonumber(info["width"]) or 80
		local headerSpace = tonumber(info["header_height"]) or 2

		-- Format timestamp
		local iso = tostring(msg["timestamp"] or "")
		local y, m, d, h, min = iso:match("(%d+)%-(%d+)%-(%d+)T(%d+):(%d+)")
		local timestamp = string.format("%s:%s - %s/%s", h or "??", min or "??", m or "??", d or "??")

		-- Prepare message body
		local body = tostring(msg["body"] or "")
		if msg["hasMedia"] then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[MEDIA]" .. reset() .. "\n" .. body
		elseif msg["type"] == "revoked" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[DELETED]" .. reset()
		elseif msg["type"] == "ciphertext" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[VIS ONCE]" .. reset()
		end

		local allowed_types = {
			chat = true,
			image = true,
			video = true,
			audio = true,
			voice = true,
			document = true,
			sticker = true,
			contact = true,
			revoked = true,
			ciphertext = true
		}
		if not allowed_types[msg["type"]] then
			body = "msg of type(" .. tostring(msg['type']) .. ") is not properly displayed"
		end

		-- Split into lines and find max width
		local lines, width = {}, 0
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
			local l = #strip_ansi(line)
			if l > width then width = l end
		end

		local function style_line(line, color)
			return color and fg(color) .. line .. reset() or line
		end

		-- Build bubble
		local bubble = {}
		table.insert(bubble, "┌" .. string.rep("─", width + 2) .. "┐")
		for _, line in ipairs(lines) do
			local pad = width - #strip_ansi(line)
			table.insert(bubble, "│ " .. line .. string.rep(" ", pad) .. " │")
		end
		table.insert(bubble, "└" .. string.rep("─", width + 2) .. "┘")

		-- Style for sender
		if fromMe and styles.selfBody and styles.selfBody.fg then
			for i = 1, #bubble do bubble[i] = style_line(bubble[i], styles.selfBody.fg) end
		end
		if info["is_selected"] then
			for i = 1, #bubble do
				bubble[i] = "\27[30;47m" .. strip_ansi(bubble[i]) .. "\27[0m"
			end
		end

		-- Tail
		local tail = fromMe and "╰─▶" or "◀─╯"
		if fromMe and styles.selfBody and styles.selfBody.fg then
			tail = style_line(tail, styles.selfBody.fg)
		end
		table.insert(bubble, tail)

		-- Name + timestamp
		local name = tostring(info["name"] or "")
		if name ~= "" then
			local nt = name .. "  " .. timestamp
			if fromMe then
				nt = string.rep(" ", math.max(0, termWidth - #nt)) .. nt
			end
			if fromMe and styles.selfBody and styles.selfBody.fg then
				nt = style_line(nt, styles.selfBody.fg)
			end
			table.insert(bubble, 1, nt)
		end

		-- Header space
		for _ = 1, headerSpace do table.insert(bubble, 1, "") end

		-- Bubble alignment
		if fromMe then
			local pad = math.max(0, termWidth - (width + 4))
			for i = headerSpace + 2, #bubble do
				bubble[i] = string.rep(" ", pad) .. bubble[i]
			end
		end

		return table.concat(bubble, "\n")
	end,

	["chat"] = function(tbl)
		if tbl['info']['is_selected'] then
			return fg(styles.selectedStyle.fg) .. "> " .. tbl['chat']['name'] .. reset() .. "\n"
		end
		return fg(styles.unselectedStyle.fg) .. "  " .. tbl['chat']['name'] .. reset() .. "\n"
	end
}
`


var defaultColorsLua = `
function strip_ansi(str)
  -- Remove ANSI escape sequences
  str = str:gsub('\27%[[%d;?]*[ -/]*[@-~]', '') -- CSI sequences
  str = str:gsub('\27%][^%a]*%a', '')           -- OSC sequences (simplified)
  str = str:gsub('\27%]%d+;.-\7', '')           -- OSC terminated by BEL
  str = str:gsub('\27%]%d+;.-\27\\', '')        -- OSC terminated by ST (ESC\)
  str = str:gsub('\27[PX^_].-\27\\', '')        -- DCS, SOS, PM, APC sequences
  return str
end



local function hex_to_rgb(hex)
	local r, g, b = hex:match("#?(%x%x)(%x%x)(%x%x)")
	return tonumber(r, 16), tonumber(g, 16), tonumber(b, 16)
end

function fg(hex)
	local r, g, b = hex_to_rgb(hex)
	return ("\27[38;2;%d;%d;%dm"):format(r, g, b)
end

function bg(hex)
	local r, g, b = hex_to_rgb(hex)
	return ("\27[48;2;%d;%d;%dm"):format(r, g, b)
end

function reset()
	return "\27[0m"
end

function fg_rgb(r, g, b)
	return ("\27[38;2;%d;%d;%dm"):format(r, g, b)
end

function bg_rgb(r,g,b)
	return ("\27[48;2;%d;%d;%dm"):format(r, g, b)
end

function invert_colors_of_text(text)
	return "\27[7m" .. text .. "\27[0m"
end



styles = {
  selectedStyle = {
    fg = "#008000",
    bold = true
  },

  unselectedStyle = {
    fg = "#808080" -- was "8"
  },

  hyperlink = {
    fg = "#FFFFFF",
    bg = "#0000FF"
  },

  selfPrefix = {
    fg = "#00FF00" -- was "10"
  },

  selfBody = {
    fg = "#7FFF7F"
  },

  topbarStyke = {
    fg = "#FFFFFF", -- was "15"
    bg = "#808080", -- was "8"
    bold = true
  },

  bottombarStyle = {
    fg = "#FFFFFF", -- was "15"
    bg = "#808080" -- was "8"
  },

  replyHighlight = {
    fg = "#000000",
    bg = "#FFFFFF"
  },

  errorBarStyle = {
    fg = "#FFFFFF",
    bg = "#FF0000"
  }
}--
`
