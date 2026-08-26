<p align="center">
<pre>
 __       __  __                   __                 ______   __  __ 
│  ╲  _  │  ╲│  ╲                 │  ╲               ╱      ╲ │  ╲│  ╲
│ $$ ╱ ╲ │ $$│ $$____    ______  _│ $$_     _______ │  $$$$$$╲│ $$ ╲$$
│ $$╱  $╲│ $$│ $$    ╲  │      ╲│   $$ ╲   ╱       ╲│ $$   ╲$$│ $$│  ╲
│ $$  $$$╲ $$│ $$$$$$$╲  ╲$$$$$$╲╲$$$$$$  │  $$$$$$$│ $$      │ $$│ $$
│ $$ $$╲$$╲$$│ $$  │ $$ ╱      $$ │ $$ __  ╲$$    ╲ │ $$   __ │ $$│ $$
│ $$$$  ╲$$$$│ $$  │ $$│  $$$$$$$ │ $$│  ╲ _╲$$$$$$╲│ $$__╱  ╲│ $$│ $$
│ $$$    ╲$$$│ $$  │ $$ ╲$$    $$  ╲$$  $$│       $$ ╲$$    $$│ $$│ $$
 ╲$$      ╲$$ ╲$$   ╲$$  ╲$$$$$$$   ╲$$$$  ╲$$$$$$$   ╲$$$$$$  ╲$$ ╲$$
</pre>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-frontend-blue?logo=go" />
  <img src="https://img.shields.io/badge/Lua-scripting-lightgrey?logo=lua" />
  <img src="https://img.shields.io/badge/TypeScript-backend-blue?logo=typescript" />
  <img src="https://img.shields.io/badge/Docker-backend-blue?logo=docker" />
</p>

A command-line WhatsApp client written in Go and highly riceable with Lua scripts

---

# Screenshots Mural

A visual mural of all screenshots in the [`screenshots`](https://github.com/ArturCSegat/whats-cli/tree/master/screenshots) folder.  
<table>
  <tr>
    <td>
      <a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/default.jpeg">
        <img src="https://raw.githubusercontent.com/ArturCSegat/whats-cli/master/screenshots/default.jpeg" alt="default" width="180"/>
      </a><br/>
      <sub><a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/default.jpeg">default UI</a></sub>
    </td>
    <td>
      <a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/different_default.jpg">
        <img src="https://raw.githubusercontent.com/ArturCSegat/whats-cli/master/screenshots/different_default.jpg" alt="different_default" width="180"/>
      </a><br/>
      <sub><a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/different_default.jpg">default UI with custom colors</a></sub>
    </td>
    <td>
      <a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/different_old.jpg">
        <img src="https://raw.githubusercontent.com/ArturCSegat/whats-cli/master/screenshots/different_old.jpg" alt="different_old" width="180"/>
      </a><br/>
      <sub><a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/different_old.jpg">old school UI with custom colors</a></sub>
    </td>
  </tr>
  <tr>
    <td>
      <a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/discord_like.jpg">
        <img src="https://raw.githubusercontent.com/ArturCSegat/whats-cli/master/screenshots/discord_like.jpg" alt="discord_like" width="180"/>
      </a><br/>
      <sub><a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/discord_like.jpg">Discord like UI</a></sub>
    </td>
    <td>
      <a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/old.jpg">
        <img src="https://raw.githubusercontent.com/ArturCSegat/whats-cli/master/screenshots/old.jpg" alt="old" width="180"/>
      </a><br/>
      <sub><a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/old.jpg">old school UI with default colors</a></sub>
    </td>
    <td>
      <a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/tower.jpg">
        <img src="https://raw.githubusercontent.com/ArturCSegat/whats-cli/master/screenshots/tower.jpg" alt="tower" width="180"/>
      </a><br/>
      <sub><a href="https://github.com/ArturCSegat/whats-cli/blob/master/screenshots/tower.jpg">weird tower UI</a></sub>
    </td>
  </tr>
</table>

---
# Features

- **Sending messages** (Text, Audio, Images, Video, etc)
- **Opening Media** (uses the default application)
- **Forwarding, Deleting, Replying** messages
- **Custom keybinding** for native functionality 
- **Custom keybinding** with custom functionality 
- **Custom rendering** of messages for personalised UI
- **Custom hooks** run on message received 

---

# Installation Guide

## Prerequisites

Released binaries have no external service dependency. Building from source requires Go 1.25 or newer; Go will select the required toolchain automatically.

---

## Download the Latest Binary

Go to the [Releases page](https://github.com/ArturCSegat/whats-cli/releases/latest) and download the binary for your OS:

- Linux: `whats-cli-linux-***`
- macOS: `whats-cli-darwin-***`
- Windows: `whats-cli-windows-***.exe`

---

## Link WhatsApp

whats-cli embeds [whatsmeow](https://github.com/tulir/whatsmeow) and connects directly to WhatsApp. No HTTP backend, Docker container, webhook, Node.js process, or browser is required.

On the first run:

1. Start `whats-cli`.
2. Open WhatsApp on your phone.
3. Go to **Settings > Linked devices > Link a device**.
4. Scan the QR code shown in the terminal.

The linked-device session and synchronized chat history are restored automatically on later runs. Data is stored in the operating system's user configuration directory:

- Linux: `$XDG_CONFIG_HOME/whats-cli`, or `~/.config/whats-cli`
- macOS: `~/Library/Application Support/whats-cli`
- Windows: `%AppData%\\whats-cli`

`session.db` contains the linked-device credentials. `history.db` contains the chats and messages received through WhatsApp history sync and live events. Keep both private. Delete `session.db` only when you intentionally want to link the application again.

If the device was linked before local chat-history persistence was available, run `whats-cli --relink` once. This explicitly clears the old linked session and local history, shows a new QR code, and imports the recent conversation list from WhatsApp's initial history sync.

---

## Running whats-cli

1. **Rename the binary (optional):**
   ```bash
   mv whats-cli-linux-amd64 whats-cli
   ```

2. **Make the binary executable (Linux/macOS):**
   ```bash
   chmod +x whats-cli
   ```

3. **Run whats-cli:**
   ```bash
   ./whats-cli
   ```

# Configuration

Configuration is loaded from `./lua/init.lua` and `./lua/colors.lua` when a `lua` directory exists in the current working directory. This makes the repository's `lua` directory work with `go run .`. Otherwise, whats-cli uses the `lua` directory beside the executable and creates default scripts there when needed. Information on configuration is available in the [docs](https://github.com/ArturCSegat/whats-cli/tree/master/docs/configuration).

The repository's default Lua configuration follows BitchX's non-`HUMBLE` defaults from `include/color.h`: blue incoming and magenta outgoing public-message brackets, private-message labels, the standard 16-color ANSI palette, white-on-blue `STATUS_FORMAT`/`STATUS_FORMAT2` lines, the `[$C] ` input prompt, and the exact `/WINDOW LIST` column layout adapted to WhatsApp chats.

# Tips

## Insert media:
To anex media to your message type either `media:"/path/to/media"` or `media:clipboard` (to paste clipboard)
## Default message interaction binds:
- `m`-> Opens selected message's media
- `r`-> Quotes the selected message
- `f`-> Fowards the selected message
- `d`-> Deletes the selected message


