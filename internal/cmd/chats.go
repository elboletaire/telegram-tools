package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
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
	cmd.PersistentFlags().IntVarP(&sharedOpts.limit, "limit", "l", sharedOpts.limit, "Number of entries to fetch (0 = all)")
	cmd.PersistentFlags().IntVar(&sharedOpts.pageSize, "page-size", sharedOpts.pageSize, "Entries per page in the viewer")

	cmd.AddCommand(newChatsListCommand(sharedOpts))

	return cmd
}

// fetchChatsRequest holds the parameters for fetching chats
type fetchChatsRequest struct {
	Limit  int
	Search string
}

// fetchChatsWithProgress fetches chats from Telegram with progress feedback
// This is shared logic used by chats subcommands
func fetchChatsWithProgress(ctx context.Context, cfg *config.Config, out io.Writer, in io.Reader, errOut io.Writer, req fetchChatsRequest) ([]telegram.ChatInfo, error) {
	listReq := telegram.ListChatsRequest{
		Limit:  req.Limit,
		Search: req.Search,
		OnBatch: func(total int) {
			fmt.Fprintf(out, "\rLoading chats... %d fetched", total)
		},
		OnFloodWait: func(delay time.Duration, total int) {
			fmt.Fprintf(out, "\rHit rate limit, pausing %s after %d fetched...", delay.Round(time.Second), total)
		},
	}

	fmt.Fprintln(out, "Loading chats...")
	svc := telegram.NewService(cfg, telegram.WithIO(in, out, errOut))
	chats, err := svc.ListChats(ctx, listReq)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(out)

	return chats, nil
}
