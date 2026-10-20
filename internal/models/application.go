// Package models contains the data exchanged between Sift's backend layers.
package models

type Application struct {
	ID        string `json:"id"`
	ThreadID  string `json:"threadId,omitempty"`
	Company   string `json:"company"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	Closed    bool   `json:"closed,omitempty"`
	LastEvent string `json:"lastEvent"`
	UpdatedAt string `json:"updatedAt"`
}

type BlockedRole struct {
	Role      string `json:"role,omitempty"`
	MessageID string `json:"messageId,omitempty"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type BlockUpdate struct {
	Role      string `json:"role"`
	MessageID string `json:"messageId"`
	Label     string `json:"label"`
	Blocked   bool   `json:"blocked"`
}

type SyncState struct {
	LastSuccessfulAt string `json:"lastSuccessfulAt"`
}

type SyncStatus struct {
	Running   bool   `json:"running"`
	Phase     string `json:"phase"`
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
}
