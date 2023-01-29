package upgrade

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func NewSafeWriter(opts ...SafeWriterOption) *SafeWriter {
	var cfg SafeWriterConfig

	cfg.Option(opts...)
	cfg.Default()

	return &SafeWriter{
		cfg: cfg,
	}
}

type SafeWriter struct {
	cfg SafeWriterConfig
}

func (w *SafeWriter) Write(path string, data []byte) error {
	backup, err := w.backup(path)
	if err != nil {
		return fmt.Errorf("backing up path: %w", err)
	}

	const perms = os.FileMode(0o755)

	if err := os.WriteFile(path, data, perms); err != nil {
		if backup != "" {
			if err := os.Rename(backup, path); err != nil {
				w.cfg.Log.Errorf("restoring from backup: %v", err)
			}
		}

		return fmt.Errorf("writing to path %q: %w", path, err)
	}

	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("cleaning up old binary: %w", err)
	}

	return nil
}

var ErrNotAFile = errors.New("not a file")

func (w *SafeWriter) backup(path string) (string, error) {
	if stat, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("inspecting file path: %w", err)
	} else if stat.IsDir() {
		return "", ErrNotAFile
	}

	backup := path + "_" + time.Now().Format("2006.01.02_15:04:05")

	if err := os.Rename(path, backup); err != nil {
		return "", fmt.Errorf("backing up file path: %w", err)
	}

	return backup, nil
}

type SafeWriterConfig struct {
	Log logrus.FieldLogger
}

func (c *SafeWriterConfig) Option(opts ...SafeWriterOption) {
	for _, opt := range opts {
		opt.ConfigureSafeWriter(c)
	}
}

func (c *SafeWriterConfig) Default() {
	if c.Log == nil {
		c.Log = logrus.New()
	}
}

type SafeWriterOption interface {
	ConfigureSafeWriter(*SafeWriterConfig)
}
