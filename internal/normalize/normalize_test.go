package normalize

import (
	"reconix-cloud/internal/model"
	"testing"
)

func TestResultPreservesEvidence(t *testing.T) {
	raw := []byte(`{"schema_version":"1.0","scan":{"target":"example.com"},"results":{"port":[{"type":"port","evidence":{"port":443,"state":"open"}}]}}`)
	result, err := Result(model.Scan{ID: "scan_test", Target: "example.com"}, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Evidence["port"] != float64(443) {
		t.Fatalf("unexpected result: %#v", result)
	}
}
