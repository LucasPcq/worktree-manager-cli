package process

import (
	"maps"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// jobPorts resolves the ports a job binds in the worktree its request describes.
// The offset is read back from the environment the client resolved rather than
// carried as its own field, so the number the daemon adds and the one the job
// reads can never drift apart.
func jobPorts(job domain.JobConfig, env map[string]string) map[string]int {
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		offset = 0
	}
	return rules.JobPorts(rules.JobPortsParams{Ports: job.Ports, PortOffset: offset})
}

// withJobPorts layers a job's resolved ports onto the worktree environment. A
// declaration in run.toml is an explicit instruction, so it wins over whatever
// the daemon inherited for that name — without that, the isolation the ports
// exist for would not happen.
func withJobPorts(job domain.JobConfig, env map[string]string) map[string]string {
	ports := jobPorts(job, env)
	if len(ports) == 0 {
		return env
	}

	merged := make(map[string]string, len(env)+len(ports))
	maps.Copy(merged, env)
	for name, port := range ports {
		merged[name] = strconv.Itoa(port)
	}
	return merged
}
