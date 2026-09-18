package policy

import (
	"context"
	"testing"
)

func TestRejectsPrivateAndLocalTargets(t *testing.T) {
	p, err := New("open", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"http://127.0.0.1", "http://localhost", "http://169.254.169.254"} {
		if _, err := p.Validate(context.Background(), target); err == nil {
			t.Fatalf("expected %s to be rejected", target)
		}
	}
}

func TestAllowsConfiguredDomain(t *testing.T) {
	p, err := New("allowlist", []string{"example.com"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Validate(context.Background(), "example.com"); err != nil {
		t.Fatalf("expected configured domain: %v", err)
	}
	if _, err := p.Validate(context.Background(), "not-example.com"); err == nil {
		t.Fatal("expected non-allowlisted domain to be rejected")
	}
}
