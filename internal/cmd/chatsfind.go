package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/ui/chatslist"
)

type chatsFindOptions struct {
	chatType      string
	unread        bool
	hasUsername   bool
	minMembers    int
	maxMembers    int
	activeSince   string
	inactiveSince string
}

func newChatsFindCommand(sharedOpts *sharedChatsOptions) *cobra.Command {
	opts := &chatsFindOptions{}

	cmd := &cobra.Command{
		Use:   "find [text]",
		Short: "Find chats by text, type, or other criteria",
		Long: `Find chats by searching in names/usernames and filtering by various criteria.

Examples:
  # Find chats containing "telegram" in name
  ttools chats find "telegram"

  # Find all broadcast channels
  ttools chats find --type broadcast

  # Find megagroups with unread messages
  ttools chats find --type megagroup --unread

  # Find large public channels (1000+ members)
  ttools chats find --type channel --has-username --min-members 1000

  # Find small private groups
  ttools chats find --type group --max-members 20

  # Find recently active broadcasts
  ttools chats find --type broadcast --active-since 24h

  # Find abandoned channels (no activity in 90 days)
  ttools chats find --inactive-since 90d

  # Combined: active megagroups with "crypto" in name
  ttools chats find "crypto" --type megagroup --active-since 7d

Chat types: broadcast, megagroup, channel, group
Duration format: 24h, 7d, 30d, 90d`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cfg, err := configFromContext(cmd)
			if err != nil {
				return err
			}
			if err := cfg.RequireAPI(); err != nil {
				return err
			}

			// Determine the search text
			searchText := ""
			if len(args) > 0 {
				searchText = args[0]
			}

			// Parse duration filters
			var activeSince, inactiveSince time.Duration
			if opts.activeSince != "" {
				activeSince, err = time.ParseDuration(opts.activeSince)
				if err != nil {
					return fmt.Errorf("invalid --active-since duration: %w", err)
				}
			}
			if opts.inactiveSince != "" {
				inactiveSince, err = time.ParseDuration(opts.inactiveSince)
				if err != nil {
					return fmt.Errorf("invalid --inactive-since duration: %w", err)
				}
			}

			// Fetch chats using the shared helper
			req := fetchChatsRequest{
				Limit:         sharedOpts.limit,
				Search:        searchText,
				TypeFilter:    opts.chatType,
				UnreadOnly:    opts.unread,
				HasUsername:   opts.hasUsername,
				MinMembers:    opts.minMembers,
				MaxMembers:    opts.maxMembers,
				ActiveSince:   activeSince,
				InactiveSince: inactiveSince,
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
				fmt.Fprintln(out, "No chats found matching the criteria")
				fmt.Fprintln(out, "\nTips to improve your search:")

				// Suggest increasing limit if it's set and relatively small
				if sharedOpts.limit > 0 && sharedOpts.limit < 500 {
					fmt.Fprintf(out, "  • Try increasing --limit (currently %d, try 500 or more for better coverage)\n", sharedOpts.limit)
				}

				// Suggest relaxing type filter if set
				if opts.chatType != "" {
					fmt.Fprintf(out, "  • Remove or change --type filter (currently: %s)\n", opts.chatType)
				}

				// Suggest using activity filter if not already used
				if opts.activeSince == "" && opts.inactiveSince == "" {
					fmt.Fprintln(out, "  • Use --active-since 30d to show only recently active chats")
				}

				// Suggest relaxing member filters if set
				if opts.minMembers > 0 || opts.maxMembers > 0 {
					fmt.Fprintln(out, "  • Try relaxing member count filters")
				}

				// Suggest removing username filter
				if opts.hasUsername {
					fmt.Fprintln(out, "  • Remove --has-username to include private chats")
				}

				return nil
			}

			fmt.Fprintf(out, "Found %d chats\n\n", len(chats))

			// Display using the same TUI as list
			if err := chatslist.Display(cmd.InOrStdin(), out, searchText, chats, sharedOpts.pageSize); err != nil {
				return fmt.Errorf("render list: %w", err)
			}
			return nil
		},
	}

	// Chat type filter
	cmd.Flags().StringVar(&opts.chatType, "type", "", "Filter by chat type (broadcast, megagroup, channel, group)")

	// Message filters
	cmd.Flags().BoolVar(&opts.unread, "unread", false, "Show only chats with unread messages")
	cmd.Flags().BoolVar(&opts.hasUsername, "has-username", false, "Show only public chats (with @username)")

	// Member count filters
	cmd.Flags().IntVar(&opts.minMembers, "min-members", 0, "Minimum member/subscriber count")
	cmd.Flags().IntVar(&opts.maxMembers, "max-members", 0, "Maximum member/subscriber count")

	// Activity filters
	cmd.Flags().StringVar(&opts.activeSince, "active-since", "", "Show chats active within duration (e.g., 24h, 7d, 30d)")
	cmd.Flags().StringVar(&opts.inactiveSince, "inactive-since", "", "Show chats inactive for duration (e.g., 90d)")

	return cmd
}
