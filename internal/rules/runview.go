package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RunViewLayoutParams struct {
	Width  int
	Height int
	// NoticeLines is how many rows a report wants under the header. It is served
	// only out of what the body can spare.
	NoticeLines int
}

// ComputeRunViewLayout places the run view's regions for one frame, and sizes
// the emulator the pane box will hold. It is the single reference the renderer
// draws from and the job's PTY is resized against.
func ComputeRunViewLayout(params RunViewLayoutParams) domain.RunViewLayout {
	width, height := max(params.Width, 0), max(params.Height, 0)

	helpHeight := min(1, height)
	headerHeight := min(1, max(height-helpHeight, 0))
	available := max(height-helpHeight-headerHeight, 0)

	noticeHeight := min(max(params.NoticeLines, 0), max(available-domain.RunViewMinBodyRows, 0))
	bodyHeight := available - noticeHeight

	sidebarWidth := 0
	if width-domain.RunViewSidebarWidth >= domain.RunViewSidebarMinPaneCols {
		sidebarWidth = domain.RunViewSidebarWidth
	}
	bodyY := headerHeight + noticeHeight

	return domain.RunViewLayout{
		Header:         domain.Rect{X: 0, Y: 0, Width: width, Height: headerHeight},
		Notice:         domain.Rect{X: 0, Y: headerHeight, Width: width, Height: noticeHeight},
		Sidebar:        domain.Rect{X: 0, Y: bodyY, Width: sidebarWidth, Height: bodyHeight},
		Pane:           domain.Rect{X: sidebarWidth, Y: bodyY, Width: width - sidebarWidth, Height: bodyHeight},
		Help:           domain.Rect{X: 0, Y: max(height-1, 0), Width: width, Height: helpHeight},
		SidebarVisible: sidebarWidth > 0,
		SidebarRows:    max(bodyHeight-domain.RunViewPanelChrome, 0),
		PaneCols:       max(width-sidebarWidth-domain.RunViewBorderWidth, 0),
		PaneRows:       max(bodyHeight-domain.RunViewPanelChrome, 0),
	}
}

// ClipReport fits an abort report into the rows the frame can spare. Its head
// is what the run has to say, but its last line is how the band is taken off
// the screen: cutting from the bottom leaves a reader on a short terminal with
// a report they cannot remove and a pane they cannot widen. Below two rows
// there is only room for what happened.
func ClipReport(lines []string, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	if height == 1 {
		return lines[:1]
	}
	return append(lines[:height-1:height-1], lines[len(lines)-1])
}

// MatchesJobFilter reads the run view's filter box against a job's name. An
// empty filter matches everything: the box is open but says nothing yet.
func MatchesJobFilter(name, filter string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(strings.TrimSpace(filter)))
}
