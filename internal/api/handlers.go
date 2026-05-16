package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/hashir-zahoor-kh/terrasense/internal/correction"
	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type infraStoreI interface {
	ListPending(ctx context.Context) ([]models.InfraRequest, error)
	GetByID(ctx context.Context, id pgtype.UUID) (models.InfraRequest, error)
	UpdateStatus(ctx context.Context, id pgtype.UUID, status models.RequestStatus) error
}

type auditStoreI interface {
	Create(ctx context.Context, requestID pgtype.UUID, action, actor string, details map[string]interface{}) error
}

type correctionLoopI interface {
	Run(ctx context.Context, input correction.LoopInput) (correction.LoopResult, error)
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db             *pgxpool.Pool
	infraStore     infraStoreI
	auditStore     auditStoreI
	correctionLoop correctionLoopI
	workingDir     string
}

// NewHandler constructs a Handler with all required dependencies.
func NewHandler(
	db *pgxpool.Pool,
	infraStore infraStoreI,
	auditStore auditStoreI,
	correctionLoop correctionLoopI,
	workingDir string,
) *Handler {
	return &Handler{
		db:             db,
		infraStore:     infraStore,
		auditStore:     auditStore,
		correctionLoop: correctionLoop,
		workingDir:     workingDir,
	}
}

// uuidToString formats a pgtype.UUID as a standard hyphenated UUID string.
func uuidToString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type createChangeRequest struct {
	NaturalLanguageRequest string `json:"natural_language_request"`
	WorkspaceID            string `json:"workspace_id"`
}

type createChangeResponse struct {
	RequestID pgtype.UUID `json:"request_id"`
	Status    string      `json:"status"`
	Message   string      `json:"message"`
}

// CreateChange accepts a natural-language infrastructure request and starts the
// HCL generation → validation → approval workflow.
func (h *Handler) CreateChange(w http.ResponseWriter, r *http.Request) {
	var req createChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.NaturalLanguageRequest == "" || req.WorkspaceID == "" {
		http.Error(w, "natural_language_request and workspace_id are required", http.StatusBadRequest)
		return
	}

	result, err := h.correctionLoop.Run(r.Context(), correction.LoopInput{
		NaturalLanguageRequest: req.NaturalLanguageRequest,
		WorkspaceID:            req.WorkspaceID,
	})
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, createChangeResponse{
		RequestID: result.RequestID,
		Status:    "pending",
		Message:   "change request accepted and queued for approval",
	})
}

// ListChanges returns all pending change requests.
func (h *Handler) ListChanges(w http.ResponseWriter, r *http.Request) {
	changes, err := h.infraStore.ListPending(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, changes)
}

// GetChange returns a single change request by ID.
func (h *Handler) GetChange(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var uid pgtype.UUID
	if err := uid.Scan(idStr); err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	change, err := h.infraStore.GetByID(r.Context(), uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, change)
}

// ApproveChange marks a validated change as approved, runs terraform apply,
// then marks it applied. Only changes in pending status may be approved.
func (h *Handler) ApproveChange(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var uid pgtype.UUID
	if err := uid.Scan(idStr); err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	change, err := h.infraStore.GetByID(r.Context(), uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if change.Status != models.StatusPending {
		http.Error(w, "change is not in pending status", http.StatusConflict)
		return
	}

	if err := h.infraStore.UpdateStatus(r.Context(), uid, models.StatusApproved); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.auditStore.Create(r.Context(), uid, "approved", "system", nil); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	workspaceDir := filepath.Join(h.workingDir, uuidToString(uid))
	cmd := exec.CommandContext(r.Context(), "terraform", "apply", "-auto-approve", "-input=false", "-no-color")
	cmd.Dir = workspaceDir
	if _, err := cmd.Output(); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.infraStore.UpdateStatus(r.Context(), uid, models.StatusApplied); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.auditStore.Create(r.Context(), uid, "applied", "system", nil); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

type rejectChangeRequest struct {
	Reason string `json:"reason"`
}

// RejectChange marks a change as rejected, recording the reason.
func (h *Handler) RejectChange(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var uid pgtype.UUID
	if err := uid.Scan(idStr); err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req rejectChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	change, err := h.infraStore.GetByID(r.Context(), uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if change.Status != models.StatusPending {
		http.Error(w, "change is not in pending status", http.StatusConflict)
		return
	}

	if err := h.infraStore.UpdateStatus(r.Context(), uid, models.StatusRejected); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.auditStore.Create(r.Context(), uid, "rejected", "system", map[string]interface{}{
		"reason": req.Reason,
	}); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
