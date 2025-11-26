package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

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

			// Determine media filter
			mediaFilter := ""
			if opts.video {
				mediaFilter = "video"
			} else if opts.photo {
				mediaFilter = "photo"
			} else if opts.document {
				mediaFilter = "document"
			}

			// Fetch posts using the shared helper
			req := fetchPostsRequest{
				ChatId:      chat,
				Limit:       sharedOpts.limit,
				Search:      searchText,
				MediaFilter: mediaFilter,
				FileFilter:  opts.filename,
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
