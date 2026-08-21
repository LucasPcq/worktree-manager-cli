package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ComposePortVarNameParams struct {
	Service   string
	Container int
	Taken     map[string]bool
}

// ComposePortVarName is the variable wtm introduces for a frozen mapping:
// <SERVICE>_PORT, then the container port for a service exposing several.
func ComposePortVarName(params ComposePortVarNameParams) string {
	base := normalizeComposeVarSegment(params.Service) + domain.ComposePortVarSuffix
	if !params.Taken[base] {
		return base
	}

	withContainer := fmt.Sprintf("%s_%d", base, params.Container)
	if !params.Taken[withContainer] {
		return withContainer
	}

	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s_%d", withContainer, n)
		if !params.Taken[candidate] {
			return candidate
		}
	}
}

func normalizeComposeVarSegment(service string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(service) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	name := strings.Trim(collapseUnderscores(b.String()), "_")
	if name == "" {
		return domain.ComposeVarNamePrefix
	}
	if name[0] >= '0' && name[0] <= '9' {
		return domain.ComposeVarNamePrefix + name
	}
	return name
}

func collapseUnderscores(s string) string {
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

type ComposeShortPortParams struct {
	Service string
	// Mapping is the scalar as the file spells it, quotes already stripped:
	// "5432:5432", "127.0.0.1:8025:8025", "${DB_PORT:-5432}:5432".
	Mapping string
	Taken   map[string]bool
}

// ComposeShortPort reads one entry of a service's `ports:` list in short syntax
// and says what wtm may do with it. Positions in the source file are the
// scanner's job; everything else is decided here.
func ComposeShortPort(params ComposeShortPortParams) domain.ComposePortBinding {
	binding := domain.ComposePortBinding{Service: params.Service}

	fields := splitComposeMapping(params.Mapping)
	if len(fields) < 2 || len(fields) > 3 {
		return unsupportedComposePort(binding, domain.ComposePortReasonNoHost)
	}

	ip := ""
	if len(fields) == 3 {
		ip = fields[0]
		fields = fields[1:]
	}
	host, container := fields[0], fields[1]

	containerPort, ok := parseComposeContainerPort(container)
	if !ok {
		return unsupportedComposePort(binding, composeContainerReason(container))
	}
	binding.Container = containerPort

	resolved := resolveComposeHostPort(resolveComposeHostPortParams{
		Service:   params.Service,
		Host:      host,
		Container: containerPort,
		Taken:     params.Taken,
	})
	if resolved.reason != "" {
		return unsupportedComposePort(binding, resolved.reason)
	}

	binding.Status = resolved.status
	binding.Var = resolved.varName
	binding.Base = resolved.base
	if resolved.status == domain.ComposePortFrozen {
		binding.Replacement = renderComposeShortMapping(renderComposeShortMappingParams{
			IP:        ip,
			Var:       resolved.varName,
			Base:      resolved.base,
			Container: container,
		})
	}
	return binding
}

type ComposeLongPortParams struct {
	Service string
	// Published and Target are the two scalars of a long-syntax entry, as the
	// file spells them. Published carries the host port and is the only one wtm
	// ever rewrites.
	Published string
	Target    string
	Taken     map[string]bool
}

// ComposeLongPort is ComposeShortPort for the long syntax, where the host port
// stands alone under `published:` and needs no mapping to be reassembled.
func ComposeLongPort(params ComposeLongPortParams) domain.ComposePortBinding {
	binding := domain.ComposePortBinding{Service: params.Service}

	if strings.TrimSpace(params.Published) == "" {
		return unsupportedComposePort(binding, domain.ComposePortReasonNoHost)
	}

	containerPort, ok := parseComposeContainerPort(params.Target)
	if !ok {
		return unsupportedComposePort(binding, composeContainerReason(params.Target))
	}
	binding.Container = containerPort

	resolved := resolveComposeHostPort(resolveComposeHostPortParams{
		Service:   params.Service,
		Host:      params.Published,
		Container: containerPort,
		Taken:     params.Taken,
	})
	if resolved.reason != "" {
		return unsupportedComposePort(binding, resolved.reason)
	}

	binding.Status = resolved.status
	binding.Var = resolved.varName
	binding.Base = resolved.base
	if resolved.status == domain.ComposePortFrozen {
		binding.Replacement = strconv.Quote(fmt.Sprintf(domain.ComposeTemplatedPortFmt, resolved.varName, resolved.base))
	}
	return binding
}

type resolveComposeHostPortParams struct {
	Service   string
	Host      string
	Container int
	Taken     map[string]bool
}

type resolvedComposeHostPort struct {
	status  domain.ComposePortStatus
	varName string
	base    int
	reason  string
}

// resolveComposeHostPort is the one place that decides what a host port field
// is: a variable the project already templated, a literal wtm can template, or
// something it refuses to touch.
func resolveComposeHostPort(params resolveComposeHostPortParams) resolvedComposeHostPort {
	host := strings.TrimSpace(params.Host)

	if strings.Contains(host, "$") {
		name, fallback, ok := parseComposeVarReference(host)
		if !ok {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonNotAPort, host)}
		}
		if !IsEnvVarName(name) {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonBadVarName, name)}
		}

		base := params.Container
		if fallback != "" {
			parsed, err := strconv.Atoi(fallback)
			if err != nil {
				return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonNotAPort, fallback)}
			}
			base = parsed
		}
		if base < domain.PortMin || base > domain.PortMax {
			return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonOutOfRange, base, domain.PortMin, domain.PortMax)}
		}
		return resolvedComposeHostPort{status: domain.ComposePortTemplated, varName: name, base: base}
	}

	if isComposePortRange(host) {
		return resolvedComposeHostPort{reason: domain.ComposePortReasonRange}
	}

	base, err := strconv.Atoi(host)
	if err != nil {
		return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonNotAPort, host)}
	}
	if base < domain.PortMin || base > domain.PortMax {
		return resolvedComposeHostPort{reason: fmt.Sprintf(domain.ComposePortReasonOutOfRange, base, domain.PortMin, domain.PortMax)}
	}

	return resolvedComposeHostPort{
		status: domain.ComposePortFrozen,
		base:   base,
		varName: ComposePortVarName(ComposePortVarNameParams{
			Service:   params.Service,
			Container: params.Container,
			Taken:     params.Taken,
		}),
	}
}

func unsupportedComposePort(binding domain.ComposePortBinding, reason string) domain.ComposePortBinding {
	binding.Status = domain.ComposePortUnsupported
	binding.Reason = reason
	binding.Var = ""
	binding.Replacement = ""
	return binding
}

// splitComposeMapping splits on ":" while keeping "${VAR:-default}" whole.
func splitComposeMapping(mapping string) []string {
	var fields []string
	depth, start := 0, 0
	for i := 0; i < len(mapping); i++ {
		switch mapping[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				fields = append(fields, mapping[start:i])
				start = i + 1
			}
		}
	}
	return append(fields, mapping[start:])
}

// parseComposeVarReference reads "${NAME:-default}", "${NAME-default}",
// "${NAME}" or "$NAME", returning the name and the default when there is one.
func parseComposeVarReference(host string) (name, fallback string, ok bool) {
	if !strings.HasPrefix(host, "$") {
		return "", "", false
	}
	body := host[1:]

	if !strings.HasPrefix(body, "{") {
		return body, "", body != ""
	}
	if !strings.HasSuffix(body, "}") {
		return "", "", false
	}
	body = body[1 : len(body)-1]

	if n, f, found := strings.Cut(body, ":-"); found {
		return n, f, true
	}
	if n, f, found := strings.Cut(body, "-"); found {
		return n, f, true
	}
	return body, "", body != ""
}

// parseComposeContainerPort reads the container side, tolerating the "/udp"
// suffix compose allows there.
func parseComposeContainerPort(field string) (int, bool) {
	field = strings.TrimSpace(field)
	if proto := strings.IndexByte(field, '/'); proto >= 0 {
		field = field[:proto]
	}
	port, err := strconv.Atoi(field)
	if err != nil || port < domain.PortMin || port > domain.PortMax {
		return 0, false
	}
	return port, true
}

func composeContainerReason(field string) string {
	if isComposePortRange(field) {
		return domain.ComposePortReasonRange
	}
	return fmt.Sprintf(domain.ComposePortReasonNotAPort, field)
}

func isComposePortRange(field string) bool {
	low, high, found := strings.Cut(strings.TrimSpace(field), "-")
	if !found {
		return false
	}
	if proto := strings.IndexByte(high, '/'); proto >= 0 {
		high = high[:proto]
	}
	_, lowErr := strconv.Atoi(low)
	_, highErr := strconv.Atoi(high)
	return lowErr == nil && highErr == nil
}

type renderComposeShortMappingParams struct {
	IP        string
	Var       string
	Base      int
	Container string
}

// renderComposeShortMapping rewrites a whole mapping rather than splicing
// inside it: the host IP and the container side are carried over verbatim, and
// the result is always quoted, which is valid whatever the source style was.
func renderComposeShortMapping(params renderComposeShortMappingParams) string {
	host := fmt.Sprintf(domain.ComposeTemplatedPortFmt, params.Var, params.Base)
	mapping := host + ":" + params.Container
	if params.IP != "" {
		mapping = params.IP + ":" + mapping
	}
	return strconv.Quote(mapping)
}

type ApplyComposePortPatchesParams struct {
	Content  string
	Bindings []domain.ComposePortBinding
}

// ApplyComposePortPatches rewrites the host port of each binding in place,
// addressing it by the line and column the scan recorded. The file is never
// re-serialized, so comments, indentation, anchors and quoting style survive
// untouched. A token that is no longer where it was scanned aborts the whole
// patch rather than overwriting whatever took its place.
func ApplyComposePortPatches(params ApplyComposePortPatchesParams) (string, error) {
	targets := make([]domain.ComposePortBinding, 0, len(params.Bindings))
	for _, b := range params.Bindings {
		if b.Replacement != "" {
			targets = append(targets, b)
		}
	}
	if len(targets) == 0 {
		return params.Content, nil
	}

	// Descending, so splicing one token never shifts the next one's column.
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Line != targets[j].Line {
			return targets[i].Line > targets[j].Line
		}
		return targets[i].Column > targets[j].Column
	})

	lines := strings.Split(params.Content, "\n")
	for _, b := range targets {
		if b.Line < 1 || b.Line > len(lines) {
			return "", fmt.Errorf(domain.ComposePatchOutOfRangeFmt, b.File, b.Line)
		}

		line := lines[b.Line-1]
		start := b.Column - 1
		end := start + len(b.Token)
		if start < 0 || end > len(line) || line[start:end] != b.Token {
			return "", fmt.Errorf(domain.ComposePatchMovedFmt, b.File, b.Line, b.Token, composeFoundAt(line, start, len(b.Token)))
		}

		lines[b.Line-1] = line[:start] + b.Replacement + line[end:]
	}

	return strings.Join(lines, "\n"), nil
}

// composeFoundAt quotes what actually sits where a token was expected, so the
// abort message shows the mismatch instead of only asserting it.
func composeFoundAt(line string, start, length int) string {
	if start < 0 || start >= len(line) {
		return "end of line"
	}
	end := start + length
	if end > len(line) {
		end = len(line)
	}
	return strconv.Quote(line[start:end])
}

// SortedComposeFiles orders a per-file map so a run's output and its rewrites
// follow the same sequence twice in a row.
func SortedComposeFiles[T any](byFile map[string]T) []string { return sortedKeys(byFile) }

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
