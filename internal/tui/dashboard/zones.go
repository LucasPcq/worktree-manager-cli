package dashboard

import "strconv"

// Zone ids are the contract between what the renderer marks and what the mouse
// handler looks up; keeping them here is what makes a mis-mapped zone testable.
const (
	zoneTabPrefix    = "tab:"
	zoneRowPrefix    = "row:"
	zoneList         = "panel:list"
	zoneDetail       = "panel:detail"
	zoneOutput       = "panel:output"
	zoneOutputToggle = "output:toggle"
)

func tabZone(index int) string { return zoneTabPrefix + strconv.Itoa(index) }

// rowZone keys a row by its index in the full worktree slice, not by its
// position on screen, so a click resolves the same worktree at any scroll.
func rowZone(index int) string { return zoneRowPrefix + strconv.Itoa(index) }
