package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elboletaire/telegram-tools/internal/ui/postslist"
)

func newPostsListCommand(sharedOpts *sharedPostsOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channel posts to help with reuploads",
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

			// Fetch posts using the shared helper
			req := fetchPostsRequest{
				ChatId: chat,
				Limit:  sharedOpts.limit,
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
				fmt.Fprintln(out, "No posts found")
				return nil
			}

			fmt.Fprintf(out, "Loaded %d posts\n\n", len(posts))

			// Display using the TUI
			if err := postslist.Display(cmd.InOrStdin(), out, chat, "", posts, sharedOpts.pageSize); err != nil {
				return fmt.Errorf("render list: %w", err)
			}
			return nil
		},
	}

	return cmd
}
