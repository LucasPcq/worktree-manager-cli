package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

func PfConfHasWTM(content string) bool {
	return strings.Contains(content, domain.ProxyPfRdrMarkStart)
}

// PfConfWithWTM adds the two blocks pf needs, at the two places pf accepts
// them. It refuses rather than guesses on a file with no translation section.
func PfConfWithWTM(content string) (string, error) {
	if PfConfHasWTM(content) {
		return content, nil
	}

	lines := strings.Split(content, "\n")
	at := lastIndexWithPrefix(lines, domain.ProxyPfRdrAnchorPrefix)
	if at == -1 {
		at = lastIndexWithPrefix(lines, domain.ProxyPfNatAnchorPrefix)
	}
	if at == -1 {
		return "", domain.ErrPfConfUnrecognised
	}

	withRdr := append([]string{}, lines[:at+1]...)
	withRdr = append(withRdr, domain.ProxyPfRdrMarkStart, domain.ProxyPfRdrAnchorLine, domain.ProxyPfRdrMarkEnd)
	withRdr = append(withRdr, lines[at+1:]...)

	return appendLoadBlock(strings.Join(withRdr, "\n")), nil
}

func PfConfWithoutWTM(content string) string {
	withoutRdr := dropBlock(content, domain.ProxyPfRdrMarkStart, domain.ProxyPfRdrMarkEnd)
	return dropBlock(withoutRdr, domain.ProxyPfLoadMarkStart, domain.ProxyPfLoadMarkEnd)
}

func appendLoadBlock(content string) string {
	block := strings.Join([]string{domain.ProxyPfLoadMarkStart, domain.ProxyPfLoadLine, domain.ProxyPfLoadMarkEnd, ""}, "\n")
	if strings.HasSuffix(content, "\n") {
		return content + block
	}
	return content + "\n" + block
}

func lastIndexWithPrefix(lines []string, prefix string) int {
	found := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			found = i
		}
	}
	return found
}

func dropBlock(content, start, end string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inside := false
	for _, line := range lines {
		if strings.TrimSpace(line) == start {
			inside = true
			continue
		}
		if inside {
			inside = strings.TrimSpace(line) != end
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
