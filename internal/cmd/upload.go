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
		Use:   "upload [file]",
		Short: "Upload a file to a Telegram channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configFromContext(cmd)
			if err != nil {
				return err
			}
			if err := cfg.RequireAPI(); err != nil {
				return err
			}

			filePath, err := config.ExpandPath(args[0])
			if err != nil {
				return fmt.Errorf("expand file path: %w", err)
			}
			thumbProvided := cmd.Flags().Changed("thumb")
			autoThumbFound := false

			chat, err := cfg.ResolveChat("")
			if err != nil {
				return err
			}

			thumb := opts.thumb
			if !thumbProvided {
				thumb, autoThumbFound, err = findSiblingThumbnail(filePath)
				if err != nil {
					return err
				}
			}
			if thumb == "" {
				thumb = cfg.Defaults.Thumb
			}
			if thumb != "" {
				thumb, err = config.ExpandPath(thumb)
				if err != nil {
					return fmt.Errorf("expand thumb: %w", err)
				}
				if autoThumbFound {
					fmt.Fprintf(cmd.OutOrStdout(), "🖼️ Using detected thumbnail %s\n", filepath.Base(thumb))
				}
			}

			captionProvided := cmd.Flags().Changed("caption")

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
			return svc.Upload(cmd.Context(), telegram.UploadRequest{
				ChatId:     chat,
				FilePath:   filePath,
				ThumbPath:  thumb,
				Caption:    opts.caption,
				CaptionSet: captionProvided,
				Silent:     opts.silent,
			})
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
