package contexting

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const CurrentIndexSchema = 1

type ContextIndex struct {
	SchemaVersion int       `json:"schema_version"`
	RootPath      string    `json:"root_path"`
	GeneratedAt   time.Time `json:"generated_at"`
	Model         string    `json:"model,omitempty"`
	Tree          *Node     `json:"tree"`
}

func SaveContextIndex(path string, index *ContextIndex) error {
	if index == nil {
		return fmt.Errorf("index is nil")
	}

	if index.SchemaVersion < 0 || index.SchemaVersion > CurrentIndexSchema {
		return fmt.Errorf("unsupported index schema %d", index.SchemaVersion)
	}
	snapshot := *index
	snapshot.SchemaVersion = CurrentIndexSchema
	bytes, err := json.MarshalIndent(&snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	if err := writeFileAtomic(path, append(bytes, '\n'), 0o600); err != nil {
		return fmt.Errorf("write index file: %w", err)
	}

	return nil
}

func LoadContextIndex(path string) (*ContextIndex, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index file: %w", err)
	}

	var index ContextIndex
	if err := json.Unmarshal(bytes, &index); err != nil {
		return nil, fmt.Errorf("parse index file: %w", err)
	}
	if index.SchemaVersion < 0 || index.SchemaVersion > CurrentIndexSchema {
		return nil, fmt.Errorf("unsupported index schema %d; upgrade ctxt or rebuild the index", index.SchemaVersion)
	}
	if index.Tree == nil {
		return nil, fmt.Errorf("index has no tree; rebuild with ctxt init")
	}

	return &index, nil
}
