package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const (
	ProfileKey = "semantic-dev-hash-v1"
	Dimensions = 1024
)

// HashProvider is deliberately deterministic and dependency-free. It keeps
// the outbox/index/search pipeline testable until a real embedding provider is
// selected and benchmarked; it must not be presented as production semantic
// quality.
type HashProvider struct{}

func (HashProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vector := make([]float64, Dimensions)
	for _, token := range tokens(text) {
		digest := sha256.Sum256([]byte(token))
		index := binary.BigEndian.Uint32(digest[:4]) % Dimensions
		magnitude := 1.0 + float64(digest[4]%7)/7.0
		if digest[5]&1 == 1 {
			magnitude = -magnitude
		}
		vector[index] += magnitude
	}
	norm := 0.0
	for _, value := range vector {
		norm += value * value
	}
	if norm == 0 {
		return make([]float32, Dimensions), nil
	}
	norm = math.Sqrt(norm)
	encoded := make([]float32, Dimensions)
	for i, value := range vector {
		encoded[i] = float32(value / norm)
	}
	return encoded, nil
}

func VectorLiteral(vector []float32) string {
	values := make([]string, len(vector))
	for i, value := range vector {
		values[i] = strconv.FormatFloat(float64(value), 'f', 8, 32)
	}
	return "[" + strings.Join(values, ",") + "]"
}

// EmbedBatch implements Provider; hashing is local and stateless, so the
// batch is computed per text.
func (p HashProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vector, err := p.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vector)
	}
	return vectors, nil
}

func tokens(text string) []string {
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		builder.WriteByte(0)
	}
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return strings.FieldsFunc(builder.String(), func(r rune) bool { return r == 0 })
}
