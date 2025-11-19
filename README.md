# ttools

Command-line helpers for uploading and reuploading media on Telegram channels without relying on third-party bots. The CLI is organized with Cobra and leans on Viper for configuration so that frequently reused options (API credentials, default channel, thumbnail paths, session location) live in a single file. MTProto calls are powered by [`gotd/td`](https://github.com/gotd/td).

## Status

Uploading, reuploading and listing posts now use a real Telegram client built on gotd. The first run will prompt for the login code (and 2FA password when enabled) and save a session to disk so subsequent executions work non-interactively.

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

`ttools` searches for `~/.ttools.yaml` by default (override with `--config`). Any value can also come from `TTOOLS_*` environment variables or CLI flags. The `defaults.channel` (or `--channel`) option accepts either an `@username` **or** a numeric chat ID such as `-1001234567890`. Example configuration:

```yaml
api:
  id: 123456
  hash: "0123456789abcdef0123456789abcdef"
  phone: "+123456789"
  password: ""        # optional 2FA password (leave empty to be prompted)
session:
  file: ~/.local/share/ttools/session.json
defaults:
  channel: "@mychannel"
  thumb: ~/Pictures/thumb.jpg
```

Authentication flow:

1. Configure `api.id` and `api.hash` with your Telegram app credentials.
2. Supply a `phone` number (either via config, env, or `--phone`). The CLI will prompt if it is omitted during the first login.
3. Optionally provide `api.password` for 2FA; otherwise the CLI prompts when Telegram requests it.
4. The generated session is stored at `session.file` so that later runs can skip the login prompts.

If you place the config elsewhere, run with `ttools --config /path/to/file upload …`.

## CLI usage

```
ttools upload [file]
    --caption string   Override caption text
    --silent           Send without notification
    --thumb string     Custom thumbnail for this upload (videos only)

ttools reupload [file]
    --post-id int      (required) Message identifier to replace
    --caption string   New caption (default keeps original once implemented)
    --silent           Edit silently when possible
    --thumb string     Override thumbnail

ttools posts list
    --limit int        Number of entries to fetch (default 20)
    --search string    Filter posts that contain this substring
```

Global flags apply to every command and can also be stored in the config file:

```
--api-id int           Telegram API ID
--api-hash string      Telegram API hash
--phone string         Phone number used for login
--session string       Path to the session file
    --channel string       Default channel username or chat ID (e.g. -1001234567890)
--thumb string         Default thumbnail for video uploads
```

## Next steps

1. Add video metadata detection/thumbnail logic (wrapping `ffprobe`/`ffmpeg` or a Go lib) so uploads send accurate duration, width, height, and preview.
2. Let `reupload` fall back to an interactive selection when `--post-id` is omitted, powered by the same listing helper.
3. Add tests around config loading, flag overrides, and Telegram interactions (mocking gotd) plus richer error-handling/logging.
