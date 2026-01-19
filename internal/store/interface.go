package store

import "github.com/kubechronicle/kubechronicle/internal/model"

// Store defines the interface for persisting change events.
// This is a placeholder for Phase 4 implementation.
type Store interface {
	Save(event *model.ChangeEvent) error
	Close() error
}
