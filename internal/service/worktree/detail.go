package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// DetailParams holds inputs for loading worktree details.
type DetailParams struct {
	WorktreePath string
	ProjectDir   string
	StateDir     string
	Branch       string
	BaseBranch   string
}

// DetailResult holds all detail information for a worktree.
type DetailResult struct {
	Branch          string
	Path            string
	SourceBranch    string
	ModifiedFiles   []infra.ModifiedFile
	UnpushedCommits int
	ContextNotes    string
}

// Detail loads all detail information for a worktree.
func Detail(params DetailParams) DetailResult {
	result := DetailResult{
		Branch: params.Branch,
		Path:   params.WorktreePath,
	}

	result.SourceBranch = loadSourceBranch(params.StateDir, params.Branch)
	result.ModifiedFiles, _ = infra.ListModifiedFiles(infra.ListModifiedFilesParams{
		WorktreePath: params.WorktreePath,
	})
	result.UnpushedCommits, _ = infra.UnpushedCommits(infra.UnpushedCommitsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	result.ContextNotes = loadContextNotes(params.StateDir, params.Branch)

	return result
}

func loadSourceBranch(stateDir, branch string) string {
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ""
	}
	var meta domain.WorktreeMetadata
	if json.Unmarshal(data, &meta) != nil {
		return ""
	}
	return meta.SourceBranch
}

func loadContextNotes(stateDir, branch string) string {
	contextPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.ContextFileName)
	data, err := os.ReadFile(contextPath)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		lines = lines[:10]
		lines = append(lines, "…")
	}

	return strings.Join(lines, "\n")
}
