package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"baseapp/gen/db"
)

const SessionTTL = 30 * 24 * time.Hour

type SessionManager struct {
	q   *db.Queries
	geo GeoLookup
}

func NewSessionManager(q *db.Queries, geo GeoLookup) *SessionManager {
	return &SessionManager{q: q, geo: geo}
}

type CreateSessionInput struct {
	UserID    uuid.UUID
	UserAgent string
	IPAddress string
}

func (m *SessionManager) CreateSession(ctx context.Context, input CreateSessionInput) (token string, session db.Session, err error) {
	token, err = generateToken()
	if err != nil {
		return "", db.Session{}, err
	}

	device := ParseUserAgent(input.UserAgent)
	geo := m.geo.Lookup(input.IPAddress)

	session, err = m.q.CreateSession(ctx, db.CreateSessionParams{
		UserID:           input.UserID,
		RefreshTokenHash: hashToken(token),
		DeviceType:       pgtext(device.DeviceType),
		DeviceName:       pgtext(device.DeviceName),
		Browser:          pgtext(device.Browser),
		Os:               pgtext(device.OS),
		IpAddress:        input.IPAddress,
		CountryCode:      pgtext(geo.CountryCode),
		City:             pgtext(geo.City),
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(SessionTTL), Valid: true},
	})
	return token, session, err
}

func (m *SessionManager) ValidateSession(ctx context.Context, token string) (db.Session, error) {
	return m.q.GetSessionByTokenHash(ctx, hashToken(token))
}

func (m *SessionManager) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]db.Session, error) {
	return m.q.GetActiveSessionsByUserID(ctx, userID)
}

func (m *SessionManager) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return m.q.RevokeSession(ctx, db.RevokeSessionParams{ID: sessionID, UserID: userID})
}

func (m *SessionManager) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	return m.q.RevokeAllUserSessions(ctx, userID)
}

func (m *SessionManager) RevokeAllSessionsExcept(ctx context.Context, userID, keepSessionID uuid.UUID) error {
	return m.q.RevokeAllUserSessionsExcept(ctx, db.RevokeAllUserSessionsExceptParams{
		UserID: userID,
		ID:     keepSessionID,
	})
}

// TouchSession on authenticated requests to track activity (lightweight, single column update)
func (m *SessionManager) TouchSession(ctx context.Context, sessionID uuid.UUID) error {
	return m.q.TouchSession(ctx, sessionID)
}

// RotateSession after sensitive actions (password change, email change, privilege escalation). Issues a new token, invalidates the old one, extends expiry.
func (m *SessionManager) RotateSession(ctx context.Context, sessionID uuid.UUID) (newToken string, session db.Session, err error) {
	newToken, err = generateToken()
	if err != nil {
		return "", db.Session{}, err
	}

	session, err = m.q.RotateRefreshToken(ctx, db.RotateRefreshTokenParams{
		ID:               sessionID,
		RefreshTokenHash: hashToken(newToken),
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(SessionTTL), Valid: true},
	})
	if err != nil {
		return "", db.Session{}, err
	}
	return newToken, session, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func pgtext(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
