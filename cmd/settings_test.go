package main

import (
	"testing"

	"github.com/abhinavxd/libredesk/internal/setting/models"
)

func TestValidRetentionPeriod(t *testing.T) {
	for days, want := range map[int]bool{0: true, 1: false, 7: false, 29: false, 30: true, 180: true, 3650: true, 3651: false, -1: false} {
		if got := validRetentionPeriod(days); got != want {
			t.Errorf("validRetentionPeriod(%d) = %v, want %v", days, got, want)
		}
	}
}

func TestRetentionRequested(t *testing.T) {
	if retentionRequested(models.General{}) {
		t.Fatal("empty settings request retention")
	}
	for name, req := range map[string]models.General{
		"days":     {ConversationRetentionDays: 30},
		"tagged":   {ConversationRetentionTaggedDays: 365},
		"tags":     {ConversationRetentionTags: []string{"payment"}},
		"contacts": {ConversationRetentionDeleteContacts: true},
	} {
		if !retentionRequested(req) {
			t.Errorf("%s: retention not detected", name)
		}
	}
}
