package rules

import "github.com/LucasPcq/wtm/internal/domain"

type DashboardLayoutParams struct {
	Width          int
	Height         int
	OutputExpanded bool
	// DetailOpen only matters under DashboardNarrowWidth, where the detail takes
	// the list's place instead of sitting beside it.
	DetailOpen bool
}

// minDashboardBody keeps the list readable when the terminal is short: the
// output panel gives up its rows before the body does.
const minDashboardBody = 3

// ComputeDashboardLayout places every dashboard panel for one frame. It is the
// single reference the renderer draws from and the mouse zones are marked
// against, so a panel cannot drift from the region it is clickable in.
func ComputeDashboardLayout(params DashboardLayoutParams) domain.DashboardLayout {
	width, height := max(params.Width, 0), max(params.Height, 0)

	tabs := domain.Rect{X: 0, Y: 0, Width: width, Height: min(1, height)}
	help := domain.Rect{X: 0, Y: max(height-1, 0), Width: width, Height: min(1, max(height-1, 0))}

	outputHeight := domain.DashboardChromeHeight
	if params.OutputExpanded {
		outputHeight += domain.DashboardOutputBodyHeight
	}
	available := height - tabs.Height - help.Height
	if available-outputHeight < minDashboardBody {
		outputHeight = available - minDashboardBody
	}
	outputHeight = max(outputHeight, 0)
	bodyHeight := max(available-outputHeight, 0)

	layout := domain.DashboardLayout{
		Narrow:      width < domain.DashboardNarrowWidth,
		Tabs:        tabs,
		Help:        help,
		Output:      domain.Rect{X: 0, Y: tabs.Height + bodyHeight, Width: width, Height: outputHeight},
		OutputLines: max(outputHeight-domain.DashboardChromeHeight, 0),
	}

	body := domain.Rect{X: 0, Y: tabs.Height, Width: width, Height: bodyHeight}
	if layout.Narrow {
		if params.DetailOpen {
			layout.Detail, layout.DetailVisible = body, true
			return layout
		}
		layout.List, layout.ListVisible = body, true
		layout.ListRows = max(body.Height-domain.DashboardChromeHeight, 0)
		return layout
	}

	listWidth := dashboardListWidth(width)
	layout.List = domain.Rect{X: 0, Y: body.Y, Width: listWidth, Height: body.Height}
	layout.Detail = domain.Rect{X: listWidth, Y: body.Y, Width: width - listWidth, Height: body.Height}
	layout.ListVisible, layout.DetailVisible = true, true
	layout.ListRows = max(body.Height-domain.DashboardChromeHeight, 0)
	return layout
}

func dashboardListWidth(width int) int {
	if width < domain.DashboardMinListWidth+domain.DashboardMinDetailWidth {
		return width / 2
	}
	listWidth := width * domain.DashboardListWidthPercent / 100
	listWidth = max(listWidth, domain.DashboardMinListWidth)
	return min(listWidth, width-domain.DashboardMinDetailWidth)
}

type DashboardScrollParams struct {
	Cursor  int
	Total   int
	Visible int
	Offset  int
}

// DashboardScrollOffset returns the first visible row keeping Cursor on screen.
func DashboardScrollOffset(params DashboardScrollParams) int {
	if params.Visible <= 0 || params.Total <= params.Visible {
		return 0
	}
	cursor := ClampIndex(params.Cursor, params.Total)
	offset := DashboardClampOffset(DashboardOffsetParams{
		Offset:  params.Offset,
		Total:   params.Total,
		Visible: params.Visible,
	})
	return max(min(offset, cursor), cursor-params.Visible+1)
}

type DashboardOffsetParams struct {
	Offset  int
	Total   int
	Visible int
}

// DashboardClampOffset keeps a free scroll offset — one with no cursor to follow,
// as in the output panel — inside the scrollable range.
func DashboardClampOffset(params DashboardOffsetParams) int {
	if params.Visible <= 0 || params.Total <= params.Visible {
		return 0
	}
	return min(max(params.Offset, 0), params.Total-params.Visible)
}

// ClampIndex keeps an index inside [0, count-1], and returns 0 for an empty set.
func ClampIndex(index, count int) int {
	if count <= 0 {
		return 0
	}
	return min(max(index, 0), count-1)
}
