package diag

import "errors"

type Phase string

const (
	PhaseCLI      Phase = "cli"
	PhaseParse    Phase = "parse"
	PhaseValidate Phase = "validate"
	PhaseDiscover Phase = "discover"
	PhasePlan     Phase = "plan"
	PhasePrepare  Phase = "prepare"
	PhaseLaunch   Phase = "launch"
)

type Diagnostic struct {
	Code         string
	Message      string
	Language     string
	Phase        Phase
	MissingTools []string
	Line         int
	Exit         int
}

func (d *Diagnostic) Error() string { return d.Message }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var d *Diagnostic
	if errors.As(err, &d) && d != nil {
		return d.Exit
	}
	return 4
}
