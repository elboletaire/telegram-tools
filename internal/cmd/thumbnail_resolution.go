package cmd

import (
	"fmt"
	"io"
	"path/filepath"

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
