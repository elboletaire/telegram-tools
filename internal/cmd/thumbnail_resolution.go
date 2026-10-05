package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/elboletaire/ttools/internal/config"
)

func resolveReplacementThumbnail(filePath string, thumbProvided bool, providedThumb string, defaultThumb string) (string, bool, error) {
	if thumbProvided && providedThumb != "" {
		expandedProvidedThumb, err := config.ExpandPath(providedThumb)
		if err != nil {
			return "", false, fmt.Errorf("expand provided thumb: %w", err)
		}
		providedThumb = expandedProvidedThumb
	}

	if defaultThumb != "" {
		expandedDefaultThumb, err := config.ExpandPath(defaultThumb)
		if err != nil {
			return "", false, fmt.Errorf("expand default thumb: %w", err)
		}
		defaultThumb = expandedDefaultThumb
	}

	thumb := providedThumb
	autoThumbFound := false
	if !thumbProvided {
		var err error
		thumb, autoThumbFound, err = findSiblingThumbnail(filePath)
		if err != nil {
			return "", false, err
		}
		if thumb == "" {
			thumb = defaultThumb
		}
	} else if thumb == "" {
		thumb = defaultThumb
	}

	return thumb, autoThumbFound, nil
}

func printDetectedThumbnail(out io.Writer, thumbPath string, autoDetected bool) {
	if autoDetected {
		fmt.Fprintf(out, "🖼️ Using detected thumbnail %s\n", filepath.Base(thumbPath))
	}
}

// findSiblingThumbnail looks for <name>-thumb.jpg or <name>-thumb.jpeg next to filePath.
func findSiblingThumbnail(filePath string) (string, bool, error) {
	dir := filepath.Dir(filePath)
	name := filepath.Base(filePath)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	candidates := []string{
		filepath.Join(dir, base+"-thumb.jpg"),
		filepath.Join(dir, base+"-thumb.jpeg"),
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		switch {
		case err == nil:
			return candidate, true, nil
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return "", false, fmt.Errorf("checking thumbnail %q: %w", candidate, err)
		}
	}
	return "", false, nil
}
