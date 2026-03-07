package file

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	filepb "baseapp/gen/api/file"
	"baseapp/gen/api/file/filepbconnect"
	"baseapp/gen/db"
	"baseapp/infra/auth"
	"baseapp/infra/file"
)

type Handler struct {
	q  *db.Queries
	s3 *file.S3Service
}

var _ filepbconnect.FileServiceHandler = (*Handler)(nil)

func NewHandler(q *db.Queries, s3 *file.S3Service) *Handler {
	return &Handler{q: q, s3: s3}
}

func (h *Handler) RequestUpload(ctx context.Context, req *connect.Request[filepb.RequestUploadRequest]) (*connect.Response[filepb.RequestUploadResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	workspaceID, err := uuid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workspace_id"))
	}

	// Generate presigned URL
	uploadResp, err := h.s3.GenerateUploadURL(ctx, file.UploadRequest{
		Filename:    req.Msg.Filename,
		ContentType: req.Msg.ContentType,
		Size:        req.Msg.Size,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate upload url"))
	}

	// Store pending file record
	expiresAt := time.Now().Add(h.s3.URLLifetime())
	f, err := h.q.CreateFile(ctx, db.CreateFileParams{
		WorkspaceID:   workspaceID,
		UploadedBy:    userID,
		S3Key:         uploadResp.Key,
		Title:         req.Msg.Filename,
		FileSizeBytes: pgtype.Int8{Int64: req.Msg.Size, Valid: true},
		MimeType:      pgtype.Text{String: req.Msg.ContentType, Valid: true},
		ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create file record"))
	}

	return connect.NewResponse(&filepb.RequestUploadResponse{
		FileId:    f.ID.String(),
		UploadUrl: uploadResp.URL,
	}), nil
}

// ConfirmUpload TODO: for now it will be called from frontend, but should later be called from aws lambda.
func (h *Handler) ConfirmUpload(ctx context.Context, req *connect.Request[filepb.ConfirmUploadRequest]) (*connect.Response[filepb.ConfirmUploadResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	fileID, err := uuid.Parse(req.Msg.FileId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid file_id"))
	}

	// Get file to verify ownership and check S3
	f, err := h.q.GetFile(ctx, fileID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("file not found"))
	}
	if f.UploadedBy != userID {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not your file"))
	}

	// Verify file exists in S3
	if !h.s3.ObjectExists(ctx, f.S3Key) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("file not uploaded"))
	}

	// Mark as completed
	f, err = h.q.ConfirmUpload(ctx, fileID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to confirm upload"))
	}

	return connect.NewResponse(&filepb.ConfirmUploadResponse{
		FileId: f.ID.String(),
	}), nil
}

func (h *Handler) GetDownloadURL(ctx context.Context, req *connect.Request[filepb.GetDownloadURLRequest]) (*connect.Response[filepb.GetDownloadURLResponse], error) {
	_, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	fileID, err := uuid.Parse(req.Msg.FileId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid file_id"))
	}

	f, err := h.q.GetFile(ctx, fileID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("file not found"))
	}
	if f.Status != db.UploadStatusCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("file not ready"))
	}

	downloadResp, err := h.s3.GenerateDownloadURL(ctx, f.S3Key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to generate download url"))
	}

	return connect.NewResponse(&filepb.GetDownloadURLResponse{
		DownloadUrl: downloadResp.URL,
	}), nil
}

// CleanupExpired deletes expired pending uploads from DB and S3. call periodically from worker/cron.
func (h *Handler) CleanupExpired(ctx context.Context) error {
	keys, err := h.q.DeleteExpiredPendingFiles(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		_ = h.s3.Delete(ctx, key) // Best effort
	}
	return nil
}
