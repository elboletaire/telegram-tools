package postslist

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/elboletaire/ttools/internal/telegram"
)

// Display renders an interactive, paginated list of posts.
func Display(in io.Reader, out io.Writer, chatDisplay, search string, posts []telegram.PostInfo, pageSize int) error {
	pageSize = normalizePageSize(pageSize, len(posts))
	model := newPostsListModel(chatDisplay, search, posts, pageSize)
	prog := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler())
	_, err := prog.Run()
	return err
}

type postsListModel struct {
	posts     []telegram.PostInfo
	pager     paginator.Model
	chat      string
	search    string
	styles    postsListStyles
	windowWid int
}

func newPostsListModel(chat, search string, posts []telegram.PostInfo, perPage int) postsListModel {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = perPage
	p.SetTotalPages(len(posts))
	if p.TotalPages == 0 {
		p.TotalPages = 1
	}

	return postsListModel{
		posts:  posts,
		pager:  p,
		chat:   chat,
		search: strings.TrimSpace(search),
		styles: defaultPostsListStyles(),
	}
}

func (m postsListModel) Init() tea.Cmd {
	return nil
}

func (m postsListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWid = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "pgdown", "right", "l", "n":
			m.pager.NextPage()
		case "pgup", "left", "h", "p":
			m.pager.PrevPage()
		case "home", "g":
			m.pager.Page = 0
		case "end", "G":
			m.pager.Page = m.pager.TotalPages - 1
		}
	}
	return m, nil
}

func (m postsListModel) View() string {
	var b strings.Builder
	title := fmt.Sprintf("Posts in %s", m.chat)
	if m.search != "" {
		title += fmt.Sprintf(" • filter: %q", m.search)
	}
	b.WriteString(m.styles.title.Render(title))
	b.WriteString("\n")

	meta := fmt.Sprintf("%d posts • page %d/%d • %d per page", len(m.posts), m.pager.Page+1, m.pager.TotalPages, m.pager.PerPage)
	b.WriteString(m.styles.meta.Render(meta))
	b.WriteString("\n\n")

	start, end := m.pager.GetSliceBounds(len(m.posts))
	for idx := start; idx < end; idx++ {
		post := m.posts[idx]
		b.WriteString(m.renderPost(post))
		if idx < end-1 {
			b.WriteString("\n")
			b.WriteString(m.styles.divider.Render(m.dividerLine()))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	hint := fmt.Sprintf("%s  navigate: ←/h/pgup/p | →/l/pgdn/n | q to exit", m.styles.pill.Render(m.pager.View()))
	b.WriteString(m.styles.hint.Render(hint))

	return b.String()
}

func (m postsListModel) renderPost(post telegram.PostInfo) string {
	caption := formatCaption(post.Caption, m.captionLimit())
	date := post.Date.In(time.UTC).Format("2006-01-02 15:04 MST")

	var b strings.Builder
	b.WriteString(m.styles.id.Render(fmt.Sprintf("#%d", post.ID)))
	b.WriteString("  ")
	b.WriteString(m.styles.date.Render(date))
	if strings.TrimSpace(post.MediaType) != "" {
		b.WriteString("  ")
		b.WriteString(m.styles.media.Render(strings.ToUpper(post.MediaType)))
	}
	if caption != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.caption.Render(caption))
	}
	return b.String()
}

func (m postsListModel) captionLimit() int {
	if m.windowWid <= 0 {
		return 120
	}
	return max(40, m.windowWid-8)
}

func (m postsListModel) dividerLine() string {
	width := m.windowWid
	if width <= 0 {
		width = 60
	}
	if width > 100 {
		width = 100
	}
	return strings.Repeat("-", width)
}

type postsListStyles struct {
	title   lipgloss.Style
	meta    lipgloss.Style
	id      lipgloss.Style
	date    lipgloss.Style
	media   lipgloss.Style
	caption lipgloss.Style
	divider lipgloss.Style
	hint    lipgloss.Style
	pill    lipgloss.Style
}

func defaultPostsListStyles() postsListStyles {
	return postsListStyles{
		title:   lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true),
		meta:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		id:      lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true),
		date:    lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
		media:   lipgloss.NewStyle().Foreground(lipgloss.Color("180")).Bold(true),
		caption: lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		divider: lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		hint:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		pill:    lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Background(lipgloss.Color("250")).Padding(0, 1).Bold(true),
	}
}

func normalizePageSize(pageSize, total int) int {
	switch {
	case pageSize <= 0:
		return min(10, max(1, total))
	case total > 0 && pageSize > total:
		return total
	default:
		return pageSize
	}
}

func formatCaption(caption string, limit int) string {
	clean := strings.Join(strings.Fields(strings.TrimSpace(caption)), " ")
	if clean == "" {
		return ""
	}
	runes := []rune(clean)
	if limit > 0 && len(runes) > limit {
		return string(runes[:limit]) + "..."
	}
	return clean
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
