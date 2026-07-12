# JuroBot

A Minecraft: Java Edition bot built with [go-mclib](https://github.com/go-mclib). Runs on GitHub Actions so it can stay online even when your PC is off.

## Features

- Auto armor, auto eat, auto totem
- Item sorting
- Chat translation (any language → English)
- Inventory viewer, player list
- Discord command support
- Role-based command permissions
- Cloudflare tunnel for remote web panel access
- Runs on GitHub Actions (up to 6 hours per run)

## Quick Start

### Download

Go to [Releases](../../releases) and download the binary for your platform:

| Platform | File |
|----------|------|
| Linux (x64) | `jurobot-linux-amd64` |
| Linux (ARM64) | `jurobot-linux-arm64` |
| Windows (x64) | `jurobot-windows-amd64.exe` |
| Windows (ARM64) | `jurobot-windows-arm64.exe` |
| macOS (Intel) | `jurobot-darwin-amd64` |
| macOS (Apple Silicon) | `jurobot-darwin-arm64` |

### Run

```bash
./jurobot -u YourUsername -s server.address:25565
```

The bot will guide you through Microsoft authentication on first run. Tokens are saved to `~/.config/jurobot/config.json`.

## Run on GitHub Actions

1. Fork this repo (or copy the `github-actions/` folder to a new repo)
2. Add your Microsoft account tokens as GitHub Secrets (see below)
3. Go to **Actions → Run Bot → Run workflow**
4. Select duration (1-6 hours) and click **Run workflow**

The bot checks if your local bot is already running and skips the cloud run if so.

### Required Secrets

| Secret | Description |
|--------|-------------|
| `SERVER_ADDRESS` | Minecraft server address |
| `SERVER_PORT` | Server port (default: 25565) |
| `BOT_USERNAME` | Your Minecraft username |
| `REFRESH_TOKEN` | Microsoft account refresh token |
| `ACCESS_TOKEN` | Xbox Live access token |
| `XBL_TOKEN` | Xbox Live token |
| `XBL_USER_HASH` | Xbox Live user hash |

### Getting Your Tokens

Tokens are stored by your Minecraft launcher. For Prism Launcher:

**Linux/macOS:**
```bash
cat ~/.local/share/PrismLauncher/accounts/accounts.json
```

**Windows:**
```
C:\Users\YOUR_USER\AppData\Roaming\PrismLauncher\accounts\accounts.json
```

## Commands

| Command | Permission | Description |
|---------|-----------|-------------|
| `*help` | Anyone | Show available commands |
| `*inv` | Anyone | Show inventory |
| `*list` | Anyone | Show online players |
| `*end` | Trusted+ | Reconnect the bot |
| `*pull` | Admin+ | Press nearest button |
| `*role` | Owner | Manage roles |
| `*ban` | Owner | Ban a user |
| `*unban` | Owner | Unban a user |

Commands work from both Minecraft chat and Discord.

## Building from Source

Requires Go 1.25+.

```bash
# Native build
go build -o jurobot .

# Cross-compile
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o jurobot-linux-amd64 .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o jurobot-windows-amd64.exe .
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o jurobot-darwin-arm64 .
```

## TODO

- [ ] Full Discord integration

## License

MIT
