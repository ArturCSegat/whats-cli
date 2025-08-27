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
  <img src="https://img.shields.io/badge/Go-81.8%25-blue?logo=go" />
  <img src="https://img.shields.io/badge/Lua-18.2%25-lightgrey?logo=lua" />
  <img src="https://img.shields.io/badge/TypeScript-backend-blue?logo=typescript" />
  <img src="https://img.shields.io/badge/Docker-ready-blue?logo=docker" />
</p>

A command-line WhatsApp client written in Go and highly riceable with Lua scripts

---

## 📥 Download Binaries (if not building from source)

Download the latest pre-built binaries for your OS from the [Releases page](https://github.com/ArturCSegat/whats-cli/releases/latest):

- [Linux (x86_64)](https://github.com/ArturCSegat/whats-cli/releases/latest/download/whats-cli-linux-amd64)
- [macOS (x86_64)](https://github.com/ArturCSegat/whats-cli/releases/latest/download/whats-cli-darwin-amd64)
- [Windows (x86_64)](https://github.com/ArturCSegat/whats-cli/releases/latest/download/whats-cli-windows-amd64.exe)

> See the [Installation Guide](#-installation-guide) for details.

---

# 🚀 whats-cli Installation Guide

Welcome! This guide will get you up and running with **whats-cli**—a command-line WhatsApp client—and its required backend, **whatshttp** (a TypeScript HTTP API for WhatsApp).

---

## 📦 Prerequisites

- **Go** (if not using binaries) — [Download Go](https://golang.org/dl/)
- **Node.js & npm/yarn** (if not using Docker) — [Download Node.js](https://nodejs.org/)
- **Docker** (if not using Node):  
  - [Windows](https://docs.docker.com/windows/started)
  - [macOS](https://docs.docker.com/mac/started/)
  - [Linux](https://docs.docker.com/linux/started/)

---

## 🏁 Quickstart

### 1. Download the Latest Binary

Go to the [Releases page](https://github.com/ArturCSegat/whats-cli/releases/latest) and download the binary for your OS:

- Linux: `whats-cli-linux-amd64`
- macOS: `whats-cli-darwin-amd64`
- Windows: `whats-cli-windows-amd64.exe`

---

## 1️⃣ Setting Up the whatshttp Backend

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
   # or
   yarn install
   ```

3. **Build the project:**  
   ```bash
   npm run build
   # or
   yarn build
   ```

4. **Run the server:**
     ```bash
     npm start
     # or
     yarn start
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
   ./whats-cli-linux
   ```

### Configuration (Optional)

Configuration is done by editing the scripts in the lua folder that will be created in the same folder as the binary. Information on how to configure can be found in the [docs](https://github.com/ArturCSegat/whats-cli/blob/tree/docs)

## 📚 Resources

- [whatshttp on GitHub](https://github.com/ArturCSegat/whatshttp)
- [whats-cli on GitHub](https://github.com/ArturCSegat/whats-cli)
- [Docker Hub for whatshttp](https://hub.docker.com/r/arturcsegat/whatshttp)

---

## 📄 License

This project is licensed under the MIT License.

---
