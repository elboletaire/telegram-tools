package cmd

import (
	"fmt"

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
	opts := &postsListOptions{limit: 20, pageSize: 10}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channel posts to help with reuploads",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			}

			svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()))
			posts, err := svc.ListPosts(cmd.Context(), req)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(posts) == 0 {
				fmt.Fprintln(out, "No posts found")
				return nil
			}

			pageSize := opts.pageSize
			if err := postslist.Display(cmd.InOrStdin(), out, chat, opts.search, posts, pageSize); err != nil {
				return fmt.Errorf("render list: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&opts.limit, "limit", "l", opts.limit, "Number of entries to fetch")
	cmd.Flags().StringVar(&opts.search, "search", "", "Filter posts containing this substring")
	cmd.Flags().IntVar(&opts.pageSize, "page-size", opts.pageSize, "Entries per page in the viewer")

	return cmd
}

type postsListOptions struct {
	limit    int
	search   string
	pageSize int
}
