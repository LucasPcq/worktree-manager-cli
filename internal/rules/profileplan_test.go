package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func profileNames(profiles []domain.ProfileConfig) []string {
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.Name)
	}
	return out
}

func findProfile(t *testing.T, profiles []domain.ProfileConfig, name string) domain.ProfileConfig {
	t.Helper()
	for _, p := range profiles {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no profile %q in %v", name, profileNames(profiles))
	return domain.ProfileConfig{}
}

var (
	composeJob = domain.JobConfig{Name: "docker-compose", Kind: domain.JobKindService, Cwd: "."}
	apiJob     = domain.JobConfig{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api"}
	webJob     = domain.JobConfig{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"}
	lintJob    = domain.JobConfig{Name: "lint", Kind: domain.JobKindTask, Cwd: "."}
)

func TestIsSharedJob(t *testing.T) {
	if !rules.IsSharedJob(composeJob) {
		t.Error("un job à la racine est de l'infra partagée")
	}
	if rules.IsSharedJob(apiJob) {
		t.Error("un job dans un package n'est pas partagé")
	}
}

func TestProposeProfilesGroupsByPackage(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, webJob}},
	})

	if got := profileNames(profiles); len(got) != 3 {
		t.Fatalf("expected 3 profiles (api, web, all), got %v", got)
	}

	api := findProfile(t, profiles, "api")
	if len(api.Jobs) != 2 || api.Jobs[0] != "docker-compose" || api.Jobs[1] != "api-dev" {
		t.Errorf("api = %v, want [docker-compose api-dev]", api.Jobs)
	}

	all := findProfile(t, profiles, domain.ProfileAllName)
	if len(all.Jobs) != 3 {
		t.Errorf("all = %v, want the three jobs", all.Jobs)
	}
	if !all.Default {
		t.Error("le profil global est le default pré-sélectionné dans le picker")
	}
}

func TestProposeProfilesCollapsesInASinglePackageRepo(t *testing.T) {
	// Un profil par package plus un global se confondent quand il n'y a qu'un
	// package : la règle doit dégrader d'elle-même, sans cas particulier.
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, {
			Name: "dev", Kind: domain.JobKindService, Cwd: ".",
		}}},
	})

	if len(profiles) != 1 {
		t.Fatalf("expected a single profile, got %v", profileNames(profiles))
	}
	if !profiles[0].Default {
		t.Error("le profil unique doit être le default")
	}
}

func TestProposeProfilesIgnoresTasks(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, lintJob}},
	})

	all := findProfile(t, profiles, domain.ProfileAllName)
	for _, job := range all.Jobs {
		if job == "lint" {
			t.Error("une task n'entre pas dans un profil proposé")
		}
	}
}

func TestProposeProfilesKeepsTheExistingSplit(t *testing.T) {
	// Sur un init relancé, la proposition est la configuration en place : on
	// montre ce qu'il y a, on n'infère pas par-dessus une composition faite à
	// la main.
	existing := []domain.ProfileConfig{
		{Name: "app1", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
	}

	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, webJob}},
		Existing: existing,
	})

	if len(profiles) != 1 || profiles[0].Name != "app1" {
		t.Fatalf("expected the existing split, got %v", profileNames(profiles))
	}
}
