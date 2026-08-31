package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// applePfConf is the stock macOS /etc/pf.conf, which is what install meets on
// an untouched machine.
const applePfConf = `#
# Default PF configuration file.
#
scrub-anchor "com.apple/*"
nat-anchor "com.apple/*"
rdr-anchor "com.apple/*"
dummynet-anchor "com.apple/*"
anchor "com.apple/*"
load anchor "com.apple" from "/etc/pf.anchors/com.apple"
`

func TestPfConfInsertsBothBlocksInOrder(t *testing.T) {
	got, err := PfConfWithWTM(applePfConf)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(got, "\n")
	rdr := lineIndex(lines, domain.ProxyPfRdrAnchorLine)
	load := lineIndex(lines, domain.ProxyPfLoadLine)
	filter := lineIndex(lines, `anchor "com.apple/*"`)
	appleLoad := lineIndex(lines, `load anchor "com.apple" from "/etc/pf.anchors/com.apple"`)

	if rdr == -1 || load == -1 {
		t.Fatalf("les deux lignes doivent être présentes:\n%s", got)
	}
	if rdr > filter {
		t.Error("rdr-anchor doit précéder les règles de filtre")
	}
	if load < appleLoad {
		t.Error("load anchor va en fin de fichier")
	}
}

func TestPfConfInsertIsIdempotent(t *testing.T) {
	once, err := PfConfWithWTM(applePfConf)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := PfConfWithWTM(once)
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Errorf("une seconde insertion ne doit rien changer:\n%s", twice)
	}
}

func TestPfConfRemovalRestoresTheOriginalByteForByte(t *testing.T) {
	with, err := PfConfWithWTM(applePfConf)
	if err != nil {
		t.Fatal(err)
	}
	if got := PfConfWithoutWTM(with); got != applePfConf {
		t.Errorf("got:\n%q\nwant:\n%q", got, applePfConf)
	}
}

func TestPfConfRemovalOnAVirginFileIsANoOp(t *testing.T) {
	if got := PfConfWithoutWTM(applePfConf); got != applePfConf {
		t.Errorf("got:\n%q", got)
	}
}

func TestPfConfKeepsHandWrittenRulesAround(t *testing.T) {
	custom := applePfConf + "block in proto tcp from any to any port 23\n"
	with, err := PfConfWithWTM(custom)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(with, "block in proto tcp from any to any port 23") {
		t.Error("une règle écrite à la main ne doit pas disparaître")
	}
	if got := PfConfWithoutWTM(with); got != custom {
		t.Errorf("le retrait doit rendre le fichier de départ:\n%q", got)
	}
}

func TestPfConfRefusesAFileWithNoTranslationSection(t *testing.T) {
	if _, err := PfConfWithWTM("set skip on lo0\n"); !errors.Is(err, domain.ErrPfConfUnrecognised) {
		t.Errorf("got %v, want ErrPfConfUnrecognised", err)
	}
}

func TestPfConfHasWTM(t *testing.T) {
	if PfConfHasWTM(applePfConf) {
		t.Error("fichier vierge")
	}
	with, err := PfConfWithWTM(applePfConf)
	if err != nil {
		t.Fatal(err)
	}
	if !PfConfHasWTM(with) {
		t.Error("fichier installé")
	}
}

func lineIndex(lines []string, want string) int {
	for i, line := range lines {
		if line == want {
			return i
		}
	}
	return -1
}
