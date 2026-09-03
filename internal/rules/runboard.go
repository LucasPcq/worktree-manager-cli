package rules

import (
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RunBoardParams struct {
	Config domain.RunConfig
	Jobs   []domain.JobInfo
	// Addresses is branch → job → where it answers.
	Addresses map[string]map[string]domain.JobAddress
	Statuses  []domain.WorktreeStatus
	Now       time.Time
}

type RunWorktreeBlock struct {
	Branch string
	Path   string
	Up     int
	Rows   []domain.DetailRow
}

// RunBoard is what the daemon holds up, worktree by worktree. Unlike the detail
// panel's RUN section — which speaks of one worktree and keeps a declared job
// listed as down — this board answers "what is running", so a worktree with
// nothing up has no block and a stopped job has no row.
func RunBoard(params RunBoardParams) []RunWorktreeBlock {
	blocks := make([]RunWorktreeBlock, 0, len(params.Statuses))
	for _, status := range params.Statuses {
		infos := upJobsByName(params.Jobs, status.Path)
		if len(infos) == 0 {
			continue
		}

		addresses := params.Addresses[status.Branch]
		rows := make([]domain.DetailRow, 0, len(infos))
		for _, job := range params.Config.Jobs {
			info, up := infos[job.Name]
			if !up {
				continue
			}
			rows = append(rows, jobRow(jobRowParams{
				Job: job, Info: info, Up: true, Address: addresses[job.Name], Now: params.Now,
			}))
		}
		if len(rows) == 0 {
			continue
		}
		blocks = append(blocks, RunWorktreeBlock{
			Branch: status.Branch, Path: status.Path, Up: len(rows), Rows: rows,
		})
	}
	return blocks
}
