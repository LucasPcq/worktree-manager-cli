package domain

// Rect is a screen region in terminal cells, top-left origin, zero-based.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// DashboardLayout is the resolved geometry of one dashboard frame. Every panel
// is placed here so the renderer only draws and the hit-testing has a single
// reference to agree with.
type DashboardLayout struct {
	Narrow bool

	Tabs   Rect
	List   Rect
	Detail Rect
	Output Rect
	Help   Rect

	ListVisible   bool
	DetailVisible bool

	// ListRows is how many worktree rows fit in the list body, OutputLines how
	// many lines fit in the output body (0 when it is folded).
	ListRows    int
	OutputLines int
}
