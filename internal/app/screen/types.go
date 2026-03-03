package screen

// Type identifies which screen or modal is active.
type Type int

const (
	None Type = iota
	Dashboard
	WorktreeCreate
	WorktreeDetail
	Confirm
	Commit
	AgentSelect
	RepoSelect
	Sessions
	Help
	Filter
)

func (t Type) String() string {
	switch t {
	case Dashboard:
		return "dashboard"
	case WorktreeCreate:
		return "create"
	case WorktreeDetail:
		return "detail"
	case Confirm:
		return "confirm"
	case Commit:
		return "commit"
	case AgentSelect:
		return "agent"
	case RepoSelect:
		return "repos"
	case Sessions:
		return "sessions"
	case Help:
		return "help"
	case Filter:
		return "filter"
	default:
		return "none"
	}
}
