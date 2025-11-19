package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/telegram"
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
	opts := &postsListOptions{limit: 20}

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

			fmt.Fprintf(out, "Found %d posts in %s\n", len(posts), chat)
			for _, post := range posts {
				fmt.Fprintf(out, "#%d %s [%s] %s\n", post.ID, post.Date.Format(time.RFC3339), post.MediaType, post.Caption)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&opts.limit, "limit", "l", opts.limit, "Number of entries to fetch")
	cmd.Flags().StringVar(&opts.search, "search", "", "Filter posts containing this substring")

	return cmd
}

type postsListOptions struct {
	limit  int
	search string
}
