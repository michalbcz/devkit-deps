package diag

import (
	"errors"
	"testing"
)

func TestDiagnosticCarriesExitAndPhase(t *testing.T) {
	d := &Diagnostic{
		Code: "declaration.bom", Message: "UTF-8 BOM is not accepted",
		Language: "lua", Phase: PhaseParse, Line: 1, Exit: 2,
	}
	if d.Error() != "UTF-8 BOM is not accepted" {
		t.Fatalf("Error() = %q", d.Error())
	}
	if ExitCode(d) != 2 {
		t.Fatalf("exit = %d", ExitCode(d))
	}
	if ExitCode(errors.New("disk")) != 4 {
		t.Fatal("plain errors must map to setup/launch failure")
	}
}
