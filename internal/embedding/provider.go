package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"bytes"
	"time"
)

// Provider produces embedding vectors for text. Implementations must return
// vectors with exactly Dimensions components so they can be stored in
// content.question_embedding (vector(1024) plus a CHECK constraint).
type Provider interface {
	// Embed returns the vector for one text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch returns vectors for many texts in one round trip when the
	// backend supports it; implementations may fall back to per-text calls.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// OllamaProvider calls a local Ollama server's /api/embed endpoint. It is the
// production retrieval provider for this stack; the endpoint stays loopback or
// Compose-network local by policy, and no hosted API keys are involved.
type OllamaProvider struct {
	Endpoint string // e.g. http://host.docker.internal:11434
	Model    string // e.g. bge-m3 (multilingual, 1024 dimensions)
	Client   *http.Client
}

// NewOllamaProvider builds a provider with bounded HTTP timeouts.
func NewOllamaProvider(endpoint, model string) OllamaProvider {
	return OllamaProvider{
		Endpoint: endpoint,
		Model:    model,
		Client:   &http.Client{Timeout: 60 * time.Second},
	}
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error"`
}

// Embed implements Provider.
func (p OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

// EmbedBatch implements Provider using the batched /api/embed contract.
func (p OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	requestBody, err := json.Marshal(ollamaEmbedRequest{Model: p.Model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("encode ollama embed request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint+"/api/embed", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("build ollama embed request: %w", err)
	}
	request.Header.Set("content-type", "application/json")
	response, err := p.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call ollama embed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed returned status %d", response.StatusCode)
	}
	var decoded ollamaEmbedResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode ollama embed response: %w", err)
	}
	if decoded.Error != "" {
		return nil, fmt.Errorf("ollama embed error: %s", decoded.Error)
	}
	if len(decoded.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embed returned %d vectors for %d inputs", len(decoded.Embeddings), len(texts))
	}
	for _, vector := range decoded.Embeddings {
		if len(vector) != Dimensions {
			return nil, fmt.Errorf("ollama model %q returned %d dimensions, expected %d", p.Model, len(vector), Dimensions)
		}
	}
	return decoded.Embeddings, nil
}

// Static assertion that the hash provider keeps satisfying the interface.
var _ Provider = HashProvider{}
var _ Provider = OllamaProvider{}
