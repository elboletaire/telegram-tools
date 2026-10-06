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

// Telegram splits uploads into at most maxUploadParts parts, and every part
// size must divide maxUploadPartSize. gotd defaults to 128KB parts, which caps
// uploads at ~500MB: anything bigger sends an out-of-range file_total_parts and
// the server rejects the very first part with FILE_PARTS_INVALID.
//
// See https://core.telegram.org/api/files#uploading-files.
const (
	defaultUploadPartSize = 128 * 1024
	maxUploadPartSize     = 524288
	maxUploadParts        = 4000
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
	if info, statErr := os.Stat(req.FilePath); statErr == nil {
		size := info.Size()
		upload = upload.WithPartSize(uploadPartSize(size))
		if uploadParts(size, maxUploadPartSize) > maxUploadParts && s.io.err != nil {
			fmt.Fprintf(s.io.err, "warning: %s exceeds the ~2GB upload limit, Telegram may reject it\n", filepath.Base(req.FilePath))
		}
	}
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
	if meta.Kind == mediaKindUnknown {
		meta.Kind = guessKindFromMIME(meta.MIME)
	}

	// Only warn about ffprobe failures for files we expect to be media
	if probeErr != nil && s.io.err != nil {
		expectedMedia := strings.HasPrefix(strings.ToLower(meta.MIME), "video/") ||
			strings.HasPrefix(strings.ToLower(meta.MIME), "image/") ||
			meta.MIME == "image/gif"
		if expectedMedia {
			fmt.Fprintf(s.io.err, "warning: could not extract media metadata for %s, using basic detection\n", filepath.Base(req.FilePath))
		}
	}

	if meta.Kind == mediaKindPhoto && thumb != nil && s.io.err != nil {
		fmt.Fprintf(s.io.err, "warning: custom thumbnails are ignored for photos (%s)\n", req.FilePath)
	}
	return buildInputMedia(file, thumb, req.FilePath, meta), nil
}

// buildInputMedia turns an uploaded file into the input media to send,
// according to its detected kind.
func buildInputMedia(file, thumb tg.InputFileClass, path string, meta mediaMetadata) tg.InputMediaClass {
	switch meta.Kind {
	case mediaKindPhoto:
		return &tg.InputMediaUploadedPhoto{File: file}
	case mediaKindVideo:
		attrs := []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: filepath.Base(path)},
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
		// Without this flag Telegram turns silent videos into GIF animations
		// (which can't be part of albums). Real GIFs stay animations.
		if !meta.HasAudio && meta.MIME != "image/gif" {
			media.SetNosoundVideo(true)
		}
		if thumb != nil {
			media.Thumb = thumb
		}
		return media
	default:
		media := &tg.InputMediaUploadedDocument{
			File:     file,
			MimeType: meta.MIME,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: filepath.Base(path)},
			},
		}
		if thumb != nil {
			media.Thumb = thumb
		}
		return media
	}
}

// uploadPartSize returns the smallest valid part size that keeps a file of the
// given size within Telegram's part count limit.
func uploadPartSize(size int64) int {
	partSize := defaultUploadPartSize
	for partSize < maxUploadPartSize && uploadParts(size, partSize) > maxUploadParts {
		partSize *= 2
	}
	return partSize
}

func uploadParts(size int64, partSize int) int64 {
	if size <= 0 || partSize <= 0 {
		return 0
	}
	return (size + int64(partSize) - 1) / int64(partSize)
}

func analyzeMedia(ctx context.Context, path string) (mediaMetadata, error) {
	meta := mediaMetadata{}
	mimeType, err := detectMimeType(path)
	if err != nil {
		return meta, err
	}
	meta.MIME = mimeType

	// ffprobe reports still images as an mjpeg/png "video stream", so trust
	// the MIME type for them: they must be sent as photos.
	if kind := guessKindFromMIME(mimeType); kind == mediaKindPhoto {
		meta.Kind = kind
		return meta, nil
	}

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
