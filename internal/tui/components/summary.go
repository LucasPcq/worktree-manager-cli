package components

import "strings"

// TextSummary returns the value of a TextInputModel step. Use as a Step.Summary.
func TextSummary(model any) string {
	if c, ok := model.(TextInputModel); ok {
		return c.Value()
	}
	return ""
}

// SelectSummary returns the value of a SelectListModel step. Use as a Step.Summary.
func SelectSummary(model any) string {
	if c, ok := model.(SelectListModel); ok {
		return c.Value()
	}
	return ""
}

// OptionalTextSummary returns the value of a TextInputModel step, falling back
// to emptyLabel when the value is empty. Use as a Step.Summary.
func OptionalTextSummary(emptyLabel string) func(any) string {
	return func(model any) string {
		c, ok := model.(TextInputModel)
		if !ok {
			return ""
		}
		if v := c.Value(); v != "" {
			return v
		}
		return emptyLabel
	}
}

// MultiSelectSummary returns the comma-joined values of a MultiSelectModel
// step, falling back to emptyLabel when nothing is selected.
func MultiSelectSummary(emptyLabel string) func(any) string {
	return func(model any) string {
		c, ok := model.(MultiSelectModel)
		if !ok {
			return ""
		}
		vals := c.Values()
		if len(vals) == 0 {
			return emptyLabel
		}
		return strings.Join(vals, ", ")
	}
}

// ConfirmSummary returns yesLabel or noLabel based on the user's choice.
func ConfirmSummary(yesLabel, noLabel string) func(any) string {
	return func(model any) string {
		c, ok := model.(ConfirmModel)
		if !ok {
			return ""
		}
		if c.Confirmed() {
			return yesLabel
		}
		return noLabel
	}
}
