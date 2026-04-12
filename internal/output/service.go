package output

import "io"

// ServiceActionResult is a single service's outcome emitted by svc commands.
type ServiceActionResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`            // "started", "stopped", "error"
	Message string `json:"message,omitempty"` // error detail when Status == "error"
}

// PRCheckoutJSON is the payload emitted by `pr checkout --output json`.
type PRCheckoutJSON struct {
	Number int    `json:"number"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// WriteServiceResultsJSON writes the JSON array describing each service outcome.
func WriteServiceResultsJSON(w io.Writer, results []ServiceActionResult) error {
	if results == nil {
		results = []ServiceActionResult{}
	}
	return encodeJSON(w, results)
}

// WriteServiceResultJSON writes a single service outcome (start/stop single service).
func WriteServiceResultJSON(w io.Writer, result ServiceActionResult) error {
	return encodeJSON(w, result)
}

// WritePRCheckoutJSON writes the payload for `pr checkout`.
func WritePRCheckoutJSON(w io.Writer, payload PRCheckoutJSON) error {
	return encodeJSON(w, payload)
}
