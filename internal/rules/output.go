package rules

import "github.com/LucasPcq/wtm/internal/domain"

// IsHumanFormat reports whether the output format targets a human reader and
// therefore must be framed with the canonical top/bottom blank lines. JSON is
// the only machine format and is always emitted flush, without decorative
// padding.
func IsHumanFormat(format string) bool {
	return format != domain.OutputJSON
}
