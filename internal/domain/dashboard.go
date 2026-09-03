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
	// HeaderTall reports which header rules.ComputeDashboardLayout chose for
	// this frame: the six-row signature block when true, the compact
	// three-row header otherwise. The renderer reads it rather than
	// re-deriving the choice from Tabs.Height against a threshold of its own.
	HeaderTall bool

	Tabs   Rect
	List   Rect
	Detail Rect
	Output Rect
	Help   Rect

	ListVisible   bool
	DetailVisible bool

	// ListRows is how many worktree rows fit in the list body, TreeRows how many
	// tree nodes fit in the same space (one line each), OutputLines how many lines
	// fit in the output body (0 when it is folded).
	ListRows int
	TreeRows int
	// ServicesRows is how many lines the Services tab draws — one per row, like
	// the tree, since its blocks are flattened into a single list.
	ServicesRows int
	OutputLines  int
}
