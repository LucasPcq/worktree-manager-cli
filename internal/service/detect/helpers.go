package detect

import "github.com/LucasPcq/wtm/internal/infra"

func fileExists(path string) bool {
	return infra.FileExists(path)
}
