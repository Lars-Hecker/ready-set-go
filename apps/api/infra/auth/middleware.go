package auth

import (
	"baseapp/infra/auth/session"
	"context"
	"errors"
	"net"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

type userIDKey struct{}
type sessionIDKey struct{}
type clientIPKey struct{}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	uid, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return uid, ok
}

func SessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	sid, ok := ctx.Value(sessionIDKey{}).(uuid.UUID)
	return sid, ok
}

func ClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

func NewAuthInterceptor(sessions *session.SessionManager, publicProcedures map[string]bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ctx = context.WithValue(ctx, clientIPKey{}, extractClientIP(req))

			if publicProcedures[req.Spec().Procedure] {
				return next(ctx, req)
			}

			token := strings.TrimPrefix(req.Header().Get("Authorization"), "Bearer ")
			if token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing auth token"))
			}

			session, err := sessions.ValidateSession(ctx, token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid session"))
			}

			ctx = context.WithValue(ctx, userIDKey{}, session.UserID)
			ctx = context.WithValue(ctx, sessionIDKey{}, session.ID)
			return next(ctx, req)
		}
	}
}

func extractClientIP(req connect.AnyRequest) string {
	if xff := req.Header().Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			xff = xff[:idx]
		}
		if ip := net.ParseIP(strings.TrimSpace(xff)); ip != nil {
			return ip.String()
		}
	}
	if xri := req.Header().Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}
	return ""
}
