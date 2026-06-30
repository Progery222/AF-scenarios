package repository

import (
	"context"
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/mobilefarm/af/scenarios/internal/domain"
)

type MemoryStore struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{files: map[string][]byte{}}
}

func key(serial, id, file string) string {
	return serial + "/" + id + "/" + file
}

func (m *MemoryStore) List(_ context.Context, serial string) ([]domain.ScenarioSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]struct{}{}
	var out []domain.ScenarioSummary
	for k, data := range m.files {
		if !endsWith(k, "/scenario.yaml") {
			continue
		}
		parts := splitKey(k)
		if len(parts) != 3 {
			continue
		}
		if serial != "" && parts[0] != serial {
			continue
		}
		uniq := parts[0] + "/" + parts[1]
		if _, ok := seen[uniq]; ok {
			continue
		}
		seen[uniq] = struct{}{}
		sum, err := summaryFromYAML(data, parts[0], parts[1])
		if err != nil {
			continue
		}
		out = append(out, sum)
	}
	return out, nil
}

func (m *MemoryStore) GetFiles(_ context.Context, serial, scenarioID string) ([]byte, []byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sc, ok := m.files[key(serial, scenarioID, "scenario.yaml")]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	vars := m.files[key(serial, scenarioID, "variables.yaml")]
	if vars == nil {
		vars = []byte("# variables\n")
	}
	return sc, vars, nil
}

func (m *MemoryStore) Put(_ context.Context, serial, scenarioID string, scenarioYAML, variablesYAML []byte) error {
	if err := validateScenarioYAML(scenarioYAML); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key(serial, scenarioID, "scenario.yaml")] = append([]byte(nil), scenarioYAML...)
	m.files[key(serial, scenarioID, "variables.yaml")] = append([]byte(nil), variablesYAML...)
	return nil
}

func (m *MemoryStore) activeKey(serial string) string {
	return serial + "/_active.json"
}

func (m *MemoryStore) GetActiveScenarioID(_ context.Context, serial string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[m.activeKey(serial)]
	if !ok {
		return "", nil
	}
	var meta domain.PhoneScenarioMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", nil
	}
	return meta.ActiveScenarioID, nil
}

func (m *MemoryStore) SetActiveScenarioID(ctx context.Context, serial, scenarioID string) error {
	if _, _, err := m.GetFiles(ctx, serial, scenarioID); err != nil {
		return err
	}
	data, _ := json.Marshal(domain.PhoneScenarioMeta{ActiveScenarioID: scenarioID})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[m.activeKey(serial)] = data
	return nil
}

func (m *MemoryStore) Delete(ctx context.Context, serial, scenarioID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := serial + "/" + scenarioID + "/"
	found := false
	for k := range m.files {
		if hasPrefix(k, prefix) {
			delete(m.files, k)
			found = true
		}
	}
	if !found {
		return domain.ErrNotFound
	}
	if data, ok := m.files[m.activeKey(serial)]; ok {
		var meta domain.PhoneScenarioMeta
		if json.Unmarshal(data, &meta) == nil && meta.ActiveScenarioID == scenarioID {
			delete(m.files, m.activeKey(serial))
		}
	}
	return nil
}

func (m *MemoryStore) GetState(_ context.Context, serial, scenarioID string) (domain.DayState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[key(serial, scenarioID, "state.json")]
	if !ok {
		return domain.DayState{}, nil
	}
	var st domain.DayState
	_ = json.Unmarshal(data, &st)
	return st, nil
}

func (m *MemoryStore) PutState(_ context.Context, serial, scenarioID string, state domain.DayState) error {
	data, _ := json.Marshal(state)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key(serial, scenarioID, "state.json")] = data
	return nil
}

func (m *MemoryStore) AppendLog(_ context.Context, serial, scenarioID, date string, line []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(serial, scenarioID, "logs/"+date+".jsonl")
	m.files[k] = append(m.files[k], line...)
	if len(m.files[k]) > 0 && m.files[k][len(m.files[k])-1] != '\n' {
		m.files[k] = append(m.files[k], '\n')
	}
	return nil
}

func (m *MemoryStore) GetLogs(_ context.Context, serial, scenarioID, date string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.files[key(serial, scenarioID, "logs/"+date+".jsonl")], nil
}

func (m *MemoryStore) ListAllScenarioPaths(_ context.Context) ([]domain.ScenarioRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]struct{}{}
	var refs []domain.ScenarioRef
	for k := range m.files {
		if !endsWith(k, "/scenario.yaml") {
			continue
		}
		parts := splitKey(k)
		if len(parts) != 3 {
			continue
		}
		uniq := parts[0] + "/" + parts[1]
		if _, ok := seen[uniq]; ok {
			continue
		}
		seen[uniq] = struct{}{}
		refs = append(refs, domain.ScenarioRef{Serial: parts[0], ScenarioID: parts[1]})
	}
	return refs, nil
}

func (m *MemoryStore) ParseScenario(ctx context.Context, serial, scenarioID string) (domain.ScenarioDoc, error) {
	data, _, err := m.GetFiles(ctx, serial, scenarioID)
	if err != nil {
		return domain.ScenarioDoc{}, err
	}
	var doc domain.ScenarioDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return domain.ScenarioDoc{}, domain.ErrInvalidYAML
	}
	if doc.ID == "" {
		doc.ID = scenarioID
	}
	if doc.Serial == "" {
		doc.Serial = serial
	}
	return doc, nil
}

// MemoryObjectStorage adapts MemoryStore for ObjectStorage (avoids Delete name clash).
type MemoryObjectStorage struct {
	*MemoryStore
}

func NewMemoryObjectStorage(m *MemoryStore) *MemoryObjectStorage {
	return &MemoryObjectStorage{MemoryStore: m}
}

func (m *MemoryObjectStorage) Put(ctx context.Context, key string, data []byte, _ string) error {
	return m.PutObject(ctx, key, data, "")
}

func (m *MemoryObjectStorage) Get(ctx context.Context, key string) ([]byte, error) {
	return m.GetObject(ctx, key)
}

func (m *MemoryObjectStorage) Delete(ctx context.Context, key string) error {
	return m.DeleteObject(ctx, key)
}

func (m *MemoryObjectStorage) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	return m.MemoryStore.ListPrefix(ctx, prefix)
}

func (m *MemoryObjectStorage) Ping(ctx context.Context) error {
	return m.MemoryStore.Ping(ctx)
}

func (m *MemoryStore) PutObject(ctx context.Context, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key] = append([]byte(nil), data...)
	return nil
}

func (m *MemoryStore) GetObject(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return data, nil
}

func (m *MemoryStore) DeleteObject(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key)
	return nil
}

func (m *MemoryStore) ListPrefix(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.files {
		if hasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *MemoryStore) Ping(context.Context) error { return nil }

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func splitKey(k string) []string {
	// serial/id/file
	var parts []string
	start := 0
	for i := 0; i < len(k); i++ {
		if k[i] == '/' {
			parts = append(parts, k[start:i])
			start = i + 1
		}
	}
	parts = append(parts, k[start:])
	return parts
}
