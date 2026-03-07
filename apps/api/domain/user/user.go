package user

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	userpb "baseapp/gen/api/user"
	"baseapp/gen/api/user/userpbconnect"
	"baseapp/gen/db"
	"baseapp/infra/auth"
)

type UserHandler struct {
	q *db.Queries
}

var _ userpbconnect.UserServiceHandler = (*UserHandler)(nil)

func NewUserHandler(q *db.Queries) *UserHandler {
	return &UserHandler{q: q}
}

func (h *UserHandler) GetProfile(ctx context.Context, req *connect.Request[userpb.GetProfileRequest]) (*connect.Response[userpb.Profile], error) {
	uid, err := uuid.Parse(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	u, err := h.q.GetUserByID(ctx, uid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}

	return connect.NewResponse(&userpb.Profile{
		UserId:    u.ID.String(),
		Name:      textVal(u.Name),
		Username:  textVal(u.Username),
		AvatarUrl: textVal(u.AvatarUrl),
	}), nil
}

func (h *UserHandler) GetMe(ctx context.Context, req *connect.Request[userpb.GetMeRequest]) (*connect.Response[userpb.GetMeResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	u, err := h.q.GetUserByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}

	return connect.NewResponse(&userpb.GetMeResponse{
		UserId:    u.ID.String(),
		Email:     u.Email,
		Name:      textVal(u.Name),
		Username:  textVal(u.Username),
		AvatarUrl: textVal(u.AvatarUrl),
		IsActive:  u.IsActive,
	}), nil
}

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}
