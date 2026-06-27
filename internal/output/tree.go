package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// Tree connector glyphs for the ASCII forest.
const (
	treeBranch = "├─ "
	treeLast   = "└─ "
	treePipe   = "│  "
	treeBlank  = "   "
)

// FormatTree renders the worktree forest as a coloured ASCII tree. The returned
// body is raw (no outer blank lines); the command frames it.
func FormatTree(forest domain.Forest) string {
	if len(forest.Roots) == 0 {
		return "No worktrees found."
	}

	lines := make([]treeLine, 0)
	for _, root := range forest.Roots {
		appendTreeLines(&root, "", "", &lines)
	}

	labelWidth := 0
	for _, l := range lines {
		labelWidth = max(labelWidth, l.labelWidth())
	}

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.render(labelWidth))
		b.WriteString("\n")
	}
	return b.String()
}

// treeLine is one rendered node: its connector prefix, the node it represents,
// and the pre-rendered annotation string.
type treeLine struct {
	prefix      string // plain connector run, e.g. "│  ├─ "
	branch      string // plain branch name (alignment is measured on this)
	styled      string // styled branch label
	annotations string // styled, trailing
}

func (l treeLine) labelWidth() int {
	return len([]rune(l.prefix)) + len([]rune(l.branch))
}

func (l treeLine) render(labelWidth int) string {
	line := styles.Muted.Render(l.prefix) + l.styled
	if l.annotations == "" {
		return styles.Indent + line
	}
	pad := labelWidth - l.labelWidth() + 2
	return styles.Indent + line + strings.Repeat(" ", pad) + l.annotations
}

// appendTreeLines walks the forest depth-first, building one treeLine per node.
// prefix is the gutter inherited from ancestors; connector is this node's own
// glyph ("" for a root).
func appendTreeLines(node *domain.TreeNode, prefix, connector string, out *[]treeLine) {
	*out = append(*out, treeLine{
		prefix:      prefix + connector,
		branch:      node.Branch,
		styled:      styleBranch(node),
		annotations: formatTreeAnnotations(node),
	})

	childPrefix := prefix
	switch connector {
	case treeBranch:
		childPrefix += treePipe
	case treeLast:
		childPrefix += treeBlank
	}

	for i := range node.Children {
		glyph := treeBranch
		if i == len(node.Children)-1 {
			glyph = treeLast
		}
		appendTreeLines(&node.Children[i], childPrefix, glyph, out)
	}
}

func styleBranch(node *domain.TreeNode) string {
	if node.IsVirtual {
		return styles.Muted.Render(node.Branch)
	}
	return styles.Bold.Render(node.Branch)
}

// treeAnnotation enumerates a node's status badges in their canonical display
// order. nodeAnnotations is the single source of truth for which badges appear
// and in what order; the ASCII (formatTreeAnnotations) and Mermaid (mermaidLabel)
// renderers differ only in how they style each one, so they can't drift apart.
type treeAnnotation int

const (
	annVirtual treeAnnotation = iota
	annPR
	annAhead
	annDirty
	annNeedsSync
	annCycle
)

func nodeAnnotations(node *domain.TreeNode) []treeAnnotation {
	kinds := make([]treeAnnotation, 0, 6)
	if node.IsVirtual {
		kinds = append(kinds, annVirtual)
	}
	if node.Status.PR != nil {
		kinds = append(kinds, annPR)
	}
	if node.Status.CommitsAhead > 0 {
		kinds = append(kinds, annAhead)
	}
	if node.Status.IsDirty {
		kinds = append(kinds, annDirty)
	}
	if node.Status.NeedsSync {
		kinds = append(kinds, annNeedsSync)
	}
	if node.Status.InCycle {
		kinds = append(kinds, annCycle)
	}
	return kinds
}

// formatTreeAnnotations builds the styled trailing status string for an ASCII node.
func formatTreeAnnotations(node *domain.TreeNode) string {
	parts := make([]string, 0, 6)
	for _, kind := range nodeAnnotations(node) {
		switch kind {
		case annVirtual:
			parts = append(parts, styles.Muted.Render("(no worktree)"))
		case annPR:
			parts = append(parts, formatTreePR(node.Status.PR))
		case annAhead:
			parts = append(parts, styles.Muted.Render(fmt.Sprintf("↑%d", node.Status.CommitsAhead)))
		case annDirty:
			parts = append(parts, styles.Warning.Render("● dirty"))
		case annNeedsSync:
			parts = append(parts, styles.Warning.Render("⚠ needs sync"))
		case annCycle:
			parts = append(parts, styles.Warning.Render("⚠ cycle"))
		}
	}
	return strings.Join(parts, "  ")
}

func formatTreePR(pr *domain.WorktreeListPR) string {
	if pr == nil {
		return ""
	}
	label := fmt.Sprintf("PR #%d", pr.Number)
	switch pr.State {
	case domain.PRStateMerged:
		return styles.Muted.Render(label + " merged")
	case domain.PRStateClosed:
		return styles.Muted.Render(label + " closed")
	default:
		return styles.Success.Render(label)
	}
}

// WriteTreeJSON writes the forest as an indented JSON object.
func WriteTreeJSON(w io.Writer, forest domain.Forest) error {
	return encodeJSON(w, forest)
}

// WriteTreeMermaid writes the forest as a Mermaid flowchart (top-down). Node IDs
// are sanitised branch names; labels carry the branch plus a plain-text status
// summary. It is machine output and is never framed.
func WriteTreeMermaid(w io.Writer, forest domain.Forest) error {
	var b strings.Builder
	b.WriteString("flowchart TD\n")

	var walk func(node *domain.TreeNode, parentID string)
	walk = func(node *domain.TreeNode, parentID string) {
		id := mermaidID(node.Branch)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", id, mermaidLabel(node))
		if parentID != "" {
			fmt.Fprintf(&b, "  %s --> %s\n", parentID, id)
		}
		for i := range node.Children {
			walk(&node.Children[i], id)
		}
	}

	for i := range forest.Roots {
		walk(&forest.Roots[i], "")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// mermaidID turns a branch name into a safe Mermaid node id (alphanumerics and
// underscores only).
func mermaidID(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func mermaidLabel(node *domain.TreeNode) string {
	parts := []string{strings.ReplaceAll(node.Branch, `"`, "'")}
	for _, kind := range nodeAnnotations(node) {
		switch kind {
		case annVirtual:
			parts = append(parts, "(no worktree)")
		case annPR:
			label := fmt.Sprintf("PR #%d", node.Status.PR.Number)
			if node.Status.PR.State == domain.PRStateMerged || node.Status.PR.State == domain.PRStateClosed {
				label += " " + node.Status.PR.State
			}
			parts = append(parts, label)
		case annAhead:
			parts = append(parts, fmt.Sprintf("↑%d", node.Status.CommitsAhead))
		case annDirty:
			parts = append(parts, "dirty")
		case annNeedsSync:
			parts = append(parts, "⚠ needs sync")
		case annCycle:
			parts = append(parts, "⚠ cycle")
		}
	}
	return strings.Join(parts, " ")
}
