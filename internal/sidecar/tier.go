package sidecar

// tier is the confinement a launch runs in (ADR 0044 point 1).
type tier uint8

const (
	tierNone tier = iota // refused
	tierN                // namespaces
	tierF                // namespace-free
)

func (t tier) String() string { return [...]string{"none", "N", "F"}[t] }
