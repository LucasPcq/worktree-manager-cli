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
	Action  RequestAction        `json:"action"`
	Service *domain.ServiceConfig `json:"service,omitempty"`
	Name    string               `json:"name,omitempty"`
	WorkDir string               `json:"work_dir,omitempty"`
	Cols    int                  `json:"cols,omitempty"`
	Rows    int                  `json:"rows,omitempty"`
}

// ResponseStatus is the status field in a daemon response.
type ResponseStatus string

const (
	StatusOK    ResponseStatus = "ok"
	StatusError ResponseStatus = "error"
)

// ServiceInfo is the JSON representation of a managed service.
type ServiceInfo struct {
	Name    string               `json:"name"`
	Status  domain.ServiceStatus `json:"status"`
	PID     int                  `json:"pid"`
	WorkDir string               `json:"work_dir"`
}

// Response is a JSON message sent from daemon to client.
type Response struct {
	Status   ResponseStatus `json:"status"`
	Message  string         `json:"message,omitempty"`
	Services []ServiceInfo  `json:"services,omitempty"`
}
