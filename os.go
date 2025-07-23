package main

import (
	"fmt"
	"io/ioutil"
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
