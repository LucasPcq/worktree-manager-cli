package rules

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestTemplateTarget(t *testing.T) {
	cases := []struct {
		name       string
		wantTarget string
		wantSuffix string
		wantOK     bool
	}{
		{".env.example", ".env", ".example", true},
		{".env.dist", ".env", ".dist", true},
		{".env.sample", ".env", ".sample", true},
		{".env.template", ".env", ".template", true},
		{".env.tmpl", ".env", ".tmpl", true},
		{".env", "", "", false},
		{".env.local", "", "", false},
		{".env.production", "", "", false},
		{".env.production.example", "", "", false},
		{"config.yaml", "", "", false},
	}
	for _, c := range cases {
		target, suffix, ok := TemplateTarget(c.name)
		if target != c.wantTarget || suffix != c.wantSuffix || ok != c.wantOK {
			t.Errorf("TemplateTarget(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.name, target, suffix, ok, c.wantTarget, c.wantSuffix, c.wantOK)
		}
	}
}

func TestTemplateCandidates(t *testing.T) {
	got := TemplateCandidates(".env")
	want := []string{".env.example", ".env.dist", ".env.sample", ".env.template", ".env.tmpl"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TemplateCandidates(.env) = %v, want %v", got, want)
	}
	if got := TemplateCandidates("apps/api/.env")[0]; got != "apps/api/.env.example" {
		t.Errorf("nested first candidate = %q, want apps/api/.env.example", got)
	}
}

func TestIsLocalEnvFile(t *testing.T) {
	if !IsLocalEnvFile(".env.local") {
		t.Error("IsLocalEnvFile(.env.local) = false, want true")
	}
	if IsLocalEnvFile(".env") {
		t.Error("IsLocalEnvFile(.env) = true, want false")
	}
}

func TestClassifyEnvFilesValueAndTemplate(t *testing.T) {
	got := ClassifyEnvFiles([]string{".env", ".env.example"})
	want := []domain.EnvFile{{Target: ".env", Template: ".env.example"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClassifyEnvFilesLoneTemplate(t *testing.T) {
	got := ClassifyEnvFiles([]string{".env.example"})
	want := []domain.EnvFile{{Target: ".env", Template: ".env.example"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClassifyEnvFilesLoneValue(t *testing.T) {
	got := ClassifyEnvFiles([]string{".env"})
	want := []domain.EnvFile{{Target: ".env"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClassifyEnvFilesLocal(t *testing.T) {
	got := ClassifyEnvFiles([]string{".env", ".env.local"})
	want := []domain.EnvFile{
		{Target: ".env"},
		{Target: ".env.local", Local: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClassifyEnvFilesTemplatePriority(t *testing.T) {
	// .dist listed before .example on disk, but .example wins by priority.
	got := ClassifyEnvFiles([]string{".env.dist", ".env.example"})
	want := []domain.EnvFile{{Target: ".env", Template: ".env.example"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClassifyEnvFilesIgnoresMultiEnv(t *testing.T) {
	got := ClassifyEnvFiles([]string{".env.production", ".env.staging", ".env.production.example"})
	if len(got) != 0 {
		t.Errorf("multi-env files should be ignored, got %+v", got)
	}
}

func TestClassifyEnvFilesNested(t *testing.T) {
	got := ClassifyEnvFiles([]string{".env", ".env.example", "apps/api/.env.dist"})
	want := []domain.EnvFile{
		{Target: ".env", Template: ".env.example"},
		{Target: "apps/api/.env", Template: "apps/api/.env.dist"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestEnvTargets(t *testing.T) {
	files := []domain.EnvFile{{Target: ".env"}, {Target: ".env.local", Local: true}}
	got := EnvTargets(files)
	want := []string{".env", ".env.local"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EnvTargets = %v, want %v", got, want)
	}
}
