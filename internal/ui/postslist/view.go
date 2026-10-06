package postslist

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/elboletaire/telegram-tools/internal/telegram"
	"github.com/elboletaire/telegram-tools/internal/ui"
)

// Display renders a read-only, paginated list of posts. Without a terminal it
// prints one tab-separated line per post instead: id, date, media, caption.
func Display(in io.Reader, out io.Writer, chatDisplay, search string, posts []telegram.PostInfo, pageSize int) error {
	if !ui.Interactive(in, out) {
		return writePlain(out, posts)
	}
	pageSize = normalizePageSize(pageSize, len(posts))
	_, err := runPostsListProgram(newPostsListModel(chatDisplay, search, posts, pageSize, false), in, out)
	return err
}

// Select renders a selectable list and returns the chosen post if confirmed.
func Select(in io.Reader, out io.Writer, chatDisplay, search string, posts []telegram.PostInfo, pageSize int) (telegram.PostInfo, bool, error) {
	if !ui.Interactive(in, out) {
		return telegram.PostInfo{}, false, fmt.Errorf("selecting a post needs an interactive terminal; pass --post-id instead")
	}
	pageSize = normalizePageSize(pageSize, len(posts))
	model := newPostsListModel(chatDisplay, search, posts, pageSize, true)
	final, err := runPostsListProgram(model, in, out)
	if err != nil {
		return telegram.PostInfo{}, false, err
	}
	if final.chosen < 0 || final.chosen >= len(final.posts) {
		return telegram.PostInfo{}, false, nil
	}
	return final.posts[final.chosen], true, nil
}

func writePlain(out io.Writer, posts []telegram.PostInfo) error {
	for _, p := range posts {
		caption := strings.Join(strings.Fields(p.Caption), " ")
		if _, err := fmt.Fprintf(out, "%d\t%s\t%s\t%s\n", p.ID, p.Date.Format("2006-01-02 15:04"), p.MediaType, caption); err != nil {
			return err
		}
	}
	return nil
}

type postsListModel struct {
	posts      []telegram.PostInfo
	pager      paginator.Model
	chat       string
	search     string
	styles     postsListStyles
	windowWid  int
	selectable bool
	selected   int
	chosen     int
}

func newPostsListModel(chat, search string, posts []telegram.PostInfo, perPage int, selectable bool) postsListModel {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = perPage
	p.SetTotalPages(len(posts))
	if p.TotalPages == 0 {
		p.TotalPages = 1
	}

	return postsListModel{
		posts:      posts,
		pager:      p,
		chat:       chat,
		search:     strings.TrimSpace(search),
		styles:     defaultPostsListStyles(),
		selectable: selectable,
		selected:   0,
		chosen:     -1,
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
			m.chosen = -1
			return m, tea.Quit
		case "enter":
			if m.selectable && len(m.posts) > 0 {
				m.chosen = m.selected
				return m, tea.Quit
			}
		case "pgdown", "right", "l", "n":
			m.pager.NextPage()
			m.alignSelectionToPageStart()
		case "pgup", "left", "h", "p":
			m.pager.PrevPage()
			m.alignSelectionToPageStart()
		case "home", "g":
			m.pager.Page = 0
			m.alignSelectionToPageStart()
		case "end", "G":
			m.pager.Page = m.pager.TotalPages - 1
			m.alignSelectionToPageStart()
		case "up", "k":
			m.moveSelection(-1)
		case "down", "j":
			m.moveSelection(1)
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
		b.WriteString(m.renderPost(post, idx == m.selected))
		if idx < end-1 {
			b.WriteString("\n")
			b.WriteString(m.styles.divider.Render(m.dividerLine()))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	hint := fmt.Sprintf("%s  navigate: ←/h/pgup/p | →/l/pgdn/n | q to exit", m.styles.pill.Render(m.pager.View()))
	if m.selectable {
		hint += " | ↑/↓ to choose, enter to select"
	}
	b.WriteString(m.styles.hint.Render(hint))

	return b.String()
}

func (m postsListModel) renderPost(post telegram.PostInfo, selected bool) string {
	caption := formatCaption(post.Caption, m.captionLimit())
	date := post.Date.In(time.UTC).Format("2006-01-02 15:04 MST")

	var b strings.Builder
	if m.selectable {
		if selected {
			b.WriteString(m.styles.markerSelected.Render(">"))
		} else {
			b.WriteString(m.styles.marker.Render(" "))
		}
		b.WriteString(" ")
	}
	content := fmt.Sprintf("%s  %s", m.styles.id.Render(fmt.Sprintf("#%d", post.ID)), m.styles.date.Render(date))
	if strings.TrimSpace(post.MediaType) != "" {
		content += "  " + m.styles.media.Render(strings.ToUpper(post.MediaType))
	}
	if caption != "" {
		content += "\n" + m.styles.caption.Render(caption)
	}
	if selected && m.selectable {
		b.WriteString(m.styles.selected.Render(content))
	} else {
		b.WriteString(content)
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
	title          lipgloss.Style
	meta           lipgloss.Style
	id             lipgloss.Style
	date           lipgloss.Style
	media          lipgloss.Style
	caption        lipgloss.Style
	divider        lipgloss.Style
	hint           lipgloss.Style
	pill           lipgloss.Style
	marker         lipgloss.Style
	markerSelected lipgloss.Style
	selected       lipgloss.Style
}

func defaultPostsListStyles() postsListStyles {
	return postsListStyles{
		title:          lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true),
		meta:           lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		id:             lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true),
		date:           lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
		media:          lipgloss.NewStyle().Foreground(lipgloss.Color("180")).Bold(true),
		caption:        lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		divider:        lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		hint:           lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		pill:           lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Background(lipgloss.Color("250")).Padding(0, 1).Bold(true),
		marker:         lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		markerSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("229")),
		selected:       lipgloss.NewStyle().Background(lipgloss.Color("236")),
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

func (m *postsListModel) moveSelection(delta int) {
	if !m.selectable || len(m.posts) == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.posts) {
		m.selected = len(m.posts) - 1
	}
	m.pager.Page = m.selected / m.pager.PerPage
}

func (m *postsListModel) alignSelectionToPageStart() {
	if !m.selectable || len(m.posts) == 0 {
		return
	}
	start, end := m.pager.GetSliceBounds(len(m.posts))
	if m.selected < start {
		m.selected = start
	} else if m.selected >= end {
		m.selected = end - 1
	}
}

func runPostsListProgram(model postsListModel, in io.Reader, out io.Writer) (postsListModel, error) {
	prog := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler())
	final, err := prog.Run()
	if err != nil {
		return postsListModel{}, err
	}
	typed, ok := final.(postsListModel)
	if !ok {
		return postsListModel{}, fmt.Errorf("unexpected model type %T", final)
	}
	return typed, nil
}
