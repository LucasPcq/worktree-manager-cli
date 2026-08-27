package rules

import "github.com/LucasPcq/wtm/internal/domain"

type URLCandidatesForParams struct {
	Config domain.RunConfig
}

// URLCandidatesFor is the jobs `run init` offers to publish under their own
// name, every one of them pre-answered yes. A job qualifies by declaring the
// port it listens on, or by already carrying a url — the latter so the step can
// withdraw one, which it could not do if the job were left off the list.
func URLCandidatesFor(params URLCandidatesForParams) []domain.JobURLChoice {
	var choices []domain.JobURLChoice
	for _, job := range params.Config.Jobs {
		port := publishablePortOf(job)
		if port == "" {
			continue
		}
		choices = append(choices, domain.JobURLChoice{Job: job.Name, Port: port, Publish: true})
	}
	return choices
}

// publishablePortOf answers which port a job would be published on, empty for
// one that has nothing to publish. A port already named by a url outranks the
// derived one: the step proposes a port to a job that has none, it never
// redirects a job to another port than the one it was given.
func publishablePortOf(job domain.JobConfig) string {
	if job.URL != nil && job.URL.Port != "" {
		return job.URL.Port
	}
	if job.Kind != domain.JobKindService {
		return ""
	}
	return ListeningPortName(job)
}

// ListeningPortName is the port a job binds, as opposed to the ones it dials.
// Only two names carry that meaning, and they are the two freePortName writes:
// PORT for the first service to claim it, <JOB>_PORT for the ones after. A
// DB_PORT or a REDIS_PORT on the same job is an address it connects to.
func ListeningPortName(job domain.JobConfig) string {
	if _, ok := job.Ports[domain.PortNameDefault]; ok {
		return domain.PortNameDefault
	}
	derived := EnvVarNameFor(job.Name) + "_" + domain.PortNameDefault
	if _, ok := job.Ports[derived]; ok {
		return derived
	}
	return ""
}

type ResolveURLChoicesParams struct {
	Config domain.RunConfig
	// Published names the jobs the reader left checked, and Asked says the
	// question was put at all — an empty selection that was answered withdraws
	// every url, where one that was never asked leaves the proposal standing.
	Published []string
	Asked     bool
}

// ResolveURLChoices is the one reading of the URLs step, shared by the wizard's
// recap and by the run that never showed it. Computing it twice let a
// non-interactive init write what the wizard would not have.
func ResolveURLChoices(params ResolveURLChoicesParams) []domain.JobURLChoice {
	choices := URLCandidatesFor(URLCandidatesForParams{Config: params.Config})
	if !params.Asked {
		return choices
	}

	published := make(map[string]bool, len(params.Published))
	for _, job := range params.Published {
		published[job] = true
	}
	for i := range choices {
		choices[i].Publish = published[choices[i].Job]
	}
	return choices
}
