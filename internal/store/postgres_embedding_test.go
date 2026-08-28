package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/embedding"
)

type countingEmbedder struct {
	mu    sync.Mutex
	calls int
	wait  time.Duration
}

func (e *countingEmbedder) Embed(ctx context.Context, _ string) ([]float32, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	if e.wait > 0 {
		select {
		case <-time.After(e.wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return make([]float32, embedding.Dimensions), nil
}

func (e *countingEmbedder) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return nil, nil
}

func (e *countingEmbedder) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

func TestQueryEmbeddingCoalescesConcurrentMissesAndCachesResult(t *testing.T) {
	provider := &countingEmbedder{wait: 20 * time.Millisecond}
	postgres := &Postgres{
		embedder:         provider,
		embeddingProfile: "semantic-v1",
	}

	const requests = 8
	results := make([][]float32, requests)
	errors := make([]error, requests)
	var group sync.WaitGroup
	group.Add(requests)
	for index := range requests {
		go func(index int) {
			defer group.Done()
			results[index], errors[index] = postgres.queryEmbedding(context.Background(), "event loop")
		}(index)
	}
	group.Wait()

	if provider.count() != 1 {
		t.Fatalf("expected one provider call for concurrent identical misses, got %d", provider.count())
	}
	for index, err := range errors {
		if err != nil {
			t.Fatalf("request %d failed: %v", index, err)
		}
		if len(results[index]) != embedding.Dimensions {
			t.Fatalf("request %d returned %d dimensions", index, len(results[index]))
		}
	}

	results[0][0] = 99
	cached, err := postgres.queryEmbedding(context.Background(), "event loop")
	if err != nil {
		t.Fatalf("cached request failed: %v", err)
	}
	if provider.count() != 1 {
		t.Fatalf("expected cached request to avoid provider, got %d calls", provider.count())
	}
	if cached[0] == 99 {
		t.Fatal("cached vector leaked the caller's mutable slice")
	}
}

func TestQueryEmbeddingUsesProfileAsCacheBoundary(t *testing.T) {
	provider := &countingEmbedder{}
	postgres := &Postgres{embedder: provider, embeddingProfile: "semantic-v1"}
	if _, err := postgres.queryEmbedding(context.Background(), "same text"); err != nil {
		t.Fatal(err)
	}
	postgres.embeddingProfile = "semantic-v2"
	if _, err := postgres.queryEmbedding(context.Background(), "same text"); err != nil {
		t.Fatal(err)
	}
	if provider.count() != 2 {
		t.Fatalf("expected profile switch to bypass cache, got %d provider calls", provider.count())
	}
}
