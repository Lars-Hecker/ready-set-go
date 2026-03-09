package workspace

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	workspacepb "baseapp/gen/api/workspace"
	"baseapp/gen/api/workspace/workspacepbconnect"
	"baseapp/gen/db"
	"baseapp/infra/auth"
	"baseapp/infra/perm"
)

type Handler struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

var _ workspacepbconnect.WorkspaceServiceHandler = (*Handler)(nil)

func NewHandler(pool *pgxpool.Pool, q *db.Queries) *Handler {
	return &Handler{pool: pool, q: q}
}

// CreateWorkspace creates a new workspace and assigns the creator as primary owner.
func (h *Handler) CreateWorkspace(ctx context.Context, req *connect.Request[workspacepb.CreateWorkspaceRequest]) (*connect.Response[workspacepb.CreateWorkspaceResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name required"))
	}
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug required"))
	}

	var ws db.Workspace
	var membership db.WorkspaceMembership

	err := pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		qtx := h.q.WithTx(tx)

		var err error
		ws, err = qtx.CreateWorkspace(ctx, db.CreateWorkspaceParams{
			Name: req.Msg.Name,
			Slug: req.Msg.Slug,
		})
		if err != nil {
			return err
		}

		membership, err = qtx.CreateMembership(ctx, db.CreateMembershipParams{
			UserID:      userID,
			WorkspaceID: ws.ID,
			Role:        perm.LevelPrimaryOwner,
			Status:      db.MembershipStatusActive,
			CreatedBy:   pgtype.UUID{Bytes: userID, Valid: true},
		})
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("slug already taken"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create workspace"))
	}

	return connect.NewResponse(&workspacepb.CreateWorkspaceResponse{
		Workspace:  workspaceToProto(&ws),
		Membership: membershipToProto(&membership),
	}), nil
}

func (h *Handler) GetWorkspace(ctx context.Context, req *connect.Request[workspacepb.GetWorkspaceRequest]) (*connect.Response[workspacepb.GetWorkspaceResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membership, err := h.q.GetMembership(ctx, db.GetMembershipParams{UserID: userID, WorkspaceID: wsID})
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not a member"))
	}
	if membership.Status != db.MembershipStatusActive {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("membership not active"))
	}

	ws, err := h.q.GetWorkspaceByID(ctx, wsID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(membership.Role, perm.MinRole(perm.PermWorkspaceRead, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions"))
	}

	return connect.NewResponse(&workspacepb.GetWorkspaceResponse{
		Workspace:    workspaceToProto(&ws),
		MyMembership: membershipToProto(&membership),
	}), nil
}

func (h *Handler) UpdateWorkspace(ctx context.Context, req *connect.Request[workspacepb.UpdateWorkspaceRequest]) (*connect.Response[workspacepb.UpdateWorkspaceResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membership, ws, err := h.getMembershipAndWorkspace(ctx, userID, wsID)
	if err != nil {
		return nil, err
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(membership.Role, perm.MinRole(perm.PermWorkspaceUpdate, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions"))
	}

	ws, err = h.q.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		ID:   wsID,
		Name: pgtextPtr(req.Msg.Name),
		Slug: pgtextPtr(req.Msg.Slug),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("slug already taken"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to update workspace"))
	}

	return connect.NewResponse(&workspacepb.UpdateWorkspaceResponse{
		Workspace: workspaceToProto(&ws),
	}), nil
}

// DeleteWorkspace soft-deletes the workspace. Only primary owner can do this.
func (h *Handler) DeleteWorkspace(ctx context.Context, req *connect.Request[workspacepb.DeleteWorkspaceRequest]) (*connect.Response[workspacepb.DeleteWorkspaceResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membership, ws, err := h.getMembershipAndWorkspace(ctx, userID, wsID)
	if err != nil {
		return nil, err
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(membership.Role, perm.MinRole(perm.PermWorkspaceDelete, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only primary owner can delete workspace"))
	}

	if err := h.q.SoftDeleteWorkspace(ctx, wsID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to delete workspace"))
	}

	return connect.NewResponse(&workspacepb.DeleteWorkspaceResponse{}), nil
}

func (h *Handler) ListWorkspaces(ctx context.Context, req *connect.Request[workspacepb.ListWorkspacesRequest]) (*connect.Response[workspacepb.ListWorkspacesResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	workspaces, err := h.q.ListWorkspacesForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list workspaces"))
	}

	pbWorkspaces := make([]*workspacepb.Workspace, len(workspaces))
	for i := range workspaces {
		pbWorkspaces[i] = workspaceToProto(&workspaces[i])
	}

	return connect.NewResponse(&workspacepb.ListWorkspacesResponse{
		Workspaces: pbWorkspaces,
	}), nil
}

// ListMembers returns all members of a workspace.
func (h *Handler) ListMembers(ctx context.Context, req *connect.Request[workspacepb.ListMembersRequest]) (*connect.Response[workspacepb.ListMembersResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membership, ws, err := h.getMembershipAndWorkspace(ctx, userID, wsID)
	if err != nil {
		return nil, err
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(membership.Role, perm.MinRole(perm.PermMembersRead, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions"))
	}

	members, err := h.q.ListMemberships(ctx, wsID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list members"))
	}

	pbMembers := make([]*workspacepb.MemberInfo, 0, len(members))
	for i := range members {
		user, err := h.q.GetUserByID(ctx, members[i].UserID)
		if err != nil {
			continue // Skip if user not found (shouldn't happen)
		}
		pbMembers = append(pbMembers, memberInfoToProto(&members[i], &user))
	}

	return connect.NewResponse(&workspacepb.ListMembersResponse{
		Members: pbMembers,
	}), nil
}

// UpdateMemberRole changes a member's role. Primary owner can change anyone's role (except their own to non-primary).
func (h *Handler) UpdateMemberRole(ctx context.Context, req *connect.Request[workspacepb.UpdateMemberRoleRequest]) (*connect.Response[workspacepb.UpdateMemberRoleResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membershipID, err := uuid.Parse(req.Msg.MembershipId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid membership_id"))
	}

	actorMembership, ws, err := h.getMembershipAndWorkspace(ctx, userID, wsID)
	if err != nil {
		return nil, err
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(actorMembership.Role, perm.MinRole(perm.PermMembersUpdate, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions"))
	}

	targetMembership, err := h.q.GetMembershipByID(ctx, membershipID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	if targetMembership.WorkspaceID != wsID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("membership not in workspace"))
	}

	newRole := protoRoleToDBRole(req.Msg.Role)

	// Cannot change primary owner's role (use TransferOwnership instead)
	if targetMembership.Role == perm.LevelPrimaryOwner {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot change primary owner role; use transfer ownership"))
	}

	// Cannot assign primary owner role (use TransferOwnership instead)
	if newRole == perm.LevelPrimaryOwner {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot assign primary owner; use transfer ownership"))
	}

	// Can only manage roles you have authority over
	if !canManageRole(actorMembership.Role, targetMembership.Role) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot manage this member's role"))
	}

	// Can only assign roles lower than or equal to your own (except primary owner)
	if !canAssignRole(actorMembership.Role, newRole) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot assign this role"))
	}

	updated, err := h.q.UpdateMembershipRole(ctx, db.UpdateMembershipRoleParams{
		Role: newRole,
		ID:   membershipID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to update role"))
	}

	return connect.NewResponse(&workspacepb.UpdateMemberRoleResponse{
		Membership: membershipToProto(&updated),
	}), nil
}

// TransferOwnership transfers primary ownership to another member.
func (h *Handler) TransferOwnership(ctx context.Context, req *connect.Request[workspacepb.TransferOwnershipRequest]) (*connect.Response[workspacepb.TransferOwnershipResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	newOwnerUserID, err := uuid.Parse(req.Msg.NewOwnerUserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid new_owner_user_id"))
	}

	if userID == newOwnerUserID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot transfer to yourself"))
	}

	actorMembership, ws, err := h.getMembershipAndWorkspace(ctx, userID, wsID)
	if err != nil {
		return nil, err
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(actorMembership.Role, perm.MinRole(perm.PermWorkspaceTransfer, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only primary owner can transfer ownership"))
	}

	newOwnerMembership, err := h.q.GetMembership(ctx, db.GetMembershipParams{UserID: newOwnerUserID, WorkspaceID: wsID})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("new owner is not a member"))
	}
	if newOwnerMembership.Status != db.MembershipStatusActive {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("new owner membership not active"))
	}

	var oldOwner, newOwner db.WorkspaceMembership
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		qtx := h.q.WithTx(tx)

		var err error
		// Demote current primary owner to owner
		oldOwner, err = qtx.UpdateMembershipRole(ctx, db.UpdateMembershipRoleParams{
			Role: perm.LevelOwner,
			ID:   actorMembership.ID,
		})
		if err != nil {
			return err
		}

		// Promote new owner to primary owner
		newOwner, err = qtx.UpdateMembershipRole(ctx, db.UpdateMembershipRoleParams{
			Role: perm.LevelPrimaryOwner,
			ID:   newOwnerMembership.ID,
		})
		return err
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to transfer ownership"))
	}

	return connect.NewResponse(&workspacepb.TransferOwnershipResponse{
		OldOwnerMembership: membershipToProto(&oldOwner),
		NewOwnerMembership: membershipToProto(&newOwner),
	}), nil
}

// RemoveMember removes a member from the workspace. Cannot remove primary owner.
func (h *Handler) RemoveMember(ctx context.Context, req *connect.Request[workspacepb.RemoveMemberRequest]) (*connect.Response[workspacepb.RemoveMemberResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membershipID, err := uuid.Parse(req.Msg.MembershipId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid membership_id"))
	}

	actorMembership, ws, err := h.getMembershipAndWorkspace(ctx, userID, wsID)
	if err != nil {
		return nil, err
	}

	wsPerms := parseWorkspacePerms(ws.Prefs)
	if !perm.Can(actorMembership.Role, perm.MinRole(perm.PermMembersRemove, wsPerms)) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions"))
	}

	targetMembership, err := h.q.GetMembershipByID(ctx, membershipID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	if targetMembership.WorkspaceID != wsID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("membership not in workspace"))
	}

	// Cannot remove primary owner
	if targetMembership.Role == perm.LevelPrimaryOwner {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot remove primary owner"))
	}

	// Can only remove members you have authority over
	if !canManageRole(actorMembership.Role, targetMembership.Role) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot remove this member"))
	}

	if err := h.q.SoftDeleteMembership(ctx, membershipID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to remove member"))
	}

	return connect.NewResponse(&workspacepb.RemoveMemberResponse{}), nil
}

// LeaveWorkspace allows a member to leave. Primary owner cannot leave.
func (h *Handler) LeaveWorkspace(ctx context.Context, req *connect.Request[workspacepb.LeaveWorkspaceRequest]) (*connect.Response[workspacepb.LeaveWorkspaceResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	wsID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	membership, err := h.q.GetMembership(ctx, db.GetMembershipParams{UserID: userID, WorkspaceID: wsID})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not a member"))
	}

	// Primary owner cannot leave, they must transfer ownership first or delete the workspace
	if membership.Role == perm.LevelPrimaryOwner {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("primary owner cannot leave; transfer ownership or delete workspace"))
	}

	if err := h.q.SoftDeleteMembership(ctx, membership.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to leave workspace"))
	}

	return connect.NewResponse(&workspacepb.LeaveWorkspaceResponse{}), nil
}

// Helper functions

func (h *Handler) getMembershipAndWorkspace(ctx context.Context, userID, wsID uuid.UUID) (db.WorkspaceMembership, db.Workspace, error) {
	membership, err := h.q.GetMembership(ctx, db.GetMembershipParams{UserID: userID, WorkspaceID: wsID})
	if err != nil {
		return db.WorkspaceMembership{}, db.Workspace{}, connect.NewError(connect.CodePermissionDenied, errors.New("not a member"))
	}
	if membership.Status != db.MembershipStatusActive {
		return db.WorkspaceMembership{}, db.Workspace{}, connect.NewError(connect.CodePermissionDenied, errors.New("membership not active"))
	}

	ws, err := h.q.GetWorkspaceByID(ctx, wsID)
	if err != nil {
		return db.WorkspaceMembership{}, db.Workspace{}, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
	}

	return membership, ws, nil
}

func parseWorkspacePerms(prefs []byte) map[string]int16 {
	if len(prefs) == 0 {
		return nil
	}
	var p struct {
		Permissions map[string]int16 `json:"permissions"`
	}
	if err := json.Unmarshal(prefs, &p); err != nil {
		return nil
	}
	return p.Permissions
}

func canManageRole(actorRole, targetRole int16) bool {
	switch actorRole {
	case perm.LevelPrimaryOwner:
		return true
	case perm.LevelOwner:
		return targetRole >= perm.LevelAdmin
	case perm.LevelAdmin:
		return targetRole >= perm.LevelMember
	default:
		return false
	}
}

func canAssignRole(actorRole, newRole int16) bool {
	if actorRole == perm.LevelPrimaryOwner {
		return newRole >= perm.LevelOwner // primary owner can assign any role except primary_owner
	}
	return newRole > actorRole // can only assign roles lower than your own
}

func workspaceToProto(ws *db.Workspace) *workspacepb.Workspace {
	return &workspacepb.Workspace{
		Id:        ws.ID.String(),
		Name:      ws.Name,
		Slug:      ws.Slug,
		IsActive:  ws.IsActive,
		CreatedAt: timestampToUnix(ws.CreatedAt),
		UpdatedAt: timestampToUnix(ws.UpdatedAt),
	}
}

func membershipToProto(m *db.WorkspaceMembership) *workspacepb.Membership {
	return &workspacepb.Membership{
		Id:          m.ID.String(),
		UserId:      m.UserID.String(),
		WorkspaceId: m.WorkspaceID.String(),
		Role:        dbRoleToProtoRole(m.Role),
		Status:      dbStatusToProtoStatus(m.Status),
		CreatedAt:   timestampToUnix(m.CreatedAt),
		UpdatedAt:   timestampToUnix(m.UpdatedAt),
	}
}

func memberInfoToProto(m *db.WorkspaceMembership, u *db.User) *workspacepb.MemberInfo {
	return &workspacepb.MemberInfo{
		MembershipId: m.ID.String(),
		UserId:       u.ID.String(),
		Name:         textVal(u.Name),
		Email:        u.Email,
		AvatarUrl:    textVal(u.AvatarUrl),
		Role:         dbRoleToProtoRole(m.Role),
		Status:       dbStatusToProtoStatus(m.Status),
	}
}

func dbRoleToProtoRole(role int16) workspacepb.Role {
	switch role {
	case perm.LevelPrimaryOwner:
		return workspacepb.Role_ROLE_PRIMARY_OWNER
	case perm.LevelOwner:
		return workspacepb.Role_ROLE_OWNER
	case perm.LevelAdmin:
		return workspacepb.Role_ROLE_ADMIN
	case perm.LevelMember:
		return workspacepb.Role_ROLE_MEMBER
	case perm.LevelGuest:
		return workspacepb.Role_ROLE_GUEST
	default:
		return workspacepb.Role_ROLE_UNSPECIFIED
	}
}

func protoRoleToDBRole(role workspacepb.Role) int16 {
	switch role {
	case workspacepb.Role_ROLE_PRIMARY_OWNER:
		return perm.LevelPrimaryOwner
	case workspacepb.Role_ROLE_OWNER:
		return perm.LevelOwner
	case workspacepb.Role_ROLE_ADMIN:
		return perm.LevelAdmin
	case workspacepb.Role_ROLE_MEMBER:
		return perm.LevelMember
	case workspacepb.Role_ROLE_GUEST:
		return perm.LevelGuest
	default:
		return perm.LevelGuest
	}
}

func dbStatusToProtoStatus(status db.MembershipStatus) workspacepb.MembershipStatus {
	switch status {
	case db.MembershipStatusInvited:
		return workspacepb.MembershipStatus_MEMBERSHIP_STATUS_INVITED
	case db.MembershipStatusActive:
		return workspacepb.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE
	case db.MembershipStatusRejected:
		return workspacepb.MembershipStatus_MEMBERSHIP_STATUS_REJECTED
	case db.MembershipStatusDisabled:
		return workspacepb.MembershipStatus_MEMBERSHIP_STATUS_DISABLED
	case db.MembershipStatusRemoved:
		return workspacepb.MembershipStatus_MEMBERSHIP_STATUS_REMOVED
	default:
		return workspacepb.MembershipStatus_MEMBERSHIP_STATUS_UNSPECIFIED
	}
}

func timestampToUnix(t pgtype.Timestamptz) int64 {
	if t.Valid {
		return t.Time.Unix()
	}
	return 0
}

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func pgtextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
