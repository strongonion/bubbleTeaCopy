package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bubblecopy/internal/model"
	"bubblecopy/internal/session"
)

func TestHandlerServesIndex(t *testing.T) {
	app := New(session.New(nil, 1), nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "Bubblecopy") {
		t.Fatalf("index body = %q, want Bubblecopy", recorder.Body.String())
	}
}

func TestHandlerSelectionDryRunResetFlow(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	dst := filepath.Join(root, "dst.txt")
	mustWriteWebFile(t, src, "hello")

	app := New(session.New([]model.Task{
		{Index: 0, Source: src, Target: dst, Op: model.OpCopy, Group: "docs"},
	}, 2), nil, nil)
	handler := app.Handler()

	state := fetchState(t, handler)
	if state.Phase != session.PhaseSelect {
		t.Fatalf("initial phase = %s, want %s", state.Phase, session.PhaseSelect)
	}

	state = postSelection(t, handler, []int{0})
	if state.SelectedCount != 1 {
		t.Fatalf("SelectedCount = %d, want 1", state.SelectedCount)
	}

	state = postAction(t, handler, "/api/dry-run")
	if state.Phase != session.PhaseDryRun {
		t.Fatalf("phase after dry-run = %s, want %s", state.Phase, session.PhaseDryRun)
	}
	if state.Tasks[0].Status != model.StatusPlanned {
		t.Fatalf("task status after dry-run = %s, want %s", state.Tasks[0].Status, model.StatusPlanned)
	}

	state = postAction(t, handler, "/api/reset")
	if state.Phase != session.PhaseSelect {
		t.Fatalf("phase after reset = %s, want %s", state.Phase, session.PhaseSelect)
	}
	if state.Tasks[0].Status != model.StatusPending {
		t.Fatalf("task status after reset = %s, want %s", state.Tasks[0].Status, model.StatusPending)
	}
	if !state.Tasks[0].Selected {
		t.Fatalf("selection should remain after reset")
	}
}

func TestHandlerStateUsesCamelCaseGroupFields(t *testing.T) {
	app := New(session.New([]model.Task{
		{Index: 0, Source: "a", Target: "b", Op: model.OpCopy, Group: "docs"},
	}, 1), nil, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/state status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "\"name\":\"docs\"") {
		t.Fatalf("GET /api/state body = %q, want group name field", body)
	}
	if !strings.Contains(body, "\"taskIndexes\":[0]") {
		t.Fatalf("GET /api/state body = %q, want camel-case taskIndexes field", body)
	}
	if !strings.Contains(body, "\"selectedCount\":0") {
		t.Fatalf("GET /api/state body = %q, want camel-case selectedCount field", body)
	}
}

func TestHandlerDryRunShowsDuplicateDestinationConflict(t *testing.T) {
	root := t.TempDir()
	srcA := filepath.Join(root, "a.txt")
	srcB := filepath.Join(root, "b.txt")
	target := filepath.Join(root, "same.txt")
	mustWriteWebFile(t, srcA, "a")
	mustWriteWebFile(t, srcB, "b")

	app := New(session.New([]model.Task{
		{Index: 0, Source: srcA, Target: target, Op: model.OpCopy, Group: "docs"},
		{Index: 1, Source: srcB, Target: target, Op: model.OpCopy, Group: "docs"},
	}, 2), nil, nil)
	handler := app.Handler()

	_ = postSelection(t, handler, []int{0, 1})
	state := postAction(t, handler, "/api/dry-run")

	for _, idx := range []int{0, 1} {
		task := state.Tasks[idx]
		if task.Status != model.StatusSkipped {
			t.Fatalf("task %d status = %s, want %s", idx, task.Status, model.StatusSkipped)
		}
		if !strings.Contains(task.Message, "duplicate final destination") {
			t.Fatalf("task %d message = %q, want duplicate destination", idx, task.Message)
		}
	}
}

func fetchState(t *testing.T, handler http.Handler) session.Snapshot {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/state status = %d, want %d", recorder.Code, http.StatusOK)
	}

	return decodeSnapshot(t, recorder.Body.Bytes())
}

func postSelection(t *testing.T, handler http.Handler, selected []int) session.Snapshot {
	t.Helper()

	body, err := json.Marshal(map[string][]int{"selected": selected})
	if err != nil {
		t.Fatalf("Marshal selection: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/selection", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /api/selection status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	return decodeSnapshot(t, recorder.Body.Bytes())
}

func postAction(t *testing.T, handler http.Handler, path string) session.Snapshot {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST %s status = %d, want %d, body=%s", path, recorder.Code, http.StatusOK, recorder.Body.String())
	}

	return decodeSnapshot(t, recorder.Body.Bytes())
}

func decodeSnapshot(t *testing.T, raw []byte) session.Snapshot {
	t.Helper()

	var snapshot session.Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("Unmarshal snapshot: %v", err)
	}
	return snapshot
}

func mustWriteWebFile(t *testing.T, path string, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
