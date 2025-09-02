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
- **Opening Media** (uses default browser)
- **Forwarding, Deleting, Replying** messages
- **Custom keybinding** for native functionality 
- **Custom keybinding** with custom functionality 
- **Custom rendering** of messages for personalised UI
- **Custom hooks** run on message received 

---

# Installation Guide

## 📦 Prerequisites

- **Node.js & npm/yarn** (if not using Docker) — [Download Node.js](https://nodejs.org/)
- **Docker** (if not using Node):  
  - [Windows](https://docs.docker.com/windows/started)
  - [macOS](https://docs.docker.com/mac/started/)
  - [Linux](https://docs.docker.com/linux/started/)

---

## Download the Latest Binary

Go to the [Releases page](https://github.com/ArturCSegat/whats-cli/releases/latest) and download the binary for your OS:

- Linux: `whats-cli-linux-***`
- macOS: `whats-cli-darwin-***`
- Windows: `whats-cli-windows-***.exe`

---

## Setting Up the whatshttp Backend

whats-cli communicates with whatshttp via HTTP. You **must** have whatshttp running before using whats-cli.

### 🐳 Option A: Run whatshttp with Docker

1. **Pull the Docker image:**
   ```bash
   docker pull arturcsegat/whatshttp:latest
   ```

2. **Run the container:**
   ```bash
   docker run -d \
     -p 3000:3000 \
     -v /home/artur/data:/app/data \
     -e PORT=3000 \
     arturcsegat/whatshttp:latest
   ```
   - Replace `/home/artur/data` with a persistent path on your machine.

---

### 🛠️ Option B: Build & Run whatshttp from Source (TypeScript)

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ArturCSegat/whatshttp.git
   cd whatshttp
   ```

2. **Install dependencies:**
   ```bash
   npm install
   ```

3. **Build the project:**  
   ```bash
   npm run build
   ```

4. **Run the server:**
     ```bash
     npm start
     ```
---

### 🔑 WhatsApp Session Initialization

> **Important:** For whats-cli to work, set:  
> - `clientId`: **1**  
> - `webHook`: `http://[your local ip]:4000/whatshttp/webhook`

#### Find your local IP:

- **Linux/macOS:**  
  `hostname -I | awk '{print $1}'`
- **Windows:**  
  Run `ipconfig` and look for your IPv4 address.

#### 💻 Generate the QR code for WhatsApp login

Open this URL in your browser (replace `[your local ip]`):

```
http://localhost:3000/client/qrCode?clientId=1&webHook=http://[your local ip]:4000/whatshttp/webhook
```

Scan the QR code with your WhatsApp app to link your account. Wait for session to be established.

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

### Configuration (Optional)

Configuration is done by editing the scripts in the lua folder that will be created in the same folder as the binary. Information on how to configure can be found in the [docs](https://github.com/ArturCSegat/whats-cli/tree/master/docs/configuration)

## 📚 Resources

- [whatshttp on GitHub](https://github.com/ArturCSegat/whatshttp)
- [whats-cli on GitHub](https://github.com/ArturCSegat/whats-cli)
- [Docker Hub for whatshttp](https://hub.docker.com/r/arturcsegat/whatshttp)

---

## 📄 License

This project is licensed under the MIT License.

---
