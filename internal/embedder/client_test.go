package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ok-folio/internal/database"
)

func TestClientEmbedValidatesModelAndDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(HealthResponse{OK: true, Model: ModelID, Dim: database.EmbeddingDim})
		case "/embed":
			embedding := make([]float32, database.EmbeddingDim)
			embedding[0] = 1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"embedding": embedding,
				"model":     ModelID,
				"dim":       database.EmbeddingDim,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	if _, err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	embedding, err := client.Embed(context.Background(), []byte("jpeg"))
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(embedding) != database.EmbeddingDim || embedding[0] != 1 {
		t.Fatalf("unexpected embedding: len=%d first=%f", len(embedding), embedding[0])
	}
}

func TestClientEmbedRejectsWrongDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float32{1, 2, 3},
			"model":     ModelID,
			"dim":       3,
		})
	}))
	defer server.Close()

	if _, err := New(server.URL).Embed(context.Background(), []byte("jpeg")); err == nil {
		t.Fatal("expected wrong-dimension response to fail")
	}
}
