package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
)

const defaultAutoCaptionRegex = "^[^-]*-\\s*(.*)$"

func newUploadCommand() *cobra.Command {
	opts := &uploadOptions{
		autoCaptionRegex: defaultAutoCaptionRegex,
	}

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
			if err := cfg.Requirements(); err != nil {
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

			var autoCaptionRe *regexp.Regexp
			if opts.autoCaption {
				autoCaptionRe, err = regexp.Compile(opts.autoCaptionRegex)
				if err != nil {
					return fmt.Errorf("compile autocaption regex: %w", err)
				}
			}

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
			for i, arg := range args {
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

				caption := opts.caption
				captionSet := captionProvided
				usedAutoCaption := false
				if opts.autoCaption && autoCaptionRe != nil {
					name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
					rg := autoCaptionRe.FindStringSubmatch(name)
					if len(rg) > 1 {
						caption = strings.TrimSpace(rg[1])
					} else {
						caption = name
					}
					captionSet = true
					usedAutoCaption = true
				}
				logAutoCaption(cmd.ErrOrStderr(), usedAutoCaption, caption)

				if err := svc.Upload(cmd.Context(), telegram.UploadRequest{
					ChatId:     chat,
					FilePath:   filePath,
					ThumbPath:  thumb,
					Caption:    caption,
					CaptionSet: captionSet,
					Silent:     opts.silent,
				}); err != nil {
					return err
				}

				if i < len(args)-1 {
					select {
					case <-time.After(1100 * time.Millisecond):
					case <-cmd.Context().Done():
						return cmd.Context().Err()
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.caption, "caption", "", "Override caption text")
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "Custom thumbnail for this upload")
	cmd.Flags().BoolVar(&opts.autoCaption, "autocaption", false, "Set caption from file name using a regex")
	cmd.Flags().StringVar(&opts.autoCaptionRegex, "autocaption-regex", opts.autoCaptionRegex, "Regex to capture caption from file name (uses first group)")
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Send message without notification")

	return cmd
}

type uploadOptions struct {
	caption          string
	thumb            string
	silent           bool
	autoCaption      bool
	autoCaptionRegex string
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
