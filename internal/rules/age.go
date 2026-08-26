package rules

import (
	"fmt"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RelativeAgeParams struct {
	At  time.Time
	Now time.Time
}

// RelativeAge rend un âge lisible d'un coup d'œil. Une date future (horloge
// décalée, commit rejoué) se lit "just now" plutôt que de compter à l'envers.
func RelativeAge(params RelativeAgeParams) string {
	if params.At.IsZero() {
		return ""
	}

	elapsed := params.Now.Sub(params.At)
	switch {
	case elapsed < time.Minute:
		return domain.AgeJustNow
	case elapsed < time.Hour:
		return fmt.Sprintf(domain.AgeMinFmt, int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf(domain.AgeHourFmt, int(elapsed.Hours()))
	case elapsed < 7*24*time.Hour:
		return fmt.Sprintf(domain.AgeDayFmt, int(elapsed.Hours()/24))
	default:
		return fmt.Sprintf(domain.AgeWeekFmt, int(elapsed.Hours()/(24*7)))
	}
}

type FetchStalenessParams struct {
	FetchedAt time.Time
	Now       time.Time
}

// FetchIsStale dit si les refs origin sont assez vieilles pour que la vue le
// déclare. Jamais fetché compte comme périmé : c'est le cas le plus trompeur.
func FetchIsStale(params FetchStalenessParams) bool {
	if params.FetchedAt.IsZero() {
		return true
	}
	return params.Now.Sub(params.FetchedAt) > domain.FetchStaleAfter
}
