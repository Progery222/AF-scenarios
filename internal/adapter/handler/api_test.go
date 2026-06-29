package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobilefarm/af/scenarios/internal/adapter/handler"
	"github.com/mobilefarm/af/scenarios/internal/adapter/repository"
	"github.com/mobilefarm/af/scenarios/internal/port"
	"github.com/mobilefarm/af/scenarios/internal/service"
)

func TestAPI_PutGetScenario(t *testing.T) {
	store := repository.NewMemoryStore()
	svc := service.NewScenarioService(store, port.RealClock{}, nil)
	api := handler.NewAPI(svc, store, noopOrch{}, nil)

	yaml := `id: api-test
name: API
serial: S1
steps:
  - id: a
    at: "10:00"
    action: wait
`
	body, _ := json.Marshal(map[string]string{
		"scenario_yaml":  yaml,
		"variables_yaml": "k: [1,2]\n",
	})
	req := httptest.NewRequest(http.MethodPut, "/scenarios/S1/api-test", bytes.NewReader(body))
	w := httptest.NewRecorder()
	api.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/scenarios/S1/api-test", nil)
	w2 := httptest.NewRecorder()
	api.Routes().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("get: %d", w2.Code)
	}
}

type noopOrch struct{}

func (noopOrch) RunScenarioStep(context.Context, port.RunStepInput) (port.RunStepResult, error) {
	return port.RunStepResult{Status: "completed"}, nil
}
