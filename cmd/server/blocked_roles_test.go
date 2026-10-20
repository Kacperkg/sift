package main

import "testing"

func TestBlockedRoleFilteringIsCaseAndWhitespaceInsensitive(t *testing.T) {
	apps := []application{
		{ID: "1", Role: "Remote Data Entry"},
		{ID: "2", Role: "Frontend Developer"},
		{ID: "3", Role: "  REMOTE   DATA ENTRY "},
	}
	blocks := []blockedRole{{Role: "remote data entry"}}
	filtered := filterBlockedApplications(apps, blocks)
	if len(filtered) != 1 || filtered[0].ID != "2" {
		t.Fatalf("unexpected filtered applications: %+v", filtered)
	}
}

func TestBlockedRoleCanBeAddedAndRemoved(t *testing.T) {
	blocks := updateBlockedItems(nil, "Remote Data Entry", "", "", true)
	blocks = updateBlockedItems(blocks, " remote  data entry ", "", "", true)
	if len(blocks) != 1 {
		t.Fatalf("expected one unique block, got %+v", blocks)
	}
	blocks = updateBlockedItems(blocks, "REMOTE DATA ENTRY", "", "", false)
	if len(blocks) != 0 {
		t.Fatalf("expected block to be removed, got %+v", blocks)
	}
}

func TestUnknownRoleCanBeIgnoredByMessageID(t *testing.T) {
	apps := []application{{ID: "gmail-1", Role: "Role not detected"}, {ID: "gmail-2", Role: "Role not detected"}}
	blocks := updateBlockedItems(nil, "", "gmail-1", "Suspicious application email", true)
	filtered := filterBlockedApplications(apps, blocks)
	if len(filtered) != 1 || filtered[0].ID != "gmail-2" {
		t.Fatalf("unexpected filtered applications: %+v", filtered)
	}
	blocks = updateBlockedItems(blocks, "", "gmail-1", "Suspicious application email", false)
	if len(blocks) != 0 {
		t.Fatalf("expected message block to be removable, got %+v", blocks)
	}
}
