package utils

// ResourceType 资源类型
type ResourceType int

const (
	Group ResourceType = iota
	Node
	Policy
	Label
	Rule
)

func (r ResourceType) String() string {
	switch r {
	case Group:
		return "group"
	case Node:
		return "node"
	case Label:
		return "label"
	case Policy:
		return "policy"
	case Rule:
		return "rule"
	default:
		return "Unknown"
	}
}
