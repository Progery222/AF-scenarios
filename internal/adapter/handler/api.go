package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mobilefarm/af/scenarios/internal/domain"
	"github.com/mobilefarm/af/scenarios/internal/port"
	"github.com/mobilefarm/af/scenarios/internal/service"
)

type API struct {
	svc    *service.ScenarioService
	store  port.ScenarioRepository
	orch   port.OrchestratorClient
	sched  *service.Scheduler
}

func NewAPI(svc *service.ScenarioService, store port.ScenarioRepository, orch port.OrchestratorClient, sched *service.Scheduler) *API {
	return &API{svc: svc, store: store, orch: orch, sched: sched}
}

func (h *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/scenarios/generate", h.generate)
	mux.HandleFunc("/scenarios/validate", h.validate)
	mux.HandleFunc("/scenarios/", h.scenarios)
	return mux
}

func (h *API) generate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "только POST", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Serial string `json:"serial"`
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите prompt"})
		return
	}
	files, warnings, issues, err := h.svc.GeneratePreview(r.Context(), body.Prompt, body.Serial)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	vr := h.svc.Validate(r.Context(), files.ScenarioYAML, files.VariablesYAML, body.Serial, true)
	scenarioYAML := files.ScenarioYAML
	if vr.NormalizedScenarioYAML != "" {
		scenarioYAML = vr.NormalizedScenarioYAML
	}
	allWarnings := append(warnings, vr.Warnings...)
	stepIssues := issues
	if len(vr.StepIssues) > 0 {
		stepIssues = vr.StepIssues
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scenario_yaml":            scenarioYAML,
		"variables_yaml":           files.VariablesYAML,
		"normalized_scenario_yaml": scenarioYAML,
		"warnings":                 allWarnings,
		"step_issues":              stepIssues,
		"valid":                    vr.Valid,
		"errors":                   vr.Errors,
		"steps_count":              vr.StepsCount,
		"runnable_by_scheduler":    vr.RunnableByScheduler,
	})
}

func (h *API) validate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "только POST", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Serial         string `json:"serial"`
		ScenarioYAML   string `json:"scenario_yaml"`
		VariablesYAML  string `json:"variables_yaml"`
		Normalize      bool   `json:"normalize"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioYAML == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите scenario_yaml"})
		return
	}
	result := h.svc.Validate(r.Context(), body.ScenarioYAML, body.VariablesYAML, body.Serial, body.Normalize)
	writeJSON(w, http.StatusOK, result)
}

func (h *API) scenarios(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/scenarios/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	serial := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		items, err := h.svc.List(r.Context(), serial)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	if len(parts) == 2 && parts[1] == "active" {
		switch r.Method {
		case http.MethodGet:
			id, err := h.svc.GetActive(r.Context(), serial)
			if err != nil {
				writeSvcErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"serial": serial, "active_scenario_id": id})
		case http.MethodPut:
			var body struct {
				ScenarioID string `json:"scenario_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScenarioID == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "укажите scenario_id"})
				return
			}
			if err := h.svc.SetActive(r.Context(), serial, body.ScenarioID); err != nil {
				writeSvcErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "активный сценарий установлен", "active_scenario_id": body.ScenarioID})
		default:
			http.Error(w, "GET/PUT /scenarios/{serial}/active", http.StatusMethodNotAllowed)
		}
		return
	}
	scenarioID := parts[1]
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			files, err := h.svc.Get(r.Context(), serial, scenarioID)
			if err != nil {
				writeSvcErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, files)
		case http.MethodPut:
			var files service.ScenarioFiles
			if err := json.NewDecoder(r.Body).Decode(&files); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный JSON"})
				return
			}
			if err := h.svc.Put(r.Context(), serial, scenarioID, files); err != nil {
				writeSvcErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "сценарий сохранён"})
		case http.MethodDelete:
			if err := h.svc.Delete(r.Context(), serial, scenarioID); err != nil {
				writeSvcErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "сценарий удалён"})
		default:
			http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		}
		return
	}
	switch parts[2] {
	case "status":
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		st, err := h.svc.Status(r.Context(), serial, scenarioID)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	case "logs":
		if r.Method != http.MethodGet {
			http.Error(w, "только GET", http.StatusMethodNotAllowed)
			return
		}
		date := r.URL.Query().Get("date")
		logs, err := h.svc.GetLogs(r.Context(), serial, scenarioID, date)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
	case "trigger":
		if r.Method != http.MethodPost || len(parts) < 4 {
			http.Error(w, "POST /scenarios/{serial}/{id}/trigger/{step_id}", http.StatusMethodNotAllowed)
			return
		}
		stepID := parts[3]
		doc, err := h.store.ParseScenario(r.Context(), serial, scenarioID)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		var step domain.StepDoc
		for _, st := range doc.Steps {
			if st.ID == stepID {
				step = st
				break
			}
		}
		if step.ID == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "шаг не найден"})
			return
		}
		params := step.Params
		if params == nil {
			params = map[string]string{}
		}
		files, _ := h.svc.Get(r.Context(), serial, scenarioID)
		state, _ := h.store.GetState(r.Context(), serial, scenarioID)
		result, err := h.orch.RunScenarioStep(r.Context(), port.RunStepInput{
			Serial: serial, ScenarioID: scenarioID, StepID: step.ID, Action: step.Action,
			Params: params, Uses: step.Uses,
			VariablesYAML: files.VariablesYAML, ScenarioYAML: files.ScenarioYAML,
			ScreenshotKeys: state.ScreenshotKeys, VideoOutputKey: state.VideoOutputKey,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		_ = h.svc.ApplyStepResult(r.Context(), serial, scenarioID, step.ID, result)
		writeJSON(w, http.StatusOK, map[string]any{"message": "шаг запущен", "step_id": stepID, "result": result})
	default:
		http.NotFound(w, r)
	}
}

func writeSvcErr(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, domain.ErrInvalidYAML) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

type Health struct {
	store port.ObjectStorage
}

func NewHealth(store port.ObjectStorage) *Health {
	return &Health{store: store}
}

func (h *Health) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/ready", h.ready)
	return mux
}

func (h *Health) ready(w http.ResponseWriter, r *http.Request) {
	if h.store != nil {
		if err := h.store.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "minio": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

var _ = context.Background
