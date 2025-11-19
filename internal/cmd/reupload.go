package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
)

func newReuploadCommand() *cobra.Command {
	opts := &reuploadOptions{}

	cmd := &cobra.Command{
		Use:   "reupload [file]",
		Short: "Replace the media of an existing channel post",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.postID == 0 {
				return errors.New("--post-id is required (use 'ttools posts list' to discover posts)")
			}

			cfg, err := configFromContext(cmd)
			if err != nil {
				return err
			}
			if err := cfg.RequireAPI(); err != nil {
				return err
			}

			channel, err := cfg.ResolveChannel("")
			if err != nil {
				return err
			}

			filePath, err := config.ExpandPath(args[0])
			if err != nil {
				return fmt.Errorf("expand file path: %w", err)
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

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
			return svc.ReplaceMedia(cmd.Context(), telegram.ReplaceRequest{
				Channel:   channel,
				PostID:    opts.postID,
				FilePath:  filePath,
				ThumbPath: thumb,
				Caption:   opts.caption,
				Silent:    opts.silent,
			})
		},
	}

	cmd.Flags().IntVar(&opts.postID, "post-id", 0, "Message identifier to replace")
	cmd.Flags().StringVar(&opts.caption, "caption", "", "Optional new caption")
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "New thumbnail for this media")
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Edit message silently if possible")

	return cmd
}

type reuploadOptions struct {
	postID  int
	caption string
	thumb   string
	silent  bool
}
