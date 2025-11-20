package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	ffprobe "github.com/vansante/go-ffprobe"
)

type mediaRequest struct {
	FilePath  string
	ThumbPath string
	Progress  uploader.Progress
}

type mediaKind int

const (
	mediaKindUnknown mediaKind = iota
	mediaKindPhoto
	mediaKindVideo
)

type mediaMetadata struct {
	Kind     mediaKind
	MIME     string
	Width    int
	Height   int
	Duration time.Duration
	HasAudio bool
}

func (s *Service) prepareMedia(ctx context.Context, api *tg.Client, req mediaRequest) (tg.InputMediaClass, error) {
	upload := uploader.NewUploader(api)
	if req.Progress != nil {
		upload = upload.WithProgress(req.Progress)
	}

	file, err := upload.FromPath(ctx, req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("upload media: %w", err)
	}

	var thumb tg.InputFileClass
	if strings.TrimSpace(req.ThumbPath) != "" {
		thumbUploader := uploader.NewUploader(api)
		thumb, err = thumbUploader.FromPath(ctx, req.ThumbPath)
		if err != nil {
			return nil, fmt.Errorf("upload thumbnail: %w", err)
		}
	}

	meta, probeErr := analyzeMedia(ctx, req.FilePath)
	if probeErr != nil && s.io.err != nil {
		fmt.Fprintf(s.io.err, "warning: ffprobe failed for %s, falling back to MIME detection (%v)\n", req.FilePath, probeErr)
	}
	if meta.Kind == mediaKindUnknown {
		meta.Kind = guessKindFromMIME(meta.MIME)
	}

	switch meta.Kind {
	case mediaKindPhoto:
		if thumb != nil && s.io.err != nil {
			fmt.Fprintf(s.io.err, "warning: custom thumbnails are ignored for photos (%s)\n", req.FilePath)
		}
		return &tg.InputMediaUploadedPhoto{File: file}, nil
	case mediaKindVideo:
		attrs := []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: filepath.Base(req.FilePath)},
		}
		videoAttr := &tg.DocumentAttributeVideo{
			Duration: meta.Duration.Seconds(),
			W:        meta.Width,
			H:        meta.Height,
		}
		videoAttr.SetSupportsStreaming(true)
		if !meta.HasAudio {
			videoAttr.SetNosound(true)
		}
		attrs = append(attrs, videoAttr)
		media := &tg.InputMediaUploadedDocument{
			File:       file,
			MimeType:   meta.MIME,
			Attributes: attrs,
		}
		if thumb != nil {
			media.Thumb = thumb
		}
		return media, nil
	default:
		media := &tg.InputMediaUploadedDocument{
			File:     file,
			MimeType: meta.MIME,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: filepath.Base(req.FilePath)},
			},
		}
		if thumb != nil {
			media.Thumb = thumb
		}
		return media, nil
	}
}

func analyzeMedia(ctx context.Context, path string) (mediaMetadata, error) {
	meta := mediaMetadata{}
	mimeType, err := detectMimeType(path)
	if err != nil {
		return meta, err
	}
	meta.MIME = mimeType

	data, err := ffprobe.GetProbeDataContext(ctx, path)
	if err != nil {
		meta.Kind = guessKindFromMIME(mimeType)
		return meta, err
	}

	if video := data.GetFirstVideoStream(); video != nil {
		meta.Kind = mediaKindVideo
		meta.Width = video.Width
		meta.Height = video.Height
		if data.Format != nil {
			meta.Duration = data.Format.Duration()
		}
		if audio := data.GetFirstAudioStream(); audio != nil {
			meta.HasAudio = true
		}
		return meta, nil
	}

	meta.Kind = guessKindFromMIME(mimeType)
	return meta, nil
}

func guessKindFromMIME(mimeType string) mediaKind {
	lower := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(lower, "video/"):
		return mediaKindVideo
	case lower == "image/gif":
		return mediaKindVideo
	case strings.HasPrefix(lower, "image/"):
		return mediaKindPhoto
	default:
		return mediaKindUnknown
	}
}

func detectMimeType(path string) (string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("open file for mime detection: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read file for mime detection: %w", err)
	}
	sample := buf[:n]
	mimeType := http.DetectContentType(sample)
	if mimeType == "application/octet-stream" || mimeType == "text/plain; charset=utf-8" {
		if ext := filepath.Ext(path); ext != "" {
			if byExt := mime.TypeByExtension(strings.ToLower(ext)); byExt != "" {
				mimeType = byExt
			}
		}
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType, nil
}

func extractMessageID(updates tg.UpdatesClass) (int, bool) {
	switch u := updates.(type) {
	case *tg.Updates:
		for _, update := range u.Updates {
			switch msg := update.(type) {
			case *tg.UpdateNewMessage:
				if concrete, ok := msg.Message.(*tg.Message); ok {
					return concrete.ID, true
				}
			case *tg.UpdateNewChannelMessage:
				if concrete, ok := msg.Message.(*tg.Message); ok {
					return concrete.ID, true
				}
			case *tg.UpdateEditMessage:
				if concrete, ok := msg.Message.(*tg.Message); ok {
					return concrete.ID, true
				}
			case *tg.UpdateEditChannelMessage:
				if concrete, ok := msg.Message.(*tg.Message); ok {
					return concrete.ID, true
				}
			}
		}
	case *tg.UpdatesCombined:
		return extractMessageID(&tg.Updates{Updates: u.Updates})
	case *tg.UpdateShortSentMessage:
		return u.ID, true
	case *tg.UpdateShortMessage:
		return u.ID, true
	case *tg.UpdateShort:
		if concrete, ok := u.Update.(*tg.UpdateNewMessage); ok {
			if msg, ok := concrete.Message.(*tg.Message); ok {
				return msg.ID, true
			}
		}
	}
	return 0, false
}
