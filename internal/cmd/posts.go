package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/telegram"
	"github.com/elboletaire/ttools/internal/ui/postslist"
)

func newPostsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "posts",
		Short: "Explore channel posts",
	}

	cmd.AddCommand(newPostsListCommand())
	return cmd
}

func newPostsListCommand() *cobra.Command {
	opts := &postsListOptions{limit: 0, pageSize: 10}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channel posts to help with reuploads",
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

			fmt.Fprintln(out, "Loading posts...")
			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
			posts, err := svc.ListPosts(cmd.Context(), req)
			if err != nil {
				return err
			}
			fmt.Fprintln(out)

			if len(posts) == 0 {
				fmt.Fprintln(out, "No posts found")
				return nil
			}

			fmt.Fprintf(out, "Loaded %d posts\n\n", len(posts))

			pageSize := opts.pageSize
			if err := postslist.Display(cmd.InOrStdin(), out, chat, opts.search, posts, pageSize); err != nil {
				return fmt.Errorf("render list: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&opts.limit, "limit", "l", opts.limit, "Number of entries to fetch (0 = all)")
	cmd.Flags().StringVar(&opts.search, "search", "", "Filter posts containing this substring")
	cmd.Flags().IntVar(&opts.pageSize, "page-size", opts.pageSize, "Entries per page in the viewer")

	return cmd
}

type postsListOptions struct {
	limit    int
	search   string
	pageSize int
}
