package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/correction"
	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- fakes ---

type fakeInfraStore struct {
	pending []models.InfraRequest
	record  models.InfraRequest
	getErr  error
}

func (f *fakeInfraStore) ListPending(_ context.Context) ([]models.InfraRequest, error) {
	return f.pending, nil
}

func (f *fakeInfraStore) GetByID(_ context.Context, _ pgtype.UUID) (models.InfraRequest, error) {
	return f.record, f.getErr
}

func (f *fakeInfraStore) UpdateStatus(_ context.Context, _ pgtype.UUID, _ models.RequestStatus) error {
	return nil
}

type fakeAuditStore struct{}

func (f *fakeAuditStore) Create(_ context.Context, _ pgtype.UUID, _, _ string, _ map[string]interface{}) error {
	return nil
}

type fakeCorrectionLoop struct {
	result correction.LoopResult
	err    error
}

func (f *fakeCorrectionLoop) Run(_ context.Context, _ correction.LoopInput) (correction.LoopResult, error) {
	return f.result, f.err
}

func newTestHandler(is infraStoreI, as auditStoreI, cl correctionLoopI) *Handler {
	return &Handler{
		infraStore:     is,
		auditStore:     as,
		correctionLoop: cl,
		workingDir:     "/tmp",
	}
}

// --- tests ---

func TestListChanges_EmptyArray(t *testing.T) {
	h := newTestHandler(&fakeInfraStore{pending: []models.InfraRequest{}}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/changes", nil)
	w := httptest.NewRecorder()
	h.ListChanges(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got []models.InfraRequest
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %d items", len(got))
	}
}

func TestGetChange_NotFound(t *testing.T) {
	h := newTestHandler(&fakeInfraStore{getErr: pgx.ErrNoRows}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000001")
	w := httptest.NewRecorder()
	h.GetChange(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetChange_Found(t *testing.T) {
	record := models.InfraRequest{Status: models.StatusPending}
	h := newTestHandler(&fakeInfraStore{record: record}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000001")
	w := httptest.NewRecorder()
	h.GetChange(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateChange_MissingFields(t *testing.T) {
	h := newTestHandler(&fakeInfraStore{}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	body := `{"natural_language_request": "create a bucket"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateChange(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateChange_Valid(t *testing.T) {
	uid := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	cl := &fakeCorrectionLoop{result: correction.LoopResult{Success: true, RequestID: uid}}
	h := newTestHandler(&fakeInfraStore{}, &fakeAuditStore{}, cl)
	body, _ := json.Marshal(map[string]string{
		"natural_language_request": "create an S3 bucket",
		"workspace_id":             "ws-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/changes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateChange(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
}

func TestRejectChange_NotPending(t *testing.T) {
	record := models.InfraRequest{Status: models.StatusApproved}
	h := newTestHandler(&fakeInfraStore{record: record}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	body := `{"reason": "bad idea"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000001")
	w := httptest.NewRecorder()
	h.RejectChange(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestRejectChange_Valid(t *testing.T) {
	record := models.InfraRequest{Status: models.StatusPending}
	h := newTestHandler(&fakeInfraStore{record: record}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	body := `{"reason": "not needed"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000001")
	w := httptest.NewRecorder()
	h.RejectChange(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestApproveChange_NotPending(t *testing.T) {
	record := models.InfraRequest{Status: models.StatusApproved}
	h := newTestHandler(&fakeInfraStore{record: record}, &fakeAuditStore{}, &fakeCorrectionLoop{})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000001")
	w := httptest.NewRecorder()
	h.ApproveChange(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
