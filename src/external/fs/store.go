package fs

import (
	"context"
	"fmt"
	"io"
	"os"
)

// maxReadBytes guards memory here; trimming for the model's context window is
// the tool's job, not this layer's.
const maxReadBytes = 1 << 20

// Reader exposes one directory for reading. Paths come from a language model,
// so containment relies on the directory handle rather than on inspecting the
// path string.
type Reader struct {
	root *os.Root
}

func NewReader(dir string) (*Reader, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open data dir %q: %w", dir, err)
	}
	return &Reader{root: root}, nil
}

func (r *Reader) Close() error { return r.root.Close() }

func (r *Reader) ReadFile(_ context.Context, path string) (string, error) {
	file, err := r.root.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxReadBytes))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Writer exposes one directory for writing — deliberately not the one Reader
// serves, so the agent cannot overwrite its own inputs.
type Writer struct {
	root *os.Root
	dir  string
}

func NewWriter(dir string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir %q: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open output dir %q: %w", dir, err)
	}
	return &Writer{root: root, dir: dir}, nil
}

func (w *Writer) Close() error { return w.root.Close() }

func (w *Writer) WriteFile(_ context.Context, path, content string) (int, string, error) {
	file, err := w.root.Create(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	n, err := file.WriteString(content)
	if err != nil {
		return 0, "", err
	}
	if err := file.Close(); err != nil {
		return 0, "", err
	}
	return n, w.dir + "/" + path, nil
}
