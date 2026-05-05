package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/floodwait"
	"github.com/elboletaire/ttools/internal/telegram"
	"github.com/elboletaire/ttools/internal/ui/postslist"
)

type postsFindOptions struct {
	video    bool
	photo    bool
	document bool
	filename string
}

func newPostsFindCommand(sharedOpts *sharedPostsOptions) *cobra.Command {
	opts := &postsFindOptions{}

	cmd := &cobra.Command{
		Use:   "find [text]",
		Short: "Find posts by text, media type, or metadata",
		Long: `Find posts by searching in captions/messages or filtering by media type.

Examples:
  # Find posts containing "hello world" in caption
  ttools posts find "hello world"

  # Find all posts with videos
  ttools posts find --video

  # Find videos with "intro" in the caption
  ttools posts find "intro" --video

  # Find posts with photos
  ttools posts find --photo

  # Find documents with specific filename
  ttools posts find --document --filename "report"`,
		Args: cobra.MaximumNArgs(1),
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

			// Determine the search text
			searchText := ""
			if len(args) > 0 {
				searchText = args[0]
			}

			hasMediaFilter := opts.video || opts.photo || opts.document
			var posts []telegram.PostInfo

			if searchText == "" && !hasMediaFilter {
				// With no server-side criteria, keep the old history-based behavior so
				// `posts find` with no arguments lists posts instead of issuing an
				// empty messages.search query.
				req := fetchPostsRequest{
					ChatId:     chat,
					Limit:      sharedOpts.limit,
					FileFilter: opts.filename,
				}
				posts, err = fetchPostsWithProgress(
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
			} else {
				// Build server-side media filter for messages.search
				mediaFilter := ""
				var searchFilter tg.MessagesFilterClass = &tg.InputMessagesFilterEmpty{}
				if opts.video {
					mediaFilter = "video"
					searchFilter = &tg.InputMessagesFilterVideo{}
				} else if opts.photo {
					mediaFilter = "photo"
					searchFilter = &tg.InputMessagesFilterPhotos{}
				} else if opts.document {
					mediaFilter = "document"
					searchFilter = &tg.InputMessagesFilterDocument{}
				}

				// Use server-side search
				fmt.Fprintln(out, "Searching...")
				svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), out, cmd.ErrOrStderr()))
				searchReq := telegram.SearchPostsRequest{
					ChatId: chat,
					Limit:  sharedOpts.limit,
					Query:  searchText,
					Filter: searchFilter,
					OnBatch: func(total int) {
						fmt.Fprintf(out, "\rSearching... %d fetched", total)
					},
					OnFloodWait: func(delay time.Duration, total int) {
						floodwait.Start(cmd.Context(), out, delay, func(remaining time.Duration) string {
							return fmt.Sprintf("\rHit rate limit, retrying in %.2fs after %d fetched...", remaining.Seconds(), total)
						})
					},
				}

				posts, err = svc.SearchPosts(cmd.Context(), searchReq)
				if err != nil {
					if searchText == "" && tg.IsSearchQueryEmpty(err) {
						req := fetchPostsRequest{
							ChatId:      chat,
							Limit:       sharedOpts.limit,
							MediaFilter: mediaFilter,
							FileFilter:  opts.filename,
						}
						posts, err = fetchPostsWithProgress(
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
					} else {
						return err
					}
				} else {
					fmt.Fprintln(out)

					// Apply client-side filename filter (cannot be done server-side)
					if opts.filename != "" {
						posts = filterPostsByFilename(posts, opts.filename)
					}
				}
			}

			if len(posts) == 0 {
				fmt.Fprintln(out, "No posts found matching the criteria")
				return nil
			}

			fmt.Fprintf(out, "Found %d posts\n\n", len(posts))

			// Display using the same TUI as list
			if err := postslist.Display(cmd.InOrStdin(), out, chat, searchText, posts, sharedOpts.pageSize); err != nil {
				return fmt.Errorf("render list: %w", err)
			}
			return nil
		},
	}

	// Media type filters
	cmd.Flags().BoolVar(&opts.video, "video", false, "Filter posts with video attachments")
	cmd.Flags().BoolVar(&opts.photo, "photo", false, "Filter posts with photo attachments")
	cmd.Flags().BoolVar(&opts.document, "document", false, "Filter posts with document attachments")

	// Metadata filters
	cmd.Flags().StringVar(&opts.filename, "filename", "", "Search in attachment filenames")

	return cmd
}

// filterPostsByFilename filters posts by attachment filename (case-insensitive substring match).
func filterPostsByFilename(posts []telegram.PostInfo, filename string) []telegram.PostInfo {
	if filename == "" {
		return posts
	}
	lower := strings.ToLower(filename)
	filtered := make([]telegram.PostInfo, 0, len(posts))
	for _, post := range posts {
		if strings.Contains(strings.ToLower(post.Filename), lower) {
			filtered = append(filtered, post)
		}
	}
	return filtered
}
