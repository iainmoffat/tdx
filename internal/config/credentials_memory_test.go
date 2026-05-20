package config

import (
	"fmt"
	"sync"

	"github.com/iainmoffat/tdx/internal/domain"
)

// memoryBackend is an in-process token backend used only by tests. Lives in
// a _test.go file so it's not compiled into production binaries.
type memoryBackend struct {
	mu     sync.Mutex
	tokens map[string]string

	// failNextSet, if true, causes the next Set call to return failSetErr.
	// Test machinery uses this to exercise migration partial-failure paths.
	failNextSet bool
	failSetErr  error
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{tokens: map[string]string{}}
}

func (b *memoryBackend) Name() string { return "memory" }

func (b *memoryBackend) Get(profile string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.tokens[profile]
	if !ok || t == "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	return t, nil
}

func (b *memoryBackend) Set(profile, token string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failNextSet {
		b.failNextSet = false
		return b.failSetErr
	}
	b.tokens[profile] = token
	return nil
}

func (b *memoryBackend) Clear(profile string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.tokens, profile)
	return nil
}
