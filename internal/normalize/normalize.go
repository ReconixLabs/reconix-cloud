package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reconix-cloud/internal/model"
)

type native struct {
	Results map[string][]struct {
		Type     string         `json:"type"`
		Evidence map[string]any `json:"evidence"`
	} `json:"results"`
	Scan struct {
		Target string `json:"target"`
	} `json:"scan"`
}

func Result(scan model.Scan, raw []byte) (model.NormalizedResult, error) {
	var input native
	if err := json.Unmarshal(raw, &input); err != nil {
		return model.NormalizedResult{}, err
	}
	out := model.NormalizedResult{SchemaVersion: "1.0", ScanID: scan.ID, Target: scan.Target, Findings: []model.Finding{}}
	for category, observations := range input.Results {
		for _, observation := range observations {
			evidence := observation.Evidence
			title := fmt.Sprintf("%s observation", category)
			if name, ok := evidence["name"].(string); ok && name != "" {
				title = name
			}
			idBytes := sha256.Sum256([]byte(scan.ID + category + fmt.Sprint(evidence)))
			out.Findings = append(out.Findings, model.Finding{FindingID: "finding_" + hex.EncodeToString(idBytes[:8]), Source: "reconix", Type: observation.Type, Title: title, Severity: severity(evidence), Evidence: evidence, Metadata: map[string]any{"category": category}})
		}
	}
	return out, nil
}
func severity(e map[string]any) string {
	if value, ok := e["severity"].(string); ok && value != "" {
		return value
	}
	return "info"
}
