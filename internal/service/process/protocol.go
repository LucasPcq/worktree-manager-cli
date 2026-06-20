package process

import "github.com/LucasPcq/wtm/internal/domain"

// RequestAction identifies the daemon command.
type RequestAction string

const (
	ActionStart   RequestAction = "start"
	ActionStop    RequestAction = "stop"
	ActionStopAll RequestAction = "stop_all"
	ActionList    RequestAction = "list"
	ActionAttach  RequestAction = "attach"
)

// Request is a JSON message sent from client to daemon.
type Request struct {
	Action  RequestAction     `json:"action"`
	Job     *domain.JobConfig `json:"job,omitempty"`
	Name    string            `json:"name,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	Cols    int               `json:"cols,omitempty"`
	Rows    int               `json:"rows,omitempty"`
}

// ResponseStatus is the status field in a daemon response. For long-lived
// actions like launching a task, the daemon emits multiple NDJSON responses:
// zero or more StatusOutput with Data chunks, followed by StatusDone (success)
// or StatusError (failure). Simple actions end with a single StatusOK.
type ResponseStatus string

const (
	StatusOK     ResponseStatus = "ok"
	StatusError  ResponseStatus = "error"
	StatusOutput ResponseStatus = "output" // streamed chunk of task output
	StatusDone   ResponseStatus = "done"   // task exited successfully
)

// Response is a JSON message sent from daemon to client.
type Response struct {
	Status   ResponseStatus   `json:"status"`
	Message  string           `json:"message,omitempty"`
	Jobs     []domain.JobInfo `json:"jobs,omitempty"`
	Data     []byte           `json:"data,omitempty"`      // task output chunk
	ExitCode *int             `json:"exit_code,omitempty"` // final task exit code
}
