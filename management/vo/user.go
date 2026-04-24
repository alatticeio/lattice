package vo

type UserVo struct {
	ID          string        `json:"id,omitempty"`
	Username    string        `json:"name,omitempty"`
	Email       string        `json:"email,omitempty"`
	MobilePhone string        `json:"mobilePhone,omitempty"`
	Avatar      string        `json:"avatar,omitempty"`
	Address     string        `json:"address,omitempty"`
	Role        string        `json:"role,omitempty"`
	Workspaces  []WorkspaceVo `json:"workspaces,omitempty"`
	// Enriched fields for the admin user list
	Source      string `json:"source,omitempty"`      // e.g. "local", "github", "dex"
	InviterName string `json:"inviterName,omitempty"` // set when the user joined via invitation
	RegisteredAt string `json:"registeredAt,omitempty"` // ISO-8601 creation time
}

type InviteVo struct {
	ID           string `json:"id"`
	InviteeName  string `json:"inviteeName,omitempty"`
	InviterName  string `json:"inviterName,omitempty"`
	MobilePhone  string `json:"mobilePhone,omitempty"`
	Email        string `json:"email,omitempty"`
	Role         string `json:"role,omitempty"`
	Avatar       string `json:"avatar,omitempty"`
	GroupId      uint64 `json:"groupId,omitempty"`
	GroupName    string `json:"groupName,omitempty"`
	Permissions  string `json:"permissions,omitempty"`
	AcceptStatus string `json:"acceptStatus,omitempty"`
}

type InvitationVo struct {
	ID            uint64 `json:"id,string"`
	Group         string `json:"group,omitempty"`
	InviterName   string `json:"inviterName,omitempty"`
	InviterAvatar string `json:"inviterAvatar,omitempty"`
	InviteId      uint64 `json:"inviteId,string"`
	Role          string `json:"role,omitempty"`
	Permissions   string `json:"permissions,omitempty"`
	AcceptStatus  string `json:"acceptStatus,omitempty"`
}
