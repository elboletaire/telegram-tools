package cmd

import (
	"fmt"

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

			chat, err := cfg.ResolveChat("")
			if err != nil {
				return err
			}

			thumb := opts.thumb
			if thumb == "" {
				thumb = cfg.Defaults.Thumb
			}
			if thumb != "" {
				thumb, err = config.ExpandPath(thumb)
				if err != nil {
					return fmt.Errorf("expand thumb: %w", err)
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
