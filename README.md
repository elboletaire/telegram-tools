# ttools

Command-line tool for managing Telegram channel posts: send messages, upload media, edit posts, and explore content—all without relying on third-party bots. The first run will prompt for the login code (and 2FA password when enabled) and save a session to disk so subsequent executions work non-interactively.

## Getting Started

### Installation

**Option 1: Download Binary** (Coming soon)
```bash
# Download the latest release for your platform
# Extract and move to your PATH
```

**Option 2: Build from Source**
```bash
git clone https://github.com/yourusername/ttools.git
cd ttools
go build -o ttools cmd/ttools/main.go
```

### Quick Setup

1. **Get Telegram API Credentials**
   - Visit [apps.telegram.org](https://my.telegram.org/apps)
   - Create an application to get your `api_id` and `api_hash`

2. **Create Configuration File**
   - Create `~/.ttools.yaml` with your credentials:

```yaml
api:
  id: 123456                                        # From apps.telegram.org
  hash: "0123456789abcdef0123456789abcdef"         # From apps.telegram.org
session:
  phone: "+123456789"                               # Your phone number
  password: ""                                      # Optional 2FA password (NOT login code)
defaults:
  chat: "@mychat"                                   # Your default channel/chat
```

3. **First Run**
   - Run any command (e.g., `ttools posts list`)
   - You'll be prompted for the login code sent to your Telegram
   - If you have 2FA enabled, you'll be prompted for your password
   - A session file is saved so you won't need to login again

## Configuration

`ttools` searches for `~/.ttools.yaml` by default (override with `--config`). Any value can also come from `TTOOLS_*` environment variables or CLI flags. The `defaults.chat` (or `--chat`) option accepts either an `@username` **or** a numeric chat ID such as `-1001234567890`. Example configuration:

```yaml
api:
  id: 123456                                        # From apps.telegram.org
  hash: "0123456789abcdef0123456789abcdef"         # From apps.telegram.org
session:
  file: ~/.local/share/ttools/session.json
  phone: "+123456789"                               # Your phone number
  password: ""                                      # Optional 2FA password (NOT login code)
defaults:
  chat: "@mychat"
  thumb: ~/Pictures/thumb.jpg
```

### Authentication Methods

**Option 1: User Authentication (Phone-based)**

1. Configure `api.id` and `api.hash` with your Telegram app credentials from [apps.telegram.org](https://my.telegram.org/apps).
2. Supply a `phone` number (either via config, env, or `--phone`). The CLI will prompt if it is omitted during the first login.
3. Optionally provide `session.password` for 2FA (two-factor authentication); otherwise the CLI prompts when Telegram requests it. **Note:** This is your 2FA password, NOT the temporary login code sent via SMS/Telegram.
4. The generated session is stored at `session.file` so that later runs can skip the login prompts.

**Option 2: Bot Authentication (Recommended for automation)**

Instead of authenticating as a user, you can authenticate as a bot:

1. Create a bot with [@BotFather](https://t.me/botfather)
2. Add the bot as an administrator to your channel
3. Configure the bot token instead of phone:

```yaml
api:
  id: 123456
  hash: "abc123def456"
session:
  bot_token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
defaults:
  chat: "@mychannel"
```

Or use environment variable:
```bash
export TTOOLS_SESSION_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
ttools posts list
```

Or CLI flag:
```bash
ttools --bot-token "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" posts list
```

**Note:** Bot authentication doesn't require phone numbers or 2FA, making it ideal for CI/CD and automation. The bot must be added as an admin to manage channels.

If you place the config elsewhere, run with `ttools --config /path/to/file posts new …`.

## CLI Usage

All post-related operations are now under the `posts` command:

```bash
# Send text messages
ttools posts new "Hello **world**"
ttools posts new --message-file message.md
echo "Text" | ttools posts new
cat message.md | ttools posts new --chat -123456789
cat message.html | ttools posts new --html
cat message.md | ttools posts new --dry-run  # Preview without sending

# Upload media with caption
ttools posts new --file video.mp4 --message "My caption"
ttools posts new --file video.mp4 --autocaption  # Caption from filename

# Edit existing posts
ttools posts edit --post-id 123 "New text"
ttools posts edit --post-id 123 --file new-video.mp4
ttools posts edit --search "keyword"  # Interactive selection

# List and find posts
ttools posts list --limit 50
ttools posts find --search "keyword"
```

### Common Flags

**Message/text input** (for `posts new` and `posts edit`):
- `--message "text"` - Direct message text or caption
- `--message-file path` - Read message from file
- Piped stdin - `echo "text" | ttools posts new`
- Arguments - `ttools posts new "text here"`

**Media upload** (for `posts new`):
- `--file path` - Media file to upload (can use multiple times)
- `--thumb path` - Custom thumbnail
- `--autocaption` - Auto-generate caption from filename
- `--autocaption-regex` - Regex for caption extraction

**Formatting**:
- `--html` - Use HTML format
- `--plain` - Plain text (no formatting)
- Default is Markdown

Messages longer than Telegram's 4096 character limit are split into several
messages when sending, cutting at paragraph breaks, then line breaks, then
spaces, so formatting is never broken. Edits can't be split, so an edit over
the limit fails instead.

**Other**:
- `--silent` - Send/edit without notification
- `--delimiter` - Message delimiter for batch sending (default: `[npost]`)
- `--dry-run` - Preview text messages (rendered, and split as they would be sent) without connecting to Telegram; `posts edit --dry-run` requires `--post-id`

### Global Flags

These apply to every command and can be stored in the config file:

```
--api-id int           Telegram API ID (from apps.telegram.org)
--api-hash string      Telegram API hash (from apps.telegram.org)
--phone string         Phone number for login (user auth)
--password string      2FA password (NOT login code, user auth only)
--bot-token string     Bot token from @BotFather (alternative to phone auth)
--session string       Path to the session file
--chat string          Default chat username or ID (e.g. @username or -1001234567890)
--thumb string         Default thumbnail for video uploads
```

## Requirements

### For Binary Users

`ffprobe` (part of FFmpeg) is recommended for reading video metadata so Telegram can display videos inline. Without it, uploads still work but videos are treated as generic files.
  - Install on macOS: `brew install ffmpeg`
  - Install on Ubuntu/Debian: `apt install ffmpeg`
  - Install on Windows: Download from [ffmpeg.org](https://ffmpeg.org/download.html)
	- "I'm on Arch, btw": `pacman -S extra/ffmpeg`

### For Building from Source

- Go 1.21+
- `ffprobe` (as above)

## Thanks

Built with [gotd/td](https://github.com/gotd/td) for MTProto communication with Telegram.
