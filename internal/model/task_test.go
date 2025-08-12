package model

import (
	"testing"
	"time"
)

func TestDownloadTask_GetETAString(t *testing.T) {
	tests := []struct {
		etaSec   int
		expected string
	}{
		{-1, "—"},
		{0, "—"},
		{30, "00:30"},
		{90, "01:30"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
		{7323, "02:02:03"},
	}

	for _, test := range tests {
		task := &DownloadTask{ETASec: test.etaSec}
		result := task.GetETAString()
		if result != test.expected {
			t.Errorf("GetETAString() with ETASec=%d = %s, expected %s", test.etaSec, result, test.expected)
		}
	}
}

func TestDownloadTask_GetDisplayTitle(t *testing.T) {
	tests := []struct {
		title    string
		url      string
		expected string
	}{
		{"Video Title", "https://youtube.com/watch?v=123", "Video Title"},
		{"", "https://youtube.com/watch?v=123", "https://youtube.com/watch?v=123"},
		{"Another Title", "https://youtube.com/watch?v=456", "Another Title"},
	}

	for _, test := range tests {
		task := &DownloadTask{
			Title: test.title,
			URL:   test.url,
		}
		result := task.GetDisplayTitle()
		if result != test.expected {
			t.Errorf("GetDisplayTitle() with title='%s', url='%s' = '%s', expected '%s'",
				test.title, test.url, result, test.expected)
		}
	}
}

func TestDownloadTask_Creation(t *testing.T) {
	now := time.Now()
	task := &DownloadTask{
		ID:        "test-123",
		URL:       "https://youtube.com/watch?v=test",
		Status:    TaskStatusPending,
		Progress:  0.0,
		Percent:   0,
		Speed:     "",
		ETASec:    -1,
		StartedAt: now,
	}

	if task.ID != "test-123" {
		t.Errorf("Expected ID to be 'test-123', got '%s'", task.ID)
	}

	if task.Status != TaskStatusPending {
		t.Errorf("Expected status to be TaskStatusPending, got %s", task.Status)
	}

	if !task.StartedAt.Equal(now) {
		t.Errorf("Expected StartedAt to be %v, got %v", now, task.StartedAt)
	}
}
