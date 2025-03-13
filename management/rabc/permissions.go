package rbac

const (
	// 分组权限
	//PermCreateGroup = "group:create"
	PermDeleteGroup = "group:delete"
	PermUpdateGroup = "group:update"
	PermViewGroup   = "group:view"

	// 节点权限
	PermAddNode     = "node:add"
	PermRemoveNode  = "node:remove"
	PermUpdateNode  = "node:update"
	PermConnectNode = "node:connect"

	// policy
	PermAddPolicy    = "policy:add"
	PermRemovePolicy = "policy:remove"
	PermUpdatePolicy = "policy:update"
	PermViewPolicy   = "policy:view"

	// 成员权限
	PermManageMembers = "members:manage"
	PermViewMembers   = "members:view"
)

type AccessPolicyType string

const (
	NODE AccessPolicyType = "node"
	TAG  AccessPolicyType = "tag"
	IP   AccessPolicyType = "ip"
)

// 角色权限映射
var RolePermissions = map[string][]string{
	"admin": {
		PermDeleteGroup, PermUpdateGroup, PermViewGroup,
		PermAddNode, PermRemoveNode, PermUpdateNode, PermConnectNode, PermAddPolicy,
		PermRemovePolicy, PermUpdatePolicy, PermManageMembers, PermViewMembers,
	},
	"member": {
		PermUpdateGroup, PermViewGroup,
		PermAddNode, PermRemoveNode, PermUpdateNode, PermConnectNode, PermAddPolicy, PermRemovePolicy,
		PermUpdatePolicy, PermManageMembers, PermViewMembers,
	},
	"guest": {
		PermViewGroup,
		PermConnectNode,
		PermViewMembers,
		PermViewPolicy,
	},
}
