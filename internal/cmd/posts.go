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

// sharedPostsOptions holds flags that are shared across all posts subcommands
type sharedPostsOptions struct {
	limit    int
	pageSize int
}

func newPostsCommand() *cobra.Command {
	sharedOpts := &sharedPostsOptions{
		limit:    0,
		pageSize: 10,
	}

	cmd := &cobra.Command{
		Use:   "posts",
		Short: "Explore channel posts",
	}

	// Shared flags available to all subcommands
	cmd.PersistentFlags().IntVarP(&sharedOpts.limit, "limit", "l", sharedOpts.limit, "Number of entries to fetch (0 = all)")
	cmd.PersistentFlags().IntVar(&sharedOpts.pageSize, "page-size", sharedOpts.pageSize, "Entries per page in the viewer")

	cmd.AddCommand(newPostsListCommand(sharedOpts))
	cmd.AddCommand(newPostsFindCommand(sharedOpts))

	return cmd
}

// fetchPostsRequest holds the parameters for fetching posts
type fetchPostsRequest struct {
	ChatId      string
	Limit       int
	Search      string
	MediaFilter string // "video", "photo", "document", or empty for all
	FileFilter  string // filename search
}

// fetchPostsWithProgress fetches posts from Telegram with progress feedback
// This is shared logic used by both list and find commands
func fetchPostsWithProgress(ctx context.Context, cfg *config.Config, out io.Writer, in io.Reader, errOut io.Writer, req fetchPostsRequest) ([]telegram.PostInfo, error) {
	listReq := telegram.ListPostsRequest{
		ChatId: req.ChatId,
		Limit:  req.Limit,
		Search: req.Search,
		OnBatch: func(total int) {
			fmt.Fprintf(out, "\rLoading posts... %d fetched", total)
		},
		OnFloodWait: func(delay time.Duration, total int) {
			fmt.Fprintf(out, "\rHit rate limit, pausing %s after %d fetched...", delay.Round(time.Second), total)
		},
	}

	fmt.Fprintln(out, "Loading posts...")
	svc := telegram.NewService(cfg, telegram.WithIO(in, out, errOut))
	posts, err := svc.ListPosts(ctx, listReq)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(out)

	// Apply media type filtering if specified
	if req.MediaFilter != "" || req.FileFilter != "" {
		posts = filterPosts(posts, req.MediaFilter, req.FileFilter)
	}

	return posts, nil
}

// filterPosts filters posts by media type and/or filename
func filterPosts(posts []telegram.PostInfo, mediaType string, filename string) []telegram.PostInfo {
	if mediaType == "" && filename == "" {
		return posts
	}

	filtered := make([]telegram.PostInfo, 0, len(posts))
	for _, post := range posts {
		// Check media type filter
		if mediaType != "" && post.MediaType != mediaType {
			continue
		}

		// Check filename filter (case-insensitive substring match)
		// Note: This requires access to the actual file metadata
		// For now, we'll skip filename filtering as it requires additional API calls
		// This can be enhanced later if filename is added to PostInfo
		if filename != "" {
			// TODO: Implement filename filtering when metadata is available
			continue
		}

		filtered = append(filtered, post)
	}

	return filtered
}
