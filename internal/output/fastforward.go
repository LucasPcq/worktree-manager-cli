package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// FormatFastForwardResults prints one line per branch. Raw body: the command's
// frame owns the outer vertical padding.
func FormatFastForwardResults(w io.Writer, results []domain.FastForwardResult) {
	SectionTitle(w, domain.FastForwardHeader)
	for _, result := range results {
		InfoLine(w, result.Branch, fastForwardLine(result))
	}
}

func fastForwardLine(result domain.FastForwardResult) string {
	if result.Status == domain.FFFailed {
		return fmt.Sprintf(domain.SyncLabelErrorFmt, result.Detail)
	}
	return rules.FastForwardStatusLabel(result.Status)
}

func WriteFastForwardJSON(w io.Writer, results []domain.FastForwardResult) error {
	if results == nil {
		results = []domain.FastForwardResult{}
	}
	return encodeJSON(w, results)
}
