// Package tmplsvc owns template YAML persistence for tdx. It composes
// config.Paths (for the templates directory) and a timesvc.Service (for
// any future operations that need to resolve time entries from a template).
package tmplsvc

import (
	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// Service is the template operations facade. It holds a Store for YAML CRUD
// and a timesvc.Service for future apply/reconcile operations.
type Service struct {
	paths config.Paths
	store *Store
	tsvc  *timesvc.Service
}

// New constructs a Service rooted at the given paths, composing a Store and
// the provided timesvc.Service.
func New(paths config.Paths, tsvc *timesvc.Service) *Service {
	return &Service{
		paths: paths,
		store: NewStore(paths),
		tsvc:  tsvc,
	}
}

// Store returns the underlying template Store for direct CRUD access.
func (s *Service) Store() *Store { return s.store }
