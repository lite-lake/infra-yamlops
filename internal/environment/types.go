package environment

type CheckStatus int

const (
	CheckStatusOK CheckStatus = iota
	CheckStatusWarning
	CheckStatusError
)

type CheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
	Detail  string
}

type SyncResult struct {
	Name    string
	Success bool
	Message string
	Error   error
}
