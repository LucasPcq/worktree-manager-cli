package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestIsRunInitialized(t *testing.T) {
	cases := []struct {
		name string
		cfg  domain.RunConfig
		want bool
	}{
		{
			name: "empty is not initialized",
			cfg:  domain.RunConfig{},
			want: false,
		},
		{
			name: "one job is initialized",
			cfg: domain.RunConfig{
				Jobs: []domain.JobConfig{{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"}},
			},
			want: true,
		},
		{
			name: "one profile is initialized",
			cfg: domain.RunConfig{
				Profiles: []domain.ProfileConfig{{Name: "all", Jobs: []string{"dev"}}},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRunInitialized(tc.cfg); got != tc.want {
				t.Errorf("IsRunInitialized() = %v, want %v", got, tc.want)
			}
		})
	}
}
