package cmd

import (
	"errors"
	"fmt"
	"time"

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
			if err := cfg.RequireAPI(); err != nil {
				return err
			}

			chat, err := cfg.ResolveChat("")
			if err != nil {
				return err
			}

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))

			if opts.postID == 0 {
				req := telegram.ListPostsRequest{
					ChatId: chat,
					Limit:  opts.limit,
					Search: opts.search,
					OnBatch: func(total int) {
						fmt.Fprintf(out, "\rLoading posts... %d fetched", total)
					},
					OnFloodWait: func(delay time.Duration, total int) {
						fmt.Fprintf(out, "\rHit rate limit, pausing %s after %d fetched...", delay.Round(time.Second), total)
					},
				}

				fmt.Fprintln(out, "Loading posts to select from...")
				posts, err := svc.ListPosts(cmd.Context(), req)
				if err != nil {
					return err
				}
				fmt.Fprintln(out)

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
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "New thumbnail for this media")
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
