# AGENTS.md

Guidance for coding agents (and humans) working on this repository.

## Project

`ttools` is a CLI to manage Telegram channel posts (send, upload, edit,
browse) without third-party bots. It talks MTProto through
[gotd/td](https://github.com/gotd/td), authenticating either as a user account
(phone + login code + optional 2FA) or as a bot (token from @BotFather).

## Commands

```bash
make build                 # ./ttools for the current platform
make run ARGS="posts list --chat @mychannel"
make test                  # go test ./...
go test ./internal/telegram -run TestParseMarkdownV2
make fmt
make build/all             # cross-compile into dist/
```

CI (`.github/workflows/ci.yml`) requires `gofmt -l .` to be empty, `go vet ./...`
and `go test ./...` to pass. Releases are built by GoReleaser
(`.goreleaser.yaml`) when a `v*` tag is pushed.

## Layout

- `cmd/ttools/` – entrypoint. Keep it here so `go install .../cmd/ttools`
  always produces a binary named `ttools`. The version is injected with
  `-ldflags "-X main.version=..."`.
- `internal/config` – Viper config. Priority: flags → `TTOOLS_*` env vars →
  YAML file (`~/.ttools.yaml`). Session at `$XDG_DATA_HOME/ttools/session.json`.
  The `Config` travels through the cobra context (`config.ContextWith` /
  `config.FromContext`).
- `internal/cmd` – cobra commands (`posts new|edit|list|find`, `chats list|find`).
  The root `PersistentPreRunE` loads config; commands get it with
  `configFromContext(cmd)`.
- `internal/telegram` – `Service` wrapping the gotd client:
  - `service.go`: client lifecycle (`Service.run`), auth, chat resolution
    (`resolveChat`, accepts `@username` or Bot API ids like `-100…`), flood-wait
    retries (`callWithFloodRetry`).
  - `peercache.go`: on-disk peer cache so numeric ids don't need a dialog scan.
  - `markdown.go`: markdown → entities (`ParseMarkdownV2`). `html.go`: HTML →
    entities via gotd's HTML parser (`ParseHTML`).
  - `split.go`: `PrepareMessage` parses (MarkdownV2 / HTML / plain) and splits
    at Telegram's 4096 UTF-16 unit limit. Both sending and `--dry-run` go
    through it; keep it that way.
  - `caption.go`: captions are parsed like messages (`prepareCaption`) and their
    parsed length checked (1024, 2048 with Premium, bots 1024).
  - `album.go`: `--group` uploads (`messages.uploadMedia` + `sendMultiMedia`,
    up to 10 files per album).
  - `preview.go`: terminal rendering used by `--dry-run`.
  - `media.go`: uploads, ffprobe metadata, thumbnails.
- `internal/ui` – bubbletea list/selector UIs and progress displays.
- `internal/floodwait` – animated FLOOD_WAIT countdown.

## Conventions

- Use TDD for behaviour changes: write the failing test first.
- Commits follow Conventional Commits (`feat(posts): …`, `fix(markdown): …`).
- Errors: lowercase, no trailing punctuation, wrapped with context
  (`fmt.Errorf("read message file: %w", err)`).
- Config struct tags use `mapstructure`, not `json`.
- Entity offsets/lengths are UTF-16 code units, never bytes (`utf16Len`).
- New commands: add a file in `internal/cmd`, an options struct for its flags,
  get config via `configFromContext`, create `telegram.NewService(cfg,
  telegram.WithIO(...))`, and register it in the parent command.
- Don't commit real chat ids, phone numbers, API hashes or bot tokens, not
  even in tests. Use obviously fake values (`-1001111111111`, `123456`).

## Gotchas

- Bot API channel ids are `-100<channel_id>`; MTProto uses the bare id.
- Bots can't read history or list dialogs, so browsing commands need a user
  account.
- The markdown parser only treats `_` as italic at word boundaries and never
  touches bare URLs; keep regression tests in `markdown_test.go` green.
- `--autocaption` captions are always plain text (file names aren't markup).
- Progress UIs only render on a TTY; otherwise they fall back to plain logs.
- Message input priority is `--message-file`, `--message`, arguments, then
  piped stdin (`readMessageInput`); stdin is never read when another source
  is given, so cron/CI runs don't block.
- Bots can't resolve numeric chat ids (no dialog access): they need `@username`.
