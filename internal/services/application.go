package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Kacperkg/sift/internal/apperrors"
	"github.com/Kacperkg/sift/internal/models"
	"github.com/Kacperkg/sift/internal/repository"
)

type TokenProvider interface {
	AccessToken(context.Context) (string, error)
}

type ApplicationService struct {
	repository repository.ApplicationRepository
	tokens     TokenProvider
	localAI    *LocalAI
	progressMu sync.RWMutex
	progress   models.SyncStatus
}

func NewApplicationService(repository repository.ApplicationRepository, tokens TokenProvider, localAI *LocalAI) *ApplicationService {
	return &ApplicationService{repository: repository, tokens: tokens, localAI: localAI}
}

func (service *ApplicationService) ListApplications() ([]models.Application, error) {
	return service.repository.ListApplications()
}

func (service *ApplicationService) CloseInterview(id, outcome string) (models.Application, error) {
	id = strings.TrimSpace(id)
	outcome = strings.TrimSpace(outcome)
	if id == "" || (outcome != "Accepted" && outcome != "Declined") {
		return models.Application{}, fmt.Errorf("%w: outcome must be Accepted or Declined", apperrors.ErrInvalidInput)
	}
	return service.repository.CloseInterview(id, outcome)
}

func (service *ApplicationService) ListBlockedItems() ([]models.BlockedRole, error) {
	return service.repository.ListBlockedItems()
}

func (service *ApplicationService) UpdateBlockedItem(update models.BlockUpdate) ([]models.BlockedRole, error) {
	update.Role = strings.TrimSpace(strings.Join(strings.Fields(update.Role), " "))
	update.MessageID = strings.TrimSpace(update.MessageID)
	update.Label = strings.TrimSpace(strings.Join(strings.Fields(update.Label), " "))
	if (update.Role == "" && update.MessageID == "") || len(update.Role) > 140 || len(update.MessageID) > 100 || len(update.Label) > 180 {
		return nil, fmt.Errorf("%w: invalid blocked item", apperrors.ErrInvalidInput)
	}
	return service.repository.UpdateBlockedItem(update)
}

func (service *ApplicationService) Sync(ctx context.Context, fullScan bool) ([]models.Application, error) {
	if !service.beginSync() {
		return nil, fmt.Errorf("%w: a Gmail scan is already running", apperrors.ErrConflict)
	}
	finished := false
	defer func() {
		if !finished {
			service.finishSync("Scan stopped")
		}
	}()

	token, err := service.tokens.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	existing, _, state, err := service.repository.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	var after time.Time
	if !fullScan && state.LastSuccessfulAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, state.LastSuccessfulAt); parseErr == nil {
			after = parsed.Add(-5 * time.Minute)
		}
	}
	if after.IsZero() {
		fullScan = true
	}

	found, err := scanGmail(ctx, token, service.localAI, after, service.reportSyncProgress)
	if err != nil {
		return nil, err
	}
	applications := found
	if fullScan {
		applications = carryClosedOutcomes(existing, found)
	} else {
		applications = mergeApplications(existing, found)
	}
	service.reportSyncProgress(models.SyncStatus{Phase: "Saving private index", Processed: len(applications), Total: len(applications)})
	applications, err = service.repository.SaveSync(applications, models.SyncState{LastSuccessfulAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	completion := "Inbox updated"
	if fullScan {
		completion = "Full scan complete"
	}
	service.finishSync(completion)
	finished = true
	return applications, nil
}

func (service *ApplicationService) SyncStatus() models.SyncStatus {
	service.progressMu.RLock()
	defer service.progressMu.RUnlock()
	return service.progress
}

func (service *ApplicationService) beginSync() bool {
	service.progressMu.Lock()
	defer service.progressMu.Unlock()
	if service.progress.Running {
		return false
	}
	service.progress = models.SyncStatus{Running: true, Phase: "Starting scan"}
	return true
}

func (service *ApplicationService) reportSyncProgress(progress models.SyncStatus) {
	service.progressMu.Lock()
	service.progress = models.SyncStatus{Running: true, Phase: progress.Phase, Processed: progress.Processed, Total: progress.Total}
	service.progressMu.Unlock()
}

func (service *ApplicationService) finishSync(phase string) {
	service.progressMu.Lock()
	service.progress.Running = false
	service.progress.Phase = phase
	service.progressMu.Unlock()
}
