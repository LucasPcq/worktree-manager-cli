package rules

type LogsJobWindowParams struct {
	Jobs    []string
	Current string
	// Budget is the columns the chips may occupy, marks included.
	Budget int
	// Gap separates two chips, Marks is what one "more this way" mark costs.
	Gap   int
	Marks int
}

// LogsJobRange is the slice of jobs a selection line shows: as many as fit
// around the current one, never wrapping. A wrapping row would change the
// header's height from one worktree to the next, and the tail under it would
// start somewhere new each time.
type LogsJobRange struct {
	Start int
	End   int
}

// LogsJobWindow keeps the current job inside the window and grows outward from
// it, right first — the jobs after it are the ones a reader has not seen yet.
// A budget too small for even one chip still yields that chip: showing none
// would leave the line saying nothing about where the reader is.
func LogsJobWindow(params LogsJobWindowParams) LogsJobRange {
	if len(params.Jobs) == 0 {
		return LogsJobRange{}
	}

	current := 0
	for index, job := range params.Jobs {
		if job == params.Current {
			current = index
			break
		}
	}

	start, end := current, current+1
	used := chipWidth(params.Jobs[current])
	for {
		grown := false
		if end < len(params.Jobs) {
			cost := params.Gap + chipWidth(params.Jobs[end])
			if used+cost+marksCost(marksParams{Start: start, End: end + 1, Total: len(params.Jobs), Mark: params.Marks}) <= params.Budget {
				used, end, grown = used+cost, end+1, true
			}
		}
		if start > 0 {
			cost := params.Gap + chipWidth(params.Jobs[start-1])
			if used+cost+marksCost(marksParams{Start: start - 1, End: end, Total: len(params.Jobs), Mark: params.Marks}) <= params.Budget {
				used, start, grown = used+cost, start-1, true
			}
		}
		if !grown {
			return LogsJobRange{Start: start, End: end}
		}
	}
}

// chipWidth counts the glyph, the space after it and the job's name.
func chipWidth(job string) int { return 2 + len([]rune(job)) }

type marksParams struct {
	Start int
	End   int
	Total int
	Mark  int
}

func marksCost(params marksParams) int {
	cost := 0
	if params.Start > 0 {
		cost += params.Mark
	}
	if params.End < params.Total {
		cost += params.Mark
	}
	return cost
}
