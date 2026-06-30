package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mobilefarm/af/scenarios/internal/domain"
	"github.com/mobilefarm/af/scenarios/internal/port"
)

type MinIOStore struct {
	objects port.ObjectStorage
	prefix  string
}

func NewMinIOStore(objects port.ObjectStorage, prefix string) *MinIOStore {
	if prefix == "" {
		prefix = "scenarios"
	}
	return &MinIOStore{objects: objects, prefix: strings.Trim(prefix, "/")}
}

func (s *MinIOStore) base(serial, scenarioID string) string {
	return fmt.Sprintf("%s/%s/%s", s.prefix, serial, scenarioID)
}

func (s *MinIOStore) scenarioKey(serial, scenarioID string) string {
	return s.base(serial, scenarioID) + "/scenario.yaml"
}

func (s *MinIOStore) variablesKey(serial, scenarioID string) string {
	return s.base(serial, scenarioID) + "/variables.yaml"
}

func (s *MinIOStore) stateKey(serial, scenarioID string) string {
	return s.base(serial, scenarioID) + "/state.json"
}

func (s *MinIOStore) logKey(serial, scenarioID, date string) string {
	return s.base(serial, scenarioID) + "/logs/" + date + ".jsonl"
}

func (s *MinIOStore) List(ctx context.Context, serial string) ([]domain.ScenarioSummary, error) {
	prefix := s.prefix + "/"
	if serial != "" {
		prefix = s.base(serial, "")
	}
	keys, err := s.objects.ListPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []domain.ScenarioSummary
	for _, key := range keys {
		if !strings.HasSuffix(key, "/scenario.yaml") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(key, s.prefix+"/"), "/")
		if len(parts) < 3 {
			continue
		}
		sid, id := parts[0], parts[1]
		if serial != "" && sid != serial {
			continue
		}
		uniq := sid + "/" + id
		if _, ok := seen[uniq]; ok {
			continue
		}
		seen[uniq] = struct{}{}
		data, err := s.objects.Get(ctx, key)
		if err != nil {
			continue
		}
		sum, err := summaryFromYAML(data, sid, id)
		if err != nil {
			continue
		}
		out = append(out, sum)
	}
	return out, nil
}

func summaryFromYAML(data []byte, serial, id string) (domain.ScenarioSummary, error) {
	var meta struct {
		ID         string `yaml:"id"`
		Name       string `yaml:"name"`
		Serial     string `yaml:"serial"`
		ValidFrom  string `yaml:"valid_from"`
		ValidUntil string `yaml:"valid_until"`
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return domain.ScenarioSummary{}, err
	}
	if meta.ID == "" {
		meta.ID = id
	}
	if meta.Serial == "" {
		meta.Serial = serial
	}
	sum := domain.ScenarioSummary{
		ID: id, Name: meta.Name, Serial: meta.Serial,
		ValidFrom: meta.ValidFrom, ValidUntil: meta.ValidUntil,
	}
	if meta.ID != "" && meta.ID != id {
		sum.YamlID = meta.ID
	}
	return sum, nil
}

func (s *MinIOStore) GetFiles(ctx context.Context, serial, scenarioID string) (scenarioYAML, variablesYAML []byte, err error) {
	scenarioYAML, err = s.objects.Get(ctx, s.scenarioKey(serial, scenarioID))
	if err != nil {
		return nil, nil, domain.ErrNotFound
	}
	variablesYAML, err = s.objects.Get(ctx, s.variablesKey(serial, scenarioID))
	if err != nil {
		variablesYAML = []byte("# variables.yaml\n")
	}
	return scenarioYAML, variablesYAML, nil
}

func (s *MinIOStore) Put(ctx context.Context, serial, scenarioID string, scenarioYAML, variablesYAML []byte) error {
	if serial == "" || scenarioID == "" {
		return domain.ErrMissingID
	}
	if err := validateScenarioYAML(scenarioYAML); err != nil {
		return err
	}
	if err := s.objects.Put(ctx, s.scenarioKey(serial, scenarioID), scenarioYAML, "application/x-yaml"); err != nil {
		return err
	}
	if len(variablesYAML) == 0 {
		variablesYAML = []byte("# variables\n")
	}
	return s.objects.Put(ctx, s.variablesKey(serial, scenarioID), variablesYAML, "application/x-yaml")
}

func validateScenarioYAML(data []byte) error {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidYAML, err)
	}
	if doc["id"] == nil && doc["name"] == nil {
		return fmt.Errorf("%w: нужны id или name", domain.ErrInvalidYAML)
	}
	return nil
}

func (s *MinIOStore) activeKey(serial string) string {
	return fmt.Sprintf("%s/%s/_active.json", s.prefix, serial)
}

func (s *MinIOStore) GetActiveScenarioID(ctx context.Context, serial string) (string, error) {
	data, err := s.objects.Get(ctx, s.activeKey(serial))
	if err != nil {
		return "", nil
	}
	var meta domain.PhoneScenarioMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", nil
	}
	return meta.ActiveScenarioID, nil
}

func (s *MinIOStore) SetActiveScenarioID(ctx context.Context, serial, scenarioID string) error {
	if serial == "" || scenarioID == "" {
		return domain.ErrMissingID
	}
	if _, _, err := s.GetFiles(ctx, serial, scenarioID); err != nil {
		return err
	}
	meta := domain.PhoneScenarioMeta{ActiveScenarioID: scenarioID}
	data, _ := json.Marshal(meta)
	return s.objects.Put(ctx, s.activeKey(serial), data, "application/json")
}

func (s *MinIOStore) Delete(ctx context.Context, serial, scenarioID string) error {
	prefix := s.base(serial, scenarioID) + "/"
	keys, err := s.objects.ListPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return domain.ErrNotFound
	}
	for _, key := range keys {
		if err := s.objects.Delete(ctx, key); err != nil {
			return err
		}
	}
	active, _ := s.GetActiveScenarioID(ctx, serial)
	if active == scenarioID {
		_ = s.objects.Delete(ctx, s.activeKey(serial))
	}
	return nil
}

func (s *MinIOStore) GetState(ctx context.Context, serial, scenarioID string) (domain.DayState, error) {
	data, err := s.objects.Get(ctx, s.stateKey(serial, scenarioID))
	if err != nil {
		return domain.DayState{}, nil
	}
	var st domain.DayState
	if err := json.Unmarshal(data, &st); err != nil {
		return domain.DayState{}, nil
	}
	return st, nil
}

func (s *MinIOStore) PutState(ctx context.Context, serial, scenarioID string, state domain.DayState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.objects.Put(ctx, s.stateKey(serial, scenarioID), data, "application/json")
}

func (s *MinIOStore) AppendLog(ctx context.Context, serial, scenarioID, date string, line []byte) error {
	key := s.logKey(serial, scenarioID, date)
	existing, err := s.objects.Get(ctx, key)
	if err != nil {
		existing = nil
	}
	payload := append(existing, line...)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		payload = append(existing, '\n')
		payload = append(payload, line...)
	}
	return s.objects.Put(ctx, key, payload, "text/plain")
}

func (s *MinIOStore) GetLogs(ctx context.Context, serial, scenarioID, date string) ([]byte, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	data, err := s.objects.Get(ctx, s.logKey(serial, scenarioID, date))
	if err != nil {
		return []byte{}, nil
	}
	return data, nil
}

func (s *MinIOStore) ListAllScenarioPaths(ctx context.Context) ([]domain.ScenarioRef, error) {
	keys, err := s.objects.ListPrefix(ctx, s.prefix+"/")
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var refs []domain.ScenarioRef
	for _, key := range keys {
		if !strings.HasSuffix(key, "/scenario.yaml") {
			continue
		}
		rel := strings.TrimPrefix(key, s.prefix+"/")
		parts := strings.Split(rel, "/")
		if len(parts) < 3 {
			continue
		}
		serial, id := parts[0], parts[1]
		uniq := serial + "/" + id
		if _, ok := seen[uniq]; ok {
			continue
		}
		seen[uniq] = struct{}{}
		refs = append(refs, domain.ScenarioRef{Serial: serial, ScenarioID: id})
	}
	return refs, nil
}

func (s *MinIOStore) ParseScenario(ctx context.Context, serial, scenarioID string) (domain.ScenarioDoc, error) {
	data, err := s.objects.Get(ctx, s.scenarioKey(serial, scenarioID))
	if err != nil {
		return domain.ScenarioDoc{}, domain.ErrNotFound
	}
	var doc domain.ScenarioDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return domain.ScenarioDoc{}, fmt.Errorf("%w: %v", domain.ErrInvalidYAML, err)
	}
	if doc.ID == "" {
		doc.ID = scenarioID
	}
	if doc.Serial == "" {
		doc.Serial = serial
	}
	return doc, nil
}

var _ port.ScenarioRepository = (*MinIOStore)(nil)
