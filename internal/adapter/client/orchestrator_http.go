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
		client:  &http.Client{Timeout: 120 * time.Second},
		log:     log,
	}
}

func (c *OrchestratorHTTP) RunScenarioStep(ctx context.Context, serial, scenarioID, stepID, action string, params map[string]string) error {
	body, _ := json.Marshal(map[string]any{
		"scenario_id": scenarioID,
		"step_id":     stepID,
		"action":      action,
		"params":      params,
	})
	url := fmt.Sprintf("%s/phones/%s/scenarios/run-step", c.baseURL, serial)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("orchestrator run-step HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

var _ port.OrchestratorClient = (*OrchestratorHTTP)(nil)
