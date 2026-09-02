package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestDecideConcurrency(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ConcurrencyParams
		want   rules.ConcurrencyDecision
	}{
		{
			name:   "nothing runs elsewhere, so there is nothing to decide",
			params: rules.ConcurrencyParams{Config: domain.ConcurrencyExclusive},
			want:   rules.ConcurrencyDecision{Value: domain.ConcurrencyParallel},
		},
		{
			name:   "the flag wins over the config",
			params: rules.ConcurrencyParams{OthersRunning: true, Parallel: true, Config: domain.ConcurrencyExclusive},
			want:   rules.ConcurrencyDecision{Value: domain.ConcurrencyParallel},
		},
		{
			name:   "--exclusive stops the others without asking",
			params: rules.ConcurrencyParams{OthersRunning: true, Exclusive: true},
			want:   rules.ConcurrencyDecision{Value: domain.ConcurrencyExclusive},
		},
		{
			name:   "the config answers once and for all",
			params: rules.ConcurrencyParams{OthersRunning: true, Config: domain.ConcurrencyExclusive},
			want:   rules.ConcurrencyDecision{Value: domain.ConcurrencyExclusive},
		},
		{
			name:   "an unreadable config value is no answer at all",
			params: rules.ConcurrencyParams{OthersRunning: true, Config: domain.Concurrency("sometimes")},
			want:   rules.ConcurrencyDecision{Value: domain.ConcurrencyParallel, Ask: true},
		},
		{
			name:   "with no flag and no config the question is open, and stops nothing meanwhile",
			params: rules.ConcurrencyParams{OthersRunning: true},
			want:   rules.ConcurrencyDecision{Value: domain.ConcurrencyParallel, Ask: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.DecideConcurrency(tc.params); got != tc.want {
				t.Errorf("DecideConcurrency() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidateConcurrency(t *testing.T) {
	for _, value := range []domain.Concurrency{"", domain.ConcurrencyParallel, domain.ConcurrencyExclusive} {
		if errs := rules.ValidateConcurrency(domain.RunConfig{Concurrency: value}); len(errs) != 0 {
			t.Errorf("ValidateConcurrency(%q) = %v, want none", value, errs)
		}
	}

	errs := rules.ValidateConcurrency(domain.RunConfig{Concurrency: "exclusif"})
	if len(errs) != 1 {
		t.Fatalf("ValidateConcurrency = %v, want one error", errs)
	}
}

// A typo must fail the whole config rather than read as "not answered yet":
// otherwise the question comes back at every start with no way to see why.
func TestValidateRunRefusesAnUnknownConcurrency(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:        []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"}},
		Concurrency: "exclusif",
	}

	if _, errs := rules.ValidateRun(cfg); len(errs) == 0 {
		t.Error("ValidateRun accepted an unknown concurrency")
	}
}
