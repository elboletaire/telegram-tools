package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/telegram-tools/internal/config"
	"github.com/elboletaire/telegram-tools/internal/floodwait"
	"github.com/elboletaire/telegram-tools/internal/telegram"
)

// sharedChatsOptions holds flags that are shared across all chats subcommands
type sharedChatsOptions struct {
	limit    int
	pageSize int
}

func newChatsCommand() *cobra.Command {
	sharedOpts := &sharedChatsOptions{
		limit:    0,
		pageSize: 10,
	}

	cmd := &cobra.Command{
		Use:   "chats",
		Short: "List and manage chats and channels",
	}

	// Shared flags available to all subcommands
	cmd.PersistentFlags().IntVarP(&sharedOpts.limit, "limit", "l", sharedOpts.limit, "Number of chats to fetch from API (0 = all)")
	cmd.PersistentFlags().IntVar(&sharedOpts.pageSize, "page-size", sharedOpts.pageSize, "Entries per page in the viewer")

	cmd.AddCommand(newChatsListCommand(sharedOpts))
	cmd.AddCommand(newChatsFindCommand(sharedOpts))

	return cmd
}

// fetchChatsRequest holds the parameters for fetching and filtering chats
type fetchChatsRequest struct {
	Limit         int
	Search        string
	TypeFilter    string
	UnreadOnly    bool
	HasUsername   bool
	MinMembers    int
	MaxMembers    int
	ActiveSince   time.Duration
	InactiveSince time.Duration
}

// fetchChatsWithProgress fetches chats from Telegram with progress feedback
// This is shared logic used by chats subcommands
func fetchChatsWithProgress(ctx context.Context, cfg *config.Config, out io.Writer, in io.Reader, errOut io.Writer, req fetchChatsRequest) ([]telegram.ChatInfo, error) {
	listReq := telegram.ListChatsRequest{
		Limit:  req.Limit,
		Search: req.Search,
		OnBatch: func(total int) {
			fmt.Fprintf(errOut, "\rLoading chats... %d fetched", total)
		},
		OnFloodWait: func(delay time.Duration, total int) {
			floodwait.Start(ctx, errOut, delay, func(remaining time.Duration) string {
				return fmt.Sprintf("\rHit rate limit, retrying in %.2fs after %d fetched...", remaining.Seconds(), total)
			})
		},
	}

	fmt.Fprintln(errOut, "Loading chats...")
	svc := telegram.NewService(cfg, telegram.WithIO(in, out, errOut))
	chats, err := svc.ListChats(ctx, listReq)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(errOut)

	// Apply additional filters if specified
	if hasAdditionalFilters(req) {
		chats = filterChats(chats, req)
	}

	return chats, nil
}

// hasAdditionalFilters checks if any additional filters are set
func hasAdditionalFilters(req fetchChatsRequest) bool {
	return req.TypeFilter != "" ||
		req.UnreadOnly ||
		req.HasUsername ||
		req.MinMembers > 0 ||
		req.MaxMembers > 0 ||
		req.ActiveSince > 0 ||
		req.InactiveSince > 0
}

// filterChats filters chats by various criteria
func filterChats(chats []telegram.ChatInfo, req fetchChatsRequest) []telegram.ChatInfo {
	now := time.Now()
	filtered := make([]telegram.ChatInfo, 0, len(chats))

	for _, chat := range chats {
		// Type filter
		if req.TypeFilter != "" && chat.Type != req.TypeFilter {
			continue
		}

		// Unread filter
		if req.UnreadOnly && chat.Unread == 0 {
			continue
		}

		// Username filter
		if req.HasUsername && chat.Username == "" {
			continue
		}

		// Member count filters
		if req.MinMembers > 0 && chat.Participants < req.MinMembers {
			continue
		}
		if req.MaxMembers > 0 && chat.Participants > req.MaxMembers {
			continue
		}

		// Activity filters
		if req.ActiveSince > 0 {
			activeThreshold := now.Add(-req.ActiveSince)
			if chat.LastDate.IsZero() || chat.LastDate.Before(activeThreshold) {
				continue
			}
		}
		if req.InactiveSince > 0 {
			inactiveThreshold := now.Add(-req.InactiveSince)
			if !chat.LastDate.IsZero() && chat.LastDate.After(inactiveThreshold) {
				continue
			}
		}

		filtered = append(filtered, chat)
	}

	return filtered
}
