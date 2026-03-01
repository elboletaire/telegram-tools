package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
	"github.com/elboletaire/ttools/internal/ui/postslist"
)

func newReuploadCommand() *cobra.Command {
	opts := &reuploadOptions{
		limit:    0,
		pageSize: 8,
	}

	cmd := &cobra.Command{
		Use:   "reupload [file]",
		Short: "Replace the media of an existing channel post",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cfg, err := configFromContext(cmd)
			if err != nil {
				return err
			}
			if err := cfg.Requirements(); err != nil {
				return err
			}

			chat, err := cfg.ResolveChat("")
			if err != nil {
				return err
			}

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))

			if opts.postID == 0 {
				// Use shared helper to fetch posts
				req := fetchPostsRequest{
					ChatId: chat,
					Limit:  opts.limit,
					Search: opts.search,
				}

				posts, err := fetchPostsWithProgress(
					cmd.Context(),
					cfg,
					out,
					cmd.InOrStdin(),
					cmd.ErrOrStderr(),
					req,
				)
				if err != nil {
					return err
				}

				if len(posts) == 0 {
					return errors.New("no posts available to select")
				}

				fmt.Fprintf(out, "Loaded %d posts\n\n", len(posts))
				selected, ok, err := postslist.Select(cmd.InOrStdin(), out, chat, opts.search, posts, opts.pageSize)
				if err != nil {
					return fmt.Errorf("select post: %w", err)
				}
				if !ok {
					return errors.New("post selection cancelled")
				}
				opts.postID = selected.ID
			}

			filePath, err := config.ExpandPath(args[0])
			if err != nil {
				return fmt.Errorf("expand file path: %w", err)
			}

			thumb, autoThumbFound, err := resolveReplacementThumbnail(
				filePath,
				cmd.Flags().Changed("thumb"),
				opts.thumb,
				cfg.Defaults.Thumb,
			)
			if err != nil {
				return err
			}
			printDetectedThumbnail(out, thumb, autoThumbFound)

			captionProvided := cmd.Flags().Changed("caption")
			if captionProvided && opts.clearCaption {
				return errors.New("--caption and --clear-caption may not be used together")
			}

			return svc.ReplaceMedia(cmd.Context(), telegram.ReplaceRequest{
				ChatId:       chat,
				PostId:       opts.postID,
				FilePath:     filePath,
				ThumbPath:    thumb,
				Caption:      opts.caption,
				CaptionSet:   captionProvided || opts.clearCaption,
				ClearCaption: opts.clearCaption,
				Silent:       opts.silent,
			})
		},
	}

	cmd.Flags().IntVar(&opts.postID, "post-id", 0, "Message identifier to replace")
	cmd.Flags().StringVar(&opts.caption, "caption", "", "Optional new caption")
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "New thumbnail for this media (auto-detected from <file>-thumb.jpg/.jpeg if omitted)")
	cmd.Flags().BoolVar(&opts.clearCaption, "clear-caption", false, "Remove the caption entirely")
	cmd.Flags().BoolVar(&opts.clearCaption, "remove-caption", false, "Remove the caption entirely (alias)")
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Edit message silently if possible")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", opts.limit, "Number of entries to fetch for selection (0 = all)")
	cmd.Flags().StringVar(&opts.search, "search", "", "Filter selectable posts containing this substring")
	cmd.Flags().IntVar(&opts.pageSize, "page-size", opts.pageSize, "Entries per page in selector")

	return cmd
}

type reuploadOptions struct {
	postID       int
	caption      string
	thumb        string
	silent       bool
	clearCaption bool
	limit        int
	search       string
	pageSize     int
}
