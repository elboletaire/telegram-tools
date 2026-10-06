package telegram

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gotd/td/telegram/uploader"
	"golang.org/x/term"
)

const defaultProgressBarWidth = 30

// uploadProgressDisplay drives the Bubble Tea program that renders the upload UI.
type uploadProgressDisplay struct {
	trackedName string
	program     *tea.Program
	done        chan struct{}
	once        sync.Once
}

var _ uploader.Progress = (*uploadProgressDisplay)(nil)

func newUploadProgressDisplay(out io.Writer, actionLabel, fileName, thumbName, captionPreview string, removingCaption bool) *uploadProgressDisplay {
	// Only animate on a terminal; callers log plain lines otherwise.
	width, ok := terminalWidthFromWriter(out)
	if out == nil || !ok {
		return nil
	}
	model := newUploadProgressModel(actionLabel, fileName, thumbName, captionPreview, removingCaption, width)
	prog := tea.NewProgram(model, tea.WithOutput(out), tea.WithInput(nil), tea.WithoutSignalHandler())

	display := &uploadProgressDisplay{
		trackedName: fileName,
		program:     prog,
		done:        make(chan struct{}),
	}

	go func() {
		_, _ = prog.Run()
		close(display.done)
	}()

	return display
}

// Chunk implements uploader.Progress. It receives upload updates and forwards
// them to the Bubble Tea program.
func (d *uploadProgressDisplay) Chunk(ctx context.Context, state uploader.ProgressState) error {
	if d == nil {
		return nil
	}
	if state.Name != "" && state.Name != d.trackedName {
		return nil
	}
	var percent float64
	if state.Total > 0 {
		percent = float64(state.Uploaded) / float64(state.Total)
		if percent > 1 {
			percent = 1
		} else if percent < 0 {
			percent = 0
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		d.program.Send(progressPercentMsg{percent: percent})
		return nil
	}
}

func (d *uploadProgressDisplay) Success(message string) {
	if d == nil {
		return
	}
	d.program.Send(progressPercentMsg{percent: 1})
	d.finish(progressFinishedMsg{success: true, message: message})
}

func (d *uploadProgressDisplay) Fail(err error) {
	if d == nil {
		return
	}
	d.finish(progressFinishedMsg{success: false, message: err.Error()})
}

func (d *uploadProgressDisplay) finish(msg progressFinishedMsg) {
	d.once.Do(func() {
		d.program.Send(msg)
	})
}

func (d *uploadProgressDisplay) Wait() {
	if d == nil {
		return
	}
	<-d.done
}

type progressPercentMsg struct {
	percent float64
}

type progressFinishedMsg struct {
	success bool
	message string
}

type uploadProgressModel struct {
	progress        progress.Model
	percent         float64
	terminalWidth   int
	startedAt       time.Time
	actionLabel     string
	fileName        string
	thumbName       string
	captionPreview  string
	hasThumb        bool
	hasCaption      bool
	removingCaption bool
	done            bool
	success         bool
	statusMessage   string
}

func newUploadProgressModel(actionLabel, fileName, thumbName, captionPreview string, removingCaption bool, barWidth int) *uploadProgressModel {
	bar := progress.New(progress.WithDefaultGradient(), progress.WithWidth(progressBarWidth(barWidth)))
	bar.ShowPercentage = true
	return &uploadProgressModel{
		progress:        bar,
		terminalWidth:   progressBarWidth(barWidth),
		startedAt:       time.Now(),
		actionLabel:     actionLabel,
		fileName:        fileName,
		thumbName:       thumbName,
		captionPreview:  captionPreview,
		hasThumb:        strings.TrimSpace(thumbName) != "",
		hasCaption:      strings.TrimSpace(captionPreview) != "",
		removingCaption: removingCaption,
	}
}

func (m *uploadProgressModel) Init() tea.Cmd {
	return nil
}

func (m *uploadProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = progressBarWidth(msg.Width)
		m.progress.Width = m.terminalWidth
	case progressPercentMsg:
		m.percent = msg.percent
	case progressFinishedMsg:
		m.done = true
		m.success = msg.success
		m.statusMessage = msg.message
		return m, tea.Quit
	}
	return m, nil
}

func progressBarWidth(width int) int {
	if width <= 0 {
		return defaultProgressBarWidth
	}
	return width
}

func terminalWidthFromWriter(out io.Writer) (int, bool) {
	fileWithFD, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}

	fd := int(fileWithFD.Fd())
	if !term.IsTerminal(fd) {
		return 0, false
	}

	width, _, err := term.GetSize(fd)
	if err != nil {
		return 0, false
	}
	if width <= 0 {
		return 0, false
	}
	return width, true
}

func estimateRemainingDuration(elapsed time.Duration, percent float64) (time.Duration, bool) {
	if elapsed <= 0 || percent <= 0 || percent >= 1 {
		return 0, false
	}

	remaining := time.Duration(float64(elapsed) * (1 - percent) / percent)
	if remaining < 0 {
		return 0, false
	}
	return remaining, true
}

func buildETALabel(remaining time.Duration, known bool) string {
	if !known {
		return "ETA --:-- "
	}
	totalSeconds := int(remaining.Round(time.Second).Seconds())
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("ETA %d:%02d:%02d ", hours, minutes, seconds)
	}
	return fmt.Sprintf("ETA %02d:%02d ", minutes, seconds)
}

func progressContentWidth(totalWidth int, leftLabel string) int {
	width := progressBarWidth(totalWidth) - lipgloss.Width(leftLabel)
	if width < 1 {
		return 1
	}
	return width
}

func (m *uploadProgressModel) View() string {
	var b strings.Builder
	b.WriteString(actionLineStyle.Render(m.actionLabel + " " + fileEmphasisStyle.Render(m.fileName)))
	b.WriteString("\n")
	if m.hasThumb {
		b.WriteString(thumbLineStyle.Render("🖼️ Will use custom thumbnail " + fileEmphasisStyle.Render(m.thumbName)))
		b.WriteString("\n")
	}
	if m.hasCaption {
		b.WriteString(captionLineStyle.Render("📝 Setting caption to " + fileEmphasisStyle.Render(m.captionPreview)))
		b.WriteString("\n")
	} else if m.removingCaption {
		b.WriteString(captionLineStyle.Render("📝 Removing caption"))
		b.WriteString("\n")
	}
	if m.done {
		if m.success {
			b.WriteString(successLineStyle.Render("✅ " + m.statusMessage))
		} else {
			b.WriteString(errorLineStyle.Render("❌ " + m.statusMessage))
		}
	} else {
		remaining, known := estimateRemainingDuration(time.Since(m.startedAt), m.percent)
		etaLabel := buildETALabel(remaining, known)
		m.progress.Width = progressContentWidth(m.terminalWidth, etaLabel)
		b.WriteString(etaLineStyle.Render(etaLabel))
		b.WriteString(m.progress.ViewAs(m.percent))
	}
	return b.String()
}

var (
	fileEmphasisStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	actionLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	thumbLineStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("105"))
	captionLineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("222"))
	etaLineStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	successLineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("84")).Bold(true)
	errorLineStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)
)
