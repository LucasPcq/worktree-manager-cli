package domain

import "time"

// CommitSummary is a history line as the detail panel displays it.
type CommitSummary struct {
	SHA     string
	Subject string
	Author  string
	At      time.Time
}

// DiffStat is the volume of a diff, without the per-file breakdown.
type DiffStat struct {
	Insertions int
	Deletions  int
}
