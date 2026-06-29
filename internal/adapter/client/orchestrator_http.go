package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/config"
	"github.com/mobilefarm/af/scenarios/internal/port"
)

type OrchestratorHTTP struct {
	baseURL string
	client  *http.Client
	log     port.Logger
}

func NewOrchestratorHTTP(cfg config.Config, log port.Logger) *OrchestratorHTTP {
	base := strings.TrimRight(cfg.OrchestratorHTTPAddr, "/")
	if base == "" {
		base = "http://127.0.0.1:9092"
	}
	return &OrchestratorHTTP{
		baseURL: base,
		client:  &http.Client{Timeout: 600 * time.Second},
		log:     log,
	}
}

func (c *OrchestratorHTTP) RunScenarioStep(ctx context.Context, in port.RunStepInput) (port.RunStepResult, error) {
	body, _ := json.Marshal(map[string]any{
		"scenario_id":      in.ScenarioID,
		"step_id":            in.StepID,
		"action":             in.Action,
		"params":             in.Params,
		"uses":               in.Uses,
		"variables_yaml":     in.VariablesYAML,
		"scenario_yaml":      in.ScenarioYAML,
		"screenshot_keys":    in.ScreenshotKeys,
		"video_output_key":   in.VideoOutputKey,
	})
	url := fmt.Sprintf("%s/phones/%s/scenarios/run-step", c.baseURL, in.Serial)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return port.RunStepResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return port.RunStepResult{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return port.RunStepResult{}, fmt.Errorf("orchestrator run-step HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Status         string   `json:"status"`
		Message        string   `json:"message"`
		ScreenshotKeys []string `json:"screenshot_keys"`
		VideoJobID     string   `json:"video_job_id"`
		VideoOutputKey string   `json:"video_output_key"`
		Error          string   `json:"error"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return port.RunStepResult{}, err
	}
	if out.Error != "" && out.Status == "failed" {
		return port.RunStepResult{}, fmt.Errorf("%s", out.Error)
	}
	return port.RunStepResult{
		Status:         out.Status,
		Message:        out.Message,
		ScreenshotKeys: out.ScreenshotKeys,
		VideoJobID:     out.VideoJobID,
		VideoOutputKey: out.VideoOutputKey,
	}, nil
}

var _ port.OrchestratorClient = (*OrchestratorHTTP)(nil)
