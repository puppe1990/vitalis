package photos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// MemoryBucket is the in-process stand-in for Backblaze so handler and library
// tests never leave the machine.
type MemoryBucket struct {
	mu    sync.Mutex
	files map[string]memoryObject
}

type memoryObject struct {
	body        []byte
	contentType string
}

func NewMemoryBucket() *MemoryBucket {
	return &MemoryBucket{files: map[string]memoryObject{}}
}

// PublicURL is the native B2 download shape so diary tests assert the <img>
// hits the bucket, not the app proxy.
func (b *MemoryBucket) PublicURL(key string) string {
	return "https://f005.backblazeb2.com/file/memory/" + key
}

func (b *MemoryBucket) Put(_ context.Context, key string, body io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return fmt.Errorf("memory put %s: %w", key, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files[key] = memoryObject{body: data, contentType: contentType}
	return nil
}

func (b *MemoryBucket) Open(_ context.Context, key string) (Object, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	obj, ok := b.files[key]
	if !ok {
		return Object{}, fmt.Errorf("memory open %s: %w", key, ErrNotFound)
	}
	return Object{
		Body:        io.NopCloser(bytes.NewReader(obj.body)),
		ContentType: obj.contentType,
	}, nil
}

func (b *MemoryBucket) Remove(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.files[key]; !ok {
		return fmt.Errorf("memory remove %s: %w", key, ErrNotFound)
	}
	delete(b.files, key)
	return nil
}
