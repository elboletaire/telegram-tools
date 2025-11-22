package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/ui/chatslist"
)

type chatsListOptions struct {
	search string
}

func newChatsListCommand(sharedOpts *sharedChatsOptions) *cobra.Command {
	opts := &chatsListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List chats and channels",
		Long: `List all chats and channels you're a member of.

Examples:
  # List all chats
  ttools chats list

  # List chats with search filter
  ttools chats list --search "my channel"

  # Limit to first 50 chats
  ttools chats list --limit 50

  # Adjust page size
  ttools chats list --page-size 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cfg, err := configFromContext(cmd)
			if err != nil {
				return err
			}
			if err := cfg.RequireAPI(); err != nil {
				return err
			}

			// Fetch chats using the shared helper
			req := fetchChatsRequest{
				Limit:  sharedOpts.limit,
				Search: opts.search,
			}

			chats, err := fetchChatsWithProgress(
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

			if len(chats) == 0 {
				fmt.Fprintln(out, "No chats found")
				return nil
			}

			fmt.Fprintf(out, "Loaded %d chats\n\n", len(chats))

			// Display using the TUI
			if err := chatslist.Display(cmd.InOrStdin(), out, opts.search, chats, sharedOpts.pageSize); err != nil {
				return fmt.Errorf("render list: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.search, "search", "", "Filter chats by name or username")

	return cmd
}
