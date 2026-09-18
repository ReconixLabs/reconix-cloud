package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reconix-cloud/internal/model"
)

type ReconRunner interface {
	Run(context.Context, model.Scan, string) ([]byte, error)
}

type Subprocess struct {
	Binary, Config string
	MaxOutput      int64
	Workdir        string
}

type limitedWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("output limit exceeded")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func (r Subprocess) Run(ctx context.Context, scan model.Scan, target string) ([]byte, error) {
	configPath := r.Config
	if scan.Profile == "safe" || scan.Profile == "deep" {
		data := []byte("threads: 10\ntimeout: 10\ntemplates_dir: templates\nports:\n  - 80\n  - 443\n")
		if scan.Profile == "deep" {
			data = []byte("threads: 30\ntimeout: 15\ntemplates_dir: templates\nports:\n  - 21\n  - 22\n  - 25\n  - 53\n  - 80\n  - 443\n  - 8080\n  - 8443\n  - 8000\n  - 3000\n")
		}
		file, err := os.CreateTemp("", "reconix-profile-*.yaml")
		if err != nil {
			return nil, err
		}
		configPath = file.Name()
		if _, err = file.Write(data); err != nil {
			file.Close()
			os.Remove(configPath)
			return nil, err
		}
		file.Close()
		defer os.Remove(configPath)
	}
	cmd := exec.CommandContext(ctx, r.Binary, "-config", configPath, "-json", target)
	if r.Workdir != "" {
		cmd.Dir = r.Workdir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &limitedWriter{writer: &stdout, remaining: r.MaxOutput}, &limitedWriter{writer: &stderr, remaining: r.MaxOutput}
	err := cmd.Run()
	if int64(stdout.Len()) > r.MaxOutput {
		return nil, fmt.Errorf("reconix output exceeded limit")
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("reconix failed: %s", stderr.String())
	}
	var document json.RawMessage
	if json.Unmarshal(stdout.Bytes(), &document) != nil {
		return nil, fmt.Errorf("reconix returned invalid JSON")
	}
	return stdout.Bytes(), nil
}
