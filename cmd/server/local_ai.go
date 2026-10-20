package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const localAIPrompt = `You classify email metadata for a private job application tracker.
Return only data matching the JSON schema.

Rules:
- is_application is true only when the email concerns the user's own application.
- Job alerts, recommended vacancies, newsletters, recruiter marketing and adverts are false.
- "job offer" often means a vacancy/listing. Status Offer requires the employer actually offering this candidate employment.
- Applied includes submission confirmations and acknowledgements.
- Interviewing includes interview invitations, assessments, screening calls and next rounds.
- A description of how a company may assess a CV, suggested outreach, interview advice, or generic "next step" wording is not an interview invitation; keep it Applied.
- Declined includes rejections, closed roles and another candidate being selected.
- Wording such as "not progress your application further", "not taking your application forward", or "unsuccessful" means Declined, never Applied.
- Extract the actual employer, not the job board or applicant-tracking platform.
- Use an empty string when company or role cannot be determined.
- Confidence must reflect evidence in this email, not a guess.`

type localAI struct {
	baseURL string
	model   string
	client  *http.Client
	mu      sync.RWMutex
	enabled bool
}

type localAIResult struct {
	IsApplication bool    `json:"is_application"`
	Company       string  `json:"company"`
	Role          string  `json:"role"`
	Status        string  `json:"status"`
	Confidence    float64 `json:"confidence"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	} `json:"models"`
}

func newLocalAI(rawURL, model string, enabled bool) (*localAI, error) {
	parsed, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() == "" {
		return nil, errors.New("OLLAMA_URL must be a local HTTP URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && !net.ParseIP(host).IsLoopback() {
		return nil, errors.New("OLLAMA_URL must point to localhost; remote AI endpoints are not allowed")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("OLLAMA_MODEL cannot be empty")
	}
	return &localAI{
		baseURL: strings.TrimRight(rawURL, "/"),
		model:   strings.TrimSpace(model),
		enabled: enabled,
		client:  &http.Client{Timeout: 45 * time.Second},
	}, nil
}

func (ai *localAI) isEnabled() bool {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	return ai.enabled
}

func (ai *localAI) setEnabled(enabled bool) {
	ai.mu.Lock()
	ai.enabled = enabled
	ai.mu.Unlock()
}

func (ai *localAI) availability(ctx context.Context) (available, modelInstalled bool) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, ai.baseURL+"/api/tags", nil)
	if err != nil {
		return false, false
	}
	response, err := ai.client.Do(request)
	if err != nil {
		return false, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, false
	}
	var tags ollamaTagsResponse
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tags) != nil {
		return true, false
	}
	for _, installed := range tags.Models {
		if installed.Name == ai.model || installed.Model == ai.model || strings.TrimSuffix(installed.Name, ":latest") == strings.TrimSuffix(ai.model, ":latest") {
			return true, true
		}
	}
	return true, false
}

func (ai *localAI) classify(ctx context.Context, from, subject, snippet string) (localAIResult, error) {
	input := fmt.Sprintf("From: %s\nSubject: %s\nSnippet: %s", limitText(from, 300), limitText(subject, 500), limitText(snippet, 1200))
	format := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"is_application": map[string]any{"type": "boolean"},
			"company":        map[string]any{"type": "string"},
			"role":           map[string]any{"type": "string"},
			"status":         map[string]any{"type": "string", "enum": []string{"Applied", "Interviewing", "Offer", "Declined"}},
			"confidence":     map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required": []string{"is_application", "company", "role", "status", "confidence"},
	}
	payload := map[string]any{
		"model": ai.model,
		"messages": []map[string]string{
			{"role": "system", "content": localAIPrompt},
			{"role": "user", "content": input},
		},
		"stream":  false,
		"think":   false,
		"format":  format,
		"options": map[string]any{"temperature": 0},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return localAIResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ai.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return localAIResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := ai.client.Do(request)
	if err != nil {
		return localAIResult{}, fmt.Errorf("local AI unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return localAIResult{}, fmt.Errorf("local AI returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var chat ollamaChatResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&chat); err != nil {
		return localAIResult{}, err
	}
	var result localAIResult
	if err := json.Unmarshal([]byte(chat.Message.Content), &result); err != nil {
		return localAIResult{}, fmt.Errorf("local AI returned invalid JSON: %w", err)
	}
	return validateLocalAIResult(result)
}

func validateLocalAIResult(result localAIResult) (localAIResult, error) {
	result.Company = cleanCompany(limitText(result.Company, 100))
	result.Role = cleanRole(limitText(result.Role, 140))
	validStatus := result.Status == "Applied" || result.Status == "Interviewing" || result.Status == "Offer" || result.Status == "Declined"
	if !validStatus || result.Confidence < 0 || result.Confidence > 1 {
		return localAIResult{}, errors.New("local AI returned values outside the allowed schema")
	}
	return result, nil
}

func limitText(value string, maximum int) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}
