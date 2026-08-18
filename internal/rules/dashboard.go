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

type ModalRectParams struct {
	ScreenWidth  int
	ScreenHeight int
	// ContentHeight is how many body lines the modal wants to show; the rect adds
	// the border rows and clamps the whole box to the screen.
	ContentHeight int
}

// ComputeModalRect places a modal box over the dashboard. Like
// ComputeDashboardLayout it is the single reference: the renderer draws the box
// there and its rows are clickable there.
func ComputeModalRect(params ModalRectParams) domain.Rect {
	screenWidth, screenHeight := max(params.ScreenWidth, 0), max(params.ScreenHeight, 0)
	width := ModalWidth(screenWidth)
	height := min(max(params.ContentHeight, 1)+domain.DashboardModalChrome, screenHeight)
	return domain.Rect{
		X:      max((screenWidth-width)/2, 0),
		Y:      max((screenHeight-height)/2, 0),
		Width:  width,
		Height: max(height, 0),
	}
}

type MenuRectParams struct {
	// AnchorX and AnchorY are the cell the menu was opened from: the click, or
	// the row the keyboard is on.
	AnchorX int
	AnchorY int
	// Width and Height are the box's outer size, borders included.
	Width        int
	Height       int
	ScreenWidth  int
	ScreenHeight int
}

// ComputeMenuRect places a context menu next to the cell it was opened from. It
// hangs below the anchor and flips above it rather than running off the bottom,
// which is what every other context menu does — and, like the panels, it is the
// one reference the renderer draws from and the entries are clickable in.
func ComputeMenuRect(params MenuRectParams) domain.Rect {
	width := min(params.Width, params.ScreenWidth)
	height := min(params.Height, params.ScreenHeight)

	x := max(min(params.AnchorX, params.ScreenWidth-width), 0)

	y := params.AnchorY + 1
	if y+height > params.ScreenHeight {
		y = params.AnchorY - height
	}
	y = max(min(y, params.ScreenHeight-height), 0)

	return domain.Rect{X: x, Y: y, Width: max(width, 0), Height: max(height, 0)}
}

// ModalWidth is the width a modal gets on a screen, before its content is known:
// its widgets need it to lay themselves out.
func ModalWidth(screenWidth int) int {
	width := screenWidth * domain.DashboardModalWidthPercent / 100
	width = max(width, min(domain.DashboardModalMinWidth, screenWidth))
	return min(width, domain.DashboardModalMaxWidth, screenWidth)
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
