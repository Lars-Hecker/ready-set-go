package perm

// Slack-style permissions: each permission has a minimum role level.
// Higher roles (lower numbers) automatically have all permissions of lower roles.
// Workspaces can customize the minimum role per permission in their prefs.

// Role levels (lower = more privileged).
const (
	LevelPrimaryOwner int16 = 0
	LevelOwner        int16 = 1
	LevelAdmin        int16 = 2
	LevelMember       int16 = 3
	LevelGuest        int16 = 4
)

// Permission keys for workspace prefs.
const (
	PermWorkspaceRead     = "workspace:read"
	PermWorkspaceUpdate   = "workspace:update"
	PermWorkspaceDelete   = "workspace:delete"
	PermWorkspaceTransfer = "workspace:transfer"
	PermMembersRead       = "members:read"
	PermMembersInvite     = "members:invite"
	PermMembersUpdate     = "members:update"
	PermMembersRemove     = "members:remove"
)

// Defaults maps permission keys to their default minimum role level.
var Defaults = map[string]int16{
	PermWorkspaceRead:     LevelGuest,
	PermMembersRead:       LevelMember,
	PermMembersInvite:     LevelAdmin,
	PermMembersUpdate:     LevelAdmin,
	PermWorkspaceUpdate:   LevelOwner,
	PermMembersRemove:     LevelOwner,
	PermWorkspaceDelete:   LevelPrimaryOwner,
	PermWorkspaceTransfer: LevelPrimaryOwner,
}

// Can checks if userRole can perform a permission given the minRole setting.
func Can(userRole, minRole int16) bool {
	return userRole <= minRole
}

// MinRole returns the minimum role for a permission, using workspace override or default.
func MinRole(perm string, workspacePerms map[string]int16) int16 {
	if level, ok := workspacePerms[perm]; ok {
		return level
	}
	if level, ok := Defaults[perm]; ok {
		return level
	}
	return LevelPrimaryOwner // deny by default
}
