package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ResourceSummary is a single resource entry from a terraform.tfstate file.
type ResourceSummary struct {
	Type string
	Name string
	ID   string
}

// StateResult is the output of ReadCurrentState.
type StateResult struct {
	Resources []ResourceSummary
	Message   string
}

// StateReader reads Terraform state from a workspace directory.
type StateReader struct {
	workingDir string
}

func NewStateReader(workingDir string) *StateReader {
	return &StateReader{workingDir: workingDir}
}

// ReadCurrentState reads terraform.tfstate from the workspace directory and
// returns a summary of existing resources. Returns an empty Resources slice
// (no error) when no state file exists.
func (r *StateReader) ReadCurrentState(ctx context.Context, workspaceID string) (StateResult, error) {
	statePath := filepath.Join(r.workingDir, workspaceID, "terraform.tfstate")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StateResult{Resources: []ResourceSummary{}, Message: "No existing state"}, nil
		}
		return StateResult{}, fmt.Errorf("read state file: %w", err)
	}

	// terraform.tfstate is a JSON document. We only need the resources array.
	var raw struct {
		Resources []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			Instances []struct {
				Attributes struct {
					ID string `json:"id"`
				} `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return StateResult{}, fmt.Errorf("parse state file: %w", err)
	}

	summaries := make([]ResourceSummary, 0, len(raw.Resources))
	for _, res := range raw.Resources {
		id := ""
		if len(res.Instances) > 0 {
			id = res.Instances[0].Attributes.ID
		}
		summaries = append(summaries, ResourceSummary{
			Type: res.Type,
			Name: res.Name,
			ID:   id,
		})
	}

	return StateResult{Resources: summaries}, nil
}
