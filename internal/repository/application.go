// Package repository owns persistence operations for Sift's application data.
package repository

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kacperkg/sift/internal/apperrors"
	"github.com/Kacperkg/sift/internal/database"
	"github.com/Kacperkg/sift/internal/models"
)

type ApplicationRepository interface {
	ListApplications() ([]models.Application, error)
	CloseInterview(id, outcome string) (models.Application, error)
	ListBlockedItems() ([]models.BlockedRole, error)
	UpdateBlockedItem(update models.BlockUpdate) ([]models.BlockedRole, error)
	LoadSnapshot() ([]models.Application, []models.BlockedRole, models.SyncState, error)
	SaveSync([]models.Application, models.SyncState) ([]models.Application, error)
}

type EncryptedApplicationRepository struct {
	applications *database.Store
	blocks       *database.Store
	syncState    *database.Store
	mu           sync.Mutex
}

func NewApplicationRepository(dataDir, keychainService string) (*EncryptedApplicationRepository, error) {
	applications, err := database.New(filepath.Join(dataDir, "jobs.enc"), keychainService)
	if err != nil {
		return nil, err
	}
	blocks, err := database.New(filepath.Join(dataDir, "blocked-roles.enc"), keychainService)
	if err != nil {
		return nil, err
	}
	syncState, err := database.New(filepath.Join(dataDir, "sync-state.enc"), keychainService)
	if err != nil {
		return nil, err
	}
	return &EncryptedApplicationRepository{applications: applications, blocks: blocks, syncState: syncState}, nil
}

func (repository *EncryptedApplicationRepository) ListApplications() ([]models.Application, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.loadApplications()
}

func (repository *EncryptedApplicationRepository) CloseInterview(id, outcome string) (models.Application, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	applications, err := repository.loadApplications()
	if err != nil {
		return models.Application{}, err
	}
	updated, err := closeInterview(applications, id, outcome)
	if err != nil {
		return models.Application{}, err
	}
	if err := repository.applications.Save(applications); err != nil {
		return models.Application{}, fmt.Errorf("save applications: %w", err)
	}
	return updated, nil
}

func (repository *EncryptedApplicationRepository) ListBlockedItems() ([]models.BlockedRole, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.loadBlockedItems()
}

func (repository *EncryptedApplicationRepository) UpdateBlockedItem(update models.BlockUpdate) ([]models.BlockedRole, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	blocks, err := repository.loadBlockedItems()
	if err != nil {
		return nil, err
	}
	blocks = updateBlockedItems(blocks, update.Role, update.MessageID, update.Label, update.Blocked)
	if err := repository.blocks.Save(blocks); err != nil {
		return nil, fmt.Errorf("save blocked roles: %w", err)
	}
	if update.Blocked {
		applications, err := repository.loadApplications()
		if err != nil {
			return nil, err
		}
		if err := repository.applications.Save(filterBlockedApplications(applications, blocks)); err != nil {
			return nil, fmt.Errorf("save filtered applications: %w", err)
		}
	}
	return blocks, nil
}

func (repository *EncryptedApplicationRepository) LoadSnapshot() ([]models.Application, []models.BlockedRole, models.SyncState, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	applications, err := repository.loadApplications()
	if err != nil {
		return nil, nil, models.SyncState{}, err
	}
	blocks, err := repository.loadBlockedItems()
	if err != nil {
		return nil, nil, models.SyncState{}, err
	}
	var state models.SyncState
	if err := repository.syncState.Load(&state); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, models.SyncState{}, fmt.Errorf("load sync state: %w", err)
	}
	return applications, blocks, state, nil
}

func (repository *EncryptedApplicationRepository) SaveSync(applications []models.Application, state models.SyncState) ([]models.Application, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	blocks, err := repository.loadBlockedItems()
	if err != nil {
		return nil, err
	}
	applications = filterBlockedApplications(applications, blocks)
	if err := repository.applications.Save(applications); err != nil {
		return nil, fmt.Errorf("save applications: %w", err)
	}
	if err := repository.syncState.Save(state); err != nil {
		return nil, fmt.Errorf("save sync state: %w", err)
	}
	return applications, nil
}

func (repository *EncryptedApplicationRepository) loadApplications() ([]models.Application, error) {
	var applications []models.Application
	if err := repository.applications.Load(&applications); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load applications: %w", err)
	}
	if applications == nil {
		applications = []models.Application{}
	}
	return applications, nil
}

func (repository *EncryptedApplicationRepository) loadBlockedItems() ([]models.BlockedRole, error) {
	var blocks []models.BlockedRole
	if err := repository.blocks.Load(&blocks); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load blocked roles: %w", err)
	}
	if blocks == nil {
		blocks = []models.BlockedRole{}
	}
	return blocks, nil
}

func closeInterview(applications []models.Application, id, outcome string) (models.Application, error) {
	for index := range applications {
		if applications[index].ID != id {
			continue
		}
		if applications[index].Status != "Interviewing" || applications[index].Closed {
			return models.Application{}, fmt.Errorf("%w: only an open interview can be closed", apperrors.ErrConflict)
		}
		applications[index].Status = outcome
		applications[index].Closed = true
		return applications[index], nil
	}
	return models.Application{}, fmt.Errorf("%w: application", apperrors.ErrNotFound)
}

func updateBlockedItems(blocks []models.BlockedRole, role, messageID, label string, blocked bool) []models.BlockedRole {
	item := models.BlockedRole{Role: role, MessageID: messageID, Label: label}
	key := blockedItemKey(item)
	result := make([]models.BlockedRole, 0, len(blocks)+1)
	found := false
	for _, existing := range blocks {
		if blockedItemKey(existing) == key {
			found = true
			if !blocked {
				continue
			}
		}
		result = append(result, existing)
	}
	if blocked && !found {
		item.CreatedAt = time.Now().Format(time.RFC3339)
		result = append(result, item)
	}
	return result
}

func blockedItemKey(item models.BlockedRole) string {
	if item.MessageID != "" {
		return "message:" + strings.ToLower(item.MessageID)
	}
	return "role:" + normalizeRole(item.Role)
}

func filterBlockedApplications(applications []models.Application, blocks []models.BlockedRole) []models.Application {
	blockedRoles := make(map[string]bool, len(blocks))
	blockedMessages := make(map[string]bool, len(blocks))
	for _, item := range blocks {
		if item.MessageID != "" {
			blockedMessages[strings.ToLower(item.MessageID)] = true
		} else if item.Role != "" {
			blockedRoles[normalizeRole(item.Role)] = true
		}
	}
	result := make([]models.Application, 0, len(applications))
	for _, application := range applications {
		if !blockedMessages[strings.ToLower(application.ID)] && !blockedRoles[normalizeRole(application.Role)] {
			result = append(result, application)
		}
	}
	return result
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.Join(strings.Fields(role), " "))
}
