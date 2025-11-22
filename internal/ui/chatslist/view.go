package chatslist

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

// Display renders a read-only, paginated list of chats.
func Display(in io.Reader, out io.Writer, search string, chats []telegram.ChatInfo, pageSize int) error {
	pageSize = normalizePageSize(pageSize, len(chats))
	_, err := runChatsListProgram(newChatsListModel(search, chats, pageSize, false), in, out)
	return err
}

// Select renders a selectable list and returns the chosen chat if confirmed.
func Select(in io.Reader, out io.Writer, search string, chats []telegram.ChatInfo, pageSize int) (telegram.ChatInfo, bool, error) {
	pageSize = normalizePageSize(pageSize, len(chats))
	model := newChatsListModel(search, chats, pageSize, true)
	final, err := runChatsListProgram(model, in, out)
	if err != nil {
		return telegram.ChatInfo{}, false, err
	}
	if final.chosen < 0 || final.chosen >= len(final.chats) {
		return telegram.ChatInfo{}, false, nil
	}
	return final.chats[final.chosen], true, nil
}

type chatsListModel struct {
	chats      []telegram.ChatInfo
	pager      paginator.Model
	search     string
	styles     chatsListStyles
	windowWid  int
	selectable bool
	selected   int
	chosen     int
}

func newChatsListModel(search string, chats []telegram.ChatInfo, perPage int, selectable bool) chatsListModel {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = perPage
	p.SetTotalPages(len(chats))
	if p.TotalPages == 0 {
		p.TotalPages = 1
	}

	return chatsListModel{
		chats:      chats,
		pager:      p,
		search:     strings.TrimSpace(search),
		styles:     defaultChatsListStyles(),
		selectable: selectable,
		selected:   0,
		chosen:     -1,
	}
}

func (m chatsListModel) Init() tea.Cmd {
	return nil
}

func (m chatsListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWid = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.chosen = -1
			return m, tea.Quit
		case "enter":
			if m.selectable && len(m.chats) > 0 {
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

func (m chatsListModel) View() string {
	var b strings.Builder
	title := "Chats & Channels"
	if m.search != "" {
		title += fmt.Sprintf(" • filter: %q", m.search)
	}
	b.WriteString(m.styles.title.Render(title))
	b.WriteString("\n")

	meta := fmt.Sprintf("%d chats • page %d/%d • %d per page", len(m.chats), m.pager.Page+1, m.pager.TotalPages, m.pager.PerPage)
	b.WriteString(m.styles.meta.Render(meta))
	b.WriteString("\n\n")

	start, end := m.pager.GetSliceBounds(len(m.chats))
	for idx := start; idx < end; idx++ {
		chat := m.chats[idx]
		b.WriteString(m.renderChat(chat, idx == m.selected))
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

func (m chatsListModel) renderChat(chat telegram.ChatInfo, selected bool) string {
	var b strings.Builder
	if m.selectable {
		if selected {
			b.WriteString(m.styles.markerSelected.Render(">"))
		} else {
			b.WriteString(m.styles.marker.Render(" "))
		}
		b.WriteString(" ")
	}

	// Format chat ID and title
	idStr := fmt.Sprintf("#%d", chat.ID)
	titleStr := chat.Title
	if titleStr == "" {
		titleStr = "(no title)"
	}

	content := fmt.Sprintf("%s  %s", m.styles.id.Render(idStr), m.styles.chatTitle.Render(titleStr))

	// Add username if available
	if chat.Username != "" {
		content += fmt.Sprintf("  %s", m.styles.username.Render("@"+chat.Username))
	}

	// Add type badge
	if chat.Type != "" {
		content += "  " + m.styles.chatType.Render(strings.ToUpper(chat.Type))
	}

	// Add stats on next line
	var stats []string
	if chat.Participants > 0 {
		stats = append(stats, fmt.Sprintf("%d members", chat.Participants))
	}
	if chat.Unread > 0 {
		stats = append(stats, fmt.Sprintf("%d unread", chat.Unread))
	}
	if !chat.LastDate.IsZero() {
		lastDate := chat.LastDate.In(time.UTC).Format("2006-01-02 15:04")
		stats = append(stats, fmt.Sprintf("last: %s", lastDate))
	}

	if len(stats) > 0 {
		content += "\n" + m.styles.stats.Render(strings.Join(stats, " • "))
	}

	if selected && m.selectable {
		b.WriteString(m.styles.selected.Render(content))
	} else {
		b.WriteString(content)
	}
	return b.String()
}

func (m chatsListModel) dividerLine() string {
	width := m.windowWid
	if width <= 0 {
		width = 60
	}
	if width > 100 {
		width = 100
	}
	return strings.Repeat("-", width)
}

func (m *chatsListModel) moveSelection(delta int) {
	if len(m.chats) == 0 {
		return
	}
	newSel := m.selected + delta
	if newSel < 0 {
		newSel = 0
	}
	if newSel >= len(m.chats) {
		newSel = len(m.chats) - 1
	}
	m.selected = newSel

	// Auto-page to keep selection visible
	page := m.selected / m.pager.PerPage
	if page != m.pager.Page {
		m.pager.Page = page
	}
}

func (m *chatsListModel) alignSelectionToPageStart() {
	m.selected = m.pager.Page * m.pager.PerPage
	if m.selected >= len(m.chats) {
		m.selected = len(m.chats) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

type chatsListStyles struct {
	title          lipgloss.Style
	meta           lipgloss.Style
	id             lipgloss.Style
	chatTitle      lipgloss.Style
	username       lipgloss.Style
	chatType       lipgloss.Style
	stats          lipgloss.Style
	divider        lipgloss.Style
	hint           lipgloss.Style
	pill           lipgloss.Style
	marker         lipgloss.Style
	markerSelected lipgloss.Style
	selected       lipgloss.Style
}

func defaultChatsListStyles() chatsListStyles {
	return chatsListStyles{
		title:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")),
		meta:           lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		id:             lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")),
		chatTitle:      lipgloss.NewStyle().Bold(true),
		username:       lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
		chatType:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		stats:          lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		divider:        lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		hint:           lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
		pill:           lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
		marker:         lipgloss.NewStyle(),
		markerSelected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		selected:       lipgloss.NewStyle().Background(lipgloss.Color("236")),
	}
}

func normalizePageSize(pageSize int, totalItems int) int {
	if pageSize <= 0 {
		pageSize = 10
	}
	if totalItems > 0 && pageSize > totalItems {
		pageSize = totalItems
	}
	return pageSize
}

func runChatsListProgram(m chatsListModel, in io.Reader, out io.Writer) (chatsListModel, error) {
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
	final, err := p.Run()
	if err != nil {
		return chatsListModel{}, err
	}
	return final.(chatsListModel), nil
}
