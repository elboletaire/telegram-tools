package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
)

func newUploadCommand() *cobra.Command {
	opts := &uploadOptions{}

	cmd := &cobra.Command{
		Use:   "upload [file...]",
		Short: "Upload a file to a Telegram channel",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cfg, err := configFromContext(cmd)
			if err != nil {
				return err
			}
			if err := cfg.RequireAPI(); err != nil {
				return err
			}

			thumbProvided := cmd.Flags().Changed("thumb")
			captionProvided := cmd.Flags().Changed("caption")

			chat, err := cfg.ResolveChat("")
			if err != nil {
				return err
			}

			providedThumb := opts.thumb
			if thumbProvided && providedThumb != "" {
				providedThumb, err = config.ExpandPath(providedThumb)
				if err != nil {
					return fmt.Errorf("expand provided thumb: %w", err)
				}
			}

			defaultThumb := cfg.Defaults.Thumb
			if defaultThumb != "" {
				defaultThumb, err = config.ExpandPath(defaultThumb)
				if err != nil {
					return fmt.Errorf("expand default thumb: %w", err)
				}
			}

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
			for _, arg := range args {
				filePath, err := config.ExpandPath(arg)
				if err != nil {
					return fmt.Errorf("expand file path %q: %w", arg, err)
				}
				thumb := providedThumb
				autoThumbFound := false
				if !thumbProvided {
					thumb, autoThumbFound, err = findSiblingThumbnail(filePath)
					if err != nil {
						return err
					}
					if thumb == "" {
						thumb = defaultThumb
					}
				} else if thumb == "" {
					thumb = defaultThumb
				}
				if autoThumbFound {
					fmt.Fprintf(out, "🖼️ Using detected thumbnail %s\n", filepath.Base(thumb))
				}

				if err := svc.Upload(cmd.Context(), telegram.UploadRequest{
					ChatId:     chat,
					FilePath:   filePath,
					ThumbPath:  thumb,
					Caption:    opts.caption,
					CaptionSet: captionProvided,
					Silent:     opts.silent,
				}); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.caption, "caption", "", "Override caption text")
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "Custom thumbnail for this upload")
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Send message without notification")

	return cmd
}

type uploadOptions struct {
	caption string
	thumb   string
	silent  bool
}

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
