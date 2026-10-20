package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalAIRejectsRemoteEndpoint(t *testing.T) {
	if _, err := NewLocalAI("https://example.com", "qwen3:4b", true); err == nil {
		t.Fatal("accepted a remote AI endpoint")
	}
}

func TestLocalAIUsesStructuredLocalResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["format"] == nil || request["stream"] != false {
			t.Fatal("request did not enforce a non-streaming JSON schema")
		}
		response := map[string]any{"message": map[string]string{"content": `{"is_application":true,"company":"Link Group","role":"Junior React Developer","status":"Applied","confidence":0.97}`}}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ai, err := NewLocalAI(server.URL, "qwen3:4b", true)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ai.classify(context.Background(), "No Fluff Jobs", "Your application", "Job offer: Junior React Developer")
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsApplication || result.Company != "Link Group" || result.Status != "Applied" || result.Confidence != 0.97 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestShouldAskLocalAIOnlyForAmbiguousMessages(t *testing.T) {
	if shouldAskLocalAI("Your application for Engineer at Acme", "Acme", "Engineer", true) {
		t.Fatal("asked AI to reclassify a complete deterministic result")
	}
	if !shouldAskLocalAI("Your application update", "Unknown company", "Role not detected", true) {
		t.Fatal("did not ask AI about an incomplete result")
	}
	if shouldAskLocalAI("Recommended jobs for you", "", "", false) {
		t.Fatal("asked AI about obvious alert noise")
	}
}
