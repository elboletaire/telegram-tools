# ttools

Command-line tool for managing Telegram channel posts: send messages, upload
media, edit posts and explore content, without relying on third-party bots.

It talks to Telegram directly over MTProto (via [gotd/td](https://github.com/gotd/td)),
either as your own user account or as a bot.

## Installation

**Prebuilt binaries**: download the archive for your platform from the
[releases page](https://github.com/elboletaire/ttools/releases) (Linux and
macOS on amd64/arm64, Windows on amd64), extract it and put `ttools` in your
`PATH`.

**With Go** (1.25+):

```bash
go install github.com/elboletaire/ttools/cmd/ttools@latest
```

**From source**:

```bash
git clone https://github.com/elboletaire/ttools.git
cd ttools
make build        # produces ./ttools
```

`ffprobe` (part of [FFmpeg](https://ffmpeg.org/download.html)) is recommended:
it reads video metadata so Telegram plays videos inline. Without it, videos are
uploaded as generic files (`brew install ffmpeg`, `apt install ffmpeg`,
`pacman -S ffmpeg`…).

## Setup

### 1. Get your own API credentials

Every user needs their own `api_id` and `api_hash`: log in at
[my.telegram.org](https://my.telegram.org/apps), open *API development tools*
and create an application. They identify *your* app to Telegram, so don't share
them or reuse someone else's.

### 2. Pick an authentication mode

| | User account (MTProto) | Bot |
|---|---|---|
| Logs in as | You, with your phone number | A bot from [@BotFather](https://t.me/botfather) |
| First run | Asks for the login code (and 2FA password if enabled) | Non-interactive |
| Can post to | Any channel where you can post | Channels where the bot is an admin |
| Browsing (`posts list/find`, `chats`, interactive selector) | Yes | No, Telegram doesn't let bots read history or dialogs |
| Caption limit | 1024 chars (2048 with Premium) | 1024 chars |

Either way the session is saved (by default to
`~/.local/share/ttools/session.json`), so later runs don't log in again.

### 3. Create `~/.ttools.yaml`

```yaml
api:
  id: 123456
  hash: "0123456789abcdef0123456789abcdef"
session:
  phone: "+123456789"   # user account; omit if using bot_token
  password: ""          # optional 2FA password (NOT the login code)
  # bot_token: "123456:ABC-DEF..."   # bot mode instead of phone
defaults:
  chat: "@mychannel"    # or a numeric id such as -1001234567890
  thumb: ~/Pictures/thumb.jpg
```

Configure either `phone` or `bot_token`, not both. Every value can also come
from a `TTOOLS_*` environment variable (`TTOOLS_API_ID`,
`TTOOLS_SESSION_BOT_TOKEN`…) or a global flag (`--api-id`, `--bot-token`,
`--chat`…; see `ttools --help`). Use `--config` to load another file.

> **About Telegram's terms.** Using the API from a user account is allowed,
> but you're bound by the [Telegram API Terms of Service](https://core.telegram.org/api/terms):
> don't use it for spam, mass messaging or flooding. Accounts that abuse the
> API can be limited or banned. `ttools` respects `FLOOD_WAIT` responses and
> waits before retrying.

## Usage

```bash
# Send text
ttools posts new "Hello **world**"
ttools posts new --message-file post.md
cat post.md | ttools posts new --chat -1001234567890
ttools posts new --html "<b>Hello</b> <tg-spoiler>world</tg-spoiler>"

# Preview what would be sent (rendered and split), without connecting
cat post.md | ttools posts new --dry-run

# Upload media
ttools posts new video.mp4 --message "My caption"
ttools posts new --file a.mp4 --file b.mp4 --autocaption

# Edit posts: text, media or both
ttools posts edit --post-id 123 "New text"
ttools posts edit --post-id 123 --message-file post.md --dry-run
ttools posts edit --post-id 123 --file new-video.mp4
ttools posts edit --post-id 123 --clear-message
ttools posts edit --search "keyword"     # pick the post interactively

# Browse
ttools posts list --limit 50
ttools posts find --search "keyword"
ttools chats list
```

Run any command with `--help` for all its flags.

### Formatting

Messages are parsed as markdown by default. Use `--html` for
[Telegram-style HTML](https://core.telegram.org/bots/api#html-style)
(`<b>`, `<i>`, `<u>`, `<s>`, `<tg-spoiler>`, `<a href>`, `<code>`, `<pre>`,
`<blockquote>`…), or `--plain` to send the text untouched.

The markdown subset:

| Syntax | Result |
|---|---|
| `**bold**` | **bold** |
| `_italic_` | *italic* (only at word boundaries, so `my_var` is left alone) |
| `` `code` `` | inline code |
| ` ```lang` … ` ``` ` | code block |
| `[text](https://url)` | link |
| `> quote` | blockquote |
| `>> quote` | expandable blockquote |

Bare URLs are never altered. Formatting can be nested
(`**[bold link](https://example.com)**`). Media captions on upload are
currently sent as plain text.

### Long messages, batches and `--dry-run`

- Messages over Telegram's 4096-character limit are **split** into several
  messages, cutting at paragraph breaks, then line breaks, then spaces, so
  formatting is kept. Edits can't be split: an edit over the limit fails.
- One input can hold several posts separated by `[npost]` (change it with
  `--delimiter`); see [`examples/message.md`](examples/message.md).
- `--dry-run` shows each message exactly as it would be sent, with its length,
  and never connects to Telegram (so it needs no credentials).
  `posts edit --dry-run` requires `--post-id`. Use `--color=always|never` to
  force or disable colors (default `auto`, which honours `NO_COLOR`).
- Captions over 1024 characters print a warning (they need Premium); over 2048
  (or 1024 for bots) the upload is refused before it starts.
- `--no-preview` disables link previews and `--silent` sends without a
  notification.

## Development

```bash
make test     # go test ./...
make fmt
make build/all   # cross-compile into dist/
```

Releases are built by GoReleaser when a `v*` tag is pushed.

## License

[MIT](LICENSE)
