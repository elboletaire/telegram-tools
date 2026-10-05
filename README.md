<!-- prettier-ignore -->
<div align="center">

# ttools

*Manage your Telegram channel posts from the terminal*

[![CI](https://img.shields.io/github/actions/workflow/status/elboletaire/telegram-tools/ci.yml?style=flat-square&label=CI)](https://github.com/elboletaire/telegram-tools/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/elboletaire/telegram-tools?style=flat-square)](https://github.com/elboletaire/telegram-tools/releases)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)

[Features](#features) • [Installation](#installation) • [Setup](#setup) • [Usage](#usage) • [Formatting](#formatting)

</div>

`ttools` (short for *Telegram tools*) sends messages, uploads media, edits and
browses posts in your Telegram channels, without third-party bots in the middle.
It talks to Telegram directly over MTProto (via [gotd/td](https://github.com/gotd/td)),
as your own user account or as a bot, so it fits both day-to-day use and CI
pipelines.

```bash
cat release-notes.md | ttools posts new --chat @mychannel
```

> [!NOTE]
> `ttools` is an unofficial tool built on the Telegram API. It is not affiliated
> with or endorsed by Telegram.

## Features

- **Text posts**: markdown or HTML formatting, batches of posts from one file,
  and automatic splitting of messages over Telegram's 4096-character limit.
- **Media uploads**: videos, photos and documents with captions, auto-detected
  thumbnails, captions from file names and grouped albums. Large files show a
  progress bar with ETA.
- **Editing**: replace the text, caption or media of an existing post, picking
  it by id or from an interactive list.
- **Browsing**: page through and search posts and chats in the terminal.
- **Dry runs**: preview exactly what would be sent, split and rendered, without
  connecting to Telegram.
- **Friendly to Telegram limits**: waits out `FLOOD_WAIT` responses and
  validates caption lengths before uploading.

## Installation

Download a prebuilt binary for Linux, macOS or Windows from the
[releases page](https://github.com/elboletaire/telegram-tools/releases), extract it
and put `ttools` in your `PATH`.

Or install it with Go 1.25+:

```bash
go install github.com/elboletaire/telegram-tools/cmd/ttools@latest
```

Or build it from source:

```bash
git clone https://github.com/elboletaire/telegram-tools.git
cd telegram-tools
make build
```

> [!TIP]
> Install [FFmpeg](https://ffmpeg.org/download.html) so `ttools` can use
> `ffprobe` to read video metadata. Without it, videos are uploaded as plain
> files instead of streamable videos.

## Setup

### 1. Get your API credentials

Log in at [my.telegram.org](https://my.telegram.org/apps), open
*API development tools* and create an application to get an `api_id` and
`api_hash`.

> [!IMPORTANT]
> Every user needs their own credentials. They identify *your* application to
> Telegram, so don't share them or reuse someone else's.

### 2. Choose how to log in

| | User account | Bot |
|---|---|---|
| Logs in as | You, with your phone number | A bot created with [@BotFather](https://t.me/botfather) |
| First run | Asks for the login code (and 2FA password) | Non-interactive |
| Can post to | Any channel where you can post | Channels where the bot is an admin |
| Browse posts and chats | Yes | No, Telegram doesn't allow it for bots |
| Caption limit | 1024 characters (2048 with Premium) | 1024 characters |

Either way the session is saved (by default in
`~/.local/share/ttools/session.json`), so you only log in once.

### 3. Create `~/.ttools.yaml`

```yaml
api:
  id: 123456
  hash: "0123456789abcdef0123456789abcdef"
session:
  phone: "+123456789"       # user account...
  # bot_token: "123456:ABC" # ...or a bot, not both
  password: ""              # optional 2FA password (not the login code)
defaults:
  chat: "@mychannel"        # or a numeric id such as -1001234567890
  thumb: ~/Pictures/thumb.jpg
```

Every setting can also be passed as a `TTOOLS_*` environment variable
(`TTOOLS_API_ID`, `TTOOLS_SESSION_BOT_TOKEN`…) or a global flag (`--api-id`,
`--bot-token`, `--chat`…). Use `--config` to load a different file.

> [!WARNING]
> Automating a user account is allowed, but it's bound by the
> [Telegram API Terms of Service](https://core.telegram.org/api/terms). Don't
> use it for spam or mass messaging: accounts that abuse the API can be
> limited or banned.

## Usage

**Send text posts**

```bash
ttools posts new "Hello **world**"
ttools posts new --message-file post.md
cat post.md | ttools posts new --chat -1001234567890
ttools posts new --html "<b>Hello</b> <tg-spoiler>world</tg-spoiler>"
```

**Preview before sending**

```bash
ttools posts new --message-file post.md --dry-run
```

**Upload media**

```bash
ttools posts new video.mp4 --message "My caption"
ttools posts new *.mp4 --autocaption   # caption from each file name
ttools posts new *.jpg --group         # a single album
```

**Edit posts**

```bash
ttools posts edit --post-id 123 "New text"
ttools posts edit --post-id 123 --file new-video.mp4
ttools posts edit --post-id 123 --clear-message
ttools posts edit --search "keyword"   # pick the post from a list
```

**Browse**

```bash
ttools posts list
ttools posts find "keyword" --video
ttools chats find --type broadcast --has-username
```

Run any command with `--help` to see all its options.

### Batches and long messages

- Separate several posts in one input with `[npost]` (or set your own with
  `--delimiter`). See [`examples/message.md`](examples/message.md).
- Messages over 4096 characters are split into several messages at paragraph
  breaks, then line breaks, then spaces, keeping the formatting intact. Edits
  can't be split, so an edit over the limit fails.
- `--dry-run` shows each message as it would be sent, with its length, and
  needs no credentials. Use `--color=always` or `--color=never` to override
  color detection (`NO_COLOR` is honoured).
- `--silent` sends without a notification and `--no-preview` disables link
  previews.

## Formatting

Messages use markdown by default:

| Syntax | Result |
|---|---|
| `**bold**` | **bold** |
| `_italic_` | *italic* |
| `` `code` `` | inline code |
| ` ```lang ` … ` ``` ` | code block |
| `[text](https://example.com)` | link |
| `> quote` | blockquote |
| `>> quote` | expandable blockquote |

Formatting can be nested, as in `**[bold link](https://example.com)**`.
Underscores inside words (`my_var`) and bare URLs are always left untouched.

Use `--html` for [Telegram-style HTML](https://core.telegram.org/bots/api#html-style),
which also supports underline, strikethrough and spoilers, or `--plain` to send
the text as is.

> [!NOTE]
> Captions of new uploads are currently sent as plain text, without parsing
> markdown or HTML.

## Development

```bash
make test       # run the tests
make fmt        # format the code
make build/all  # cross-compile into dist/
```

Releases are built with GoReleaser when a `v*` tag is pushed.
