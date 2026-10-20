// Package auth implements Google OAuth and access-token refresh for Gmail.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Kacperkg/sift/internal/apperrors"
	"github.com/Kacperkg/sift/internal/database"
)

const (
	readOnlyScope  = "https://www.googleapis.com/auth/gmail.readonly"
	refreshAccount = "gmail-refresh-token"
)

type Google struct {
	clientID        string
	clientSecret    string
	redirectURL     string
	keychainService string
	client          *http.Client
	mu              sync.Mutex
	states          map[string]time.Time
	accessToken     string
	tokenExpiry     time.Time
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func NewGoogle(clientID, clientSecret, redirectURL, keychainService string) *Google {
	return &Google{
		clientID: clientID, clientSecret: clientSecret, redirectURL: redirectURL,
		keychainService: keychainService, client: http.DefaultClient, states: map[string]time.Time{},
	}
}

func (google *Google) Configured() bool {
	return google.clientID != "" && google.clientSecret != ""
}

func (google *Google) Connected() bool {
	_, err := database.GetSecret(google.keychainService, refreshAccount)
	return err == nil
}

func (google *Google) AuthorizationURL() (string, error) {
	if !google.Configured() {
		return "", fmt.Errorf("%w: Google OAuth is not configured", apperrors.ErrUnauthorized)
	}
	state, err := database.RandomToken(32)
	if err != nil {
		return "", fmt.Errorf("create OAuth state: %w", err)
	}
	google.mu.Lock()
	google.states[state] = time.Now().Add(10 * time.Minute)
	google.mu.Unlock()
	query := url.Values{
		"client_id": {google.clientID}, "redirect_uri": {google.redirectURL},
		"response_type": {"code"}, "scope": {readOnlyScope}, "access_type": {"offline"},
		"prompt": {"consent"}, "state": {state},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + query.Encode(), nil
}

func (google *Google) Complete(ctx context.Context, state, code, oauthError string) error {
	google.mu.Lock()
	expiry, ok := google.states[state]
	delete(google.states, state)
	google.mu.Unlock()
	if !ok || time.Now().After(expiry) {
		return fmt.Errorf("%w: invalid or expired OAuth state", apperrors.ErrInvalidInput)
	}
	if oauthError != "" {
		return fmt.Errorf("%w: Google authorization failed: %s", apperrors.ErrInvalidInput, oauthError)
	}
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("%w: missing authorization code", apperrors.ErrInvalidInput)
	}
	token, err := google.tokenRequest(ctx, url.Values{
		"code": {code}, "client_id": {google.clientID}, "client_secret": {google.clientSecret},
		"redirect_uri": {google.redirectURL}, "grant_type": {"authorization_code"},
	})
	if err != nil {
		return err
	}
	if token.RefreshToken != "" {
		if err := database.SetSecret(google.keychainService, refreshAccount, token.RefreshToken); err != nil {
			return fmt.Errorf("store refresh token: %w", err)
		}
	}
	google.cache(token)
	return nil
}

func (google *Google) AccessToken(ctx context.Context) (string, error) {
	google.mu.Lock()
	if google.accessToken != "" && time.Now().Before(google.tokenExpiry) {
		token := google.accessToken
		google.mu.Unlock()
		return token, nil
	}
	google.mu.Unlock()
	refreshToken, err := database.GetSecret(google.keychainService, refreshAccount)
	if err != nil {
		return "", fmt.Errorf("%w: Gmail is not connected", apperrors.ErrUnauthorized)
	}
	token, err := google.tokenRequest(ctx, url.Values{
		"refresh_token": {refreshToken}, "client_id": {google.clientID},
		"client_secret": {google.clientSecret}, "grant_type": {"refresh_token"},
	})
	if err != nil {
		return "", err
	}
	google.cache(token)
	return token.AccessToken, nil
}

func (google *Google) cache(token tokenResponse) {
	google.mu.Lock()
	google.accessToken = token.AccessToken
	lifetime := time.Duration(token.ExpiresIn-60) * time.Second
	if lifetime < 0 {
		lifetime = 0
	}
	google.tokenExpiry = time.Now().Add(lifetime)
	google.mu.Unlock()
}

func (google *Google) tokenRequest(ctx context.Context, form url.Values) (tokenResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := google.client.Do(request)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("Google token request: %w", err)
	}
	defer response.Body.Close()
	var token tokenResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token); err != nil {
		return token, err
	}
	if response.StatusCode != http.StatusOK || token.Error != "" {
		return token, fmt.Errorf("Google token exchange failed: %s", token.Error)
	}
	return token, nil
}
