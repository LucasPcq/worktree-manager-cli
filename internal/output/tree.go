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

// formatTreeAnnotations builds the trailing status string for a node, in order:
// virtual marker, PR, commits-ahead, dirty, needs-sync, cycle.
func formatTreeAnnotations(node *domain.TreeNode) string {
	parts := make([]string, 0, 5)

	if node.IsVirtual {
		parts = append(parts, styles.Muted.Render("(no worktree)"))
	}
	if pr := formatTreePR(node.Status.PR); pr != "" {
		parts = append(parts, pr)
	}
	if node.Status.CommitsAhead > 0 {
		parts = append(parts, styles.Muted.Render(fmt.Sprintf("↑%d", node.Status.CommitsAhead)))
	}
	if node.Status.IsDirty {
		parts = append(parts, styles.Warning.Render("● dirty"))
	}
	if node.Status.NeedsSync {
		parts = append(parts, styles.Warning.Render("⚠ needs sync"))
	}
	if node.Status.InCycle {
		parts = append(parts, styles.Warning.Render("⚠ cycle"))
	}

	return strings.Join(parts, "  ")
}

func formatTreePR(pr *domain.WorktreeListPR) string {
	if pr == nil {
		return ""
	}
	label := fmt.Sprintf("PR #%d", pr.Number)
	switch pr.State {
	case "merged":
		return styles.Muted.Render(label + " merged")
	case "closed":
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

	if node.IsVirtual {
		parts = append(parts, "(no worktree)")
	}
	if node.Status.PR != nil {
		label := fmt.Sprintf("PR #%d", node.Status.PR.Number)
		if node.Status.PR.State == "merged" || node.Status.PR.State == "closed" {
			label += " " + node.Status.PR.State
		}
		parts = append(parts, label)
	}
	if node.Status.CommitsAhead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", node.Status.CommitsAhead))
	}
	if node.Status.IsDirty {
		parts = append(parts, "dirty")
	}
	if node.Status.NeedsSync {
		parts = append(parts, "⚠ needs sync")
	}
	if node.Status.InCycle {
		parts = append(parts, "⚠ cycle")
	}

	return strings.Join(parts, " ")
}
