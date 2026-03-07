package user

import (
	"baseapp/infra/auth/session"
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	userpb "baseapp/gen/api/user"
	"baseapp/gen/api/user/userpbconnect"
	"baseapp/gen/db"
	"baseapp/infra/auth"
)

type AuthHandler struct {
	q       *db.Queries
	session *session.SessionManager
}

var _ userpbconnect.AuthServiceHandler = (*AuthHandler)(nil)

func NewAuthHandler(q *db.Queries, session *session.SessionManager) *AuthHandler {
	return &AuthHandler{q: q, session: session}
}

func (h *AuthHandler) Signup(ctx context.Context, req *connect.Request[userpb.SignupRequest]) (*connect.Response[userpb.AuthResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email required"))
	}

	u, err := h.q.CreateUser(ctx, db.CreateUserParams{
		Email: req.Msg.Email,
		Name:  pgtype.Text{String: req.Msg.Name, Valid: req.Msg.Name != ""},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("email already registered"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	return h.createSessionResponse(ctx, req.Header().Get("User-Agent"), u.ID)
}

func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[userpb.LoginRequest]) (*connect.Response[userpb.AuthResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email required"))
	}

	u, err := h.q.GetUserByEmail(ctx, req.Msg.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if !u.IsActive {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account disabled"))
	}

	return h.createSessionResponse(ctx, req.Header().Get("User-Agent"), u.ID)
}

func (h *AuthHandler) Logout(ctx context.Context, req *connect.Request[userpb.LogoutRequest]) (*connect.Response[userpb.LogoutResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	sessionID, ok := auth.SessionIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no session"))
	}

	if err := h.session.RevokeSession(ctx, userID, sessionID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	return connect.NewResponse(&userpb.LogoutResponse{}), nil
}

func (h *AuthHandler) GetActiveSessions(ctx context.Context, req *connect.Request[userpb.GetActiveSessionsRequest]) (*connect.Response[userpb.GetActiveSessionsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	currentSessionID, _ := auth.SessionIDFromContext(ctx)

	sessions, err := h.session.GetActiveSessions(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	pbSessions := make([]*userpb.SessionInfo, len(sessions))
	for i, s := range sessions {
		pbSessions[i] = sessionToProto(&s, currentSessionID)
	}

	return connect.NewResponse(&userpb.GetActiveSessionsResponse{Sessions: pbSessions}), nil
}

func (h *AuthHandler) RevokeSession(ctx context.Context, req *connect.Request[userpb.RevokeSessionRequest]) (*connect.Response[userpb.RevokeSessionResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	if req.Msg.SessionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id required"))
	}

	sessionID, err := uuid.Parse(req.Msg.SessionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid session_id"))
	}

	if err := h.session.RevokeSession(ctx, userID, sessionID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	return connect.NewResponse(&userpb.RevokeSessionResponse{}), nil
}

func (h *AuthHandler) RevokeAllSessions(ctx context.Context, req *connect.Request[userpb.RevokeAllSessionsRequest]) (*connect.Response[userpb.RevokeAllSessionsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	if req.Msg.KeepCurrent {
		sessionID, ok := auth.SessionIDFromContext(ctx)
		if !ok {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no session"))
		}
		if err := h.session.RevokeAllSessionsExcept(ctx, userID, sessionID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
	} else {
		if err := h.session.RevokeAllSessions(ctx, userID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
	}

	return connect.NewResponse(&userpb.RevokeAllSessionsResponse{}), nil
}

func (h *AuthHandler) createSessionResponse(ctx context.Context, userAgent string, userID uuid.UUID) (*connect.Response[userpb.AuthResponse], error) {
	ip := auth.ClientIPFromContext(ctx)
	if ip == "" {
		ip = "0.0.0.0"
	}

	token, session, err := h.session.CreateSession(ctx, session.CreateSessionInput{
		UserID:    userID,
		UserAgent: userAgent,
		IPAddress: ip,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	return connect.NewResponse(&userpb.AuthResponse{
		SessionToken: token,
		Session:      sessionToProto(&session, session.ID),
	}), nil
}

func sessionToProto(s *db.Session, currentSessionID uuid.UUID) *userpb.SessionInfo {
	info := &userpb.SessionInfo{
		Id:        s.ID.String(),
		IsCurrent: s.ID == currentSessionID,
		IpAddress: s.IpAddress,
	}
	if s.DeviceType.Valid {
		info.DeviceType = s.DeviceType.String
	}
	if s.DeviceName.Valid {
		info.DeviceName = s.DeviceName.String
	}
	if s.Browser.Valid {
		info.Browser = s.Browser.String
	}
	if s.Os.Valid {
		info.Os = s.Os.String
	}
	if s.CountryCode.Valid {
		info.CountryCode = s.CountryCode.String
	}
	if s.City.Valid {
		info.City = s.City.String
	}
	if s.LastActivityAt.Valid {
		info.LastActivityAt = s.LastActivityAt.Time.Unix()
	}
	if s.CreatedAt.Valid {
		info.CreatedAt = s.CreatedAt.Time.Unix()
	}
	return info
}
