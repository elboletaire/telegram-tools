# ttools

Command-line tool for managing Telegram channel posts: send messages, upload media, edit posts, and explore content—all without relying on third-party bots. The CLI is organized with Cobra and leans on Viper for configuration so that frequently reused options (API credentials, default chat, thumbnail paths, session location) live in a single file. MTProto calls are powered by [`gotd/td`](https://github.com/gotd/td).

## Status

All core features (sending messages, uploading media, editing posts, listing/finding posts) use a real Telegram client built on gotd. The first run will prompt for the login code (and 2FA password when enabled) and save a session to disk so subsequent executions work non-interactively.

## Requirements

- Go 1.21+
- `ffprobe` available in your `$PATH` (part of FFmpeg) so `ttools` can read video metadata and send media inline. Without it uploads still work, but Telegram will treat videos as generic files.

## Project layout

```
cmd/ttools/main.go      # binary entrypoint
internal/cmd            # Cobra commands
internal/config         # config file/env helpers
internal/telegram       # Telegram service façade (to be implemented)
```

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

Authentication flow:

1. Configure `api.id` and `api.hash` with your Telegram app credentials from [apps.telegram.org](https://my.telegram.org/apps).
2. Supply a `phone` number (either via config, env, or `--phone`). The CLI will prompt if it is omitted during the first login.
3. Optionally provide `session.password` for 2FA (two-factor authentication); otherwise the CLI prompts when Telegram requests it. **Note:** This is your 2FA password, NOT the temporary login code sent via SMS/Telegram.
4. The generated session is stored at `session.file` so that later runs can skip the login prompts.

If you place the config elsewhere, run with `ttools --config /path/to/file posts new …`.

## CLI usage

All post-related operations are now under the `posts` command:

```bash
# Send text messages
ttools posts new "Hello **world**"
ttools posts new --message-file message.md
echo "Text" | ttools posts new

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

### Common flags

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
- Default is MarkdownV2

**Other**:
- `--silent` - Send/edit without notification
- `--delimiter` - Message delimiter for batch sending (default: `[npost]`)

### Global flags

These apply to every command and can be stored in the config file:

```
--api-id int           Telegram API ID (from apps.telegram.org)
--api-hash string      Telegram API hash (from apps.telegram.org)
--phone string         Phone number for login
--password string      2FA password (NOT login code)
--session string       Path to the session file
--chat string          Default chat username or ID (e.g. @username or -1001234567890)
--thumb string         Default thumbnail for video uploads
```

## Next steps

1. ✅ ~~Let `reupload` fall back to an interactive selection when `--post-id` is omitted~~ - Implemented in `posts edit`
2. ✅ ~~Consolidate upload commands under `posts`~~ - Now unified as `posts new` and `posts edit`
3. Implement text-only editing in `posts edit` (currently only supports media replacement)
4. Auto-generate thumbnails (via ffmpeg) when none are provided, so videos always have a preview
5. Add tests around config loading, flag overrides, and Telegram interactions (mocking gotd) plus richer error-handling/logging
