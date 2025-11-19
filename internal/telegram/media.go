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

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

type mediaRequest struct {
	FilePath  string
	ThumbPath string
}

func (s *Service) prepareDocumentMedia(ctx context.Context, api *tg.Client, req mediaRequest) (tg.InputMediaClass, error) {
	upload := uploader.NewUploader(api)

	file, err := upload.FromPath(ctx, req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("upload media: %w", err)
	}

	var thumb tg.InputFileClass
	if strings.TrimSpace(req.ThumbPath) != "" {
		thumb, err = upload.FromPath(ctx, req.ThumbPath)
		if err != nil {
			return nil, fmt.Errorf("upload thumbnail: %w", err)
		}
	}

	mimeType, err := detectMimeType(req.FilePath)
	if err != nil {
		return nil, err
	}

	media := &tg.InputMediaUploadedDocument{
		File:     file,
		MimeType: mimeType,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: filepath.Base(req.FilePath)},
		},
	}
	if thumb != nil {
		media.Thumb = thumb
	}
	return media, nil
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
