package memory

type Scope struct {
	All    bool
	Owner  string
	Shared bool
}

var AllMemories = Scope{All: true}

var SharedOnly = Scope{Shared: true}

func VisibleTo(userID string) Scope {
	return Scope{Owner: userID, Shared: true}
}

func OwnedBy(owner string) Scope {
	if owner == "" {
		return SharedOnly
	}
	return Scope{Owner: owner}
}

func (s Scope) Allows(n Node) bool {
	switch {
	case s.All:
		return true
	case n.Owner == "":
		return s.Shared
	default:
		return s.Owner != "" && n.Owner == s.Owner
	}
}
