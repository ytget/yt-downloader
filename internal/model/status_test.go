package model

import "testing"

func TestTaskStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected bool
	}{
		{TaskStatusPending, false},
		{TaskStatusStarting, true},
		{TaskStatusDownloading, true},
		{TaskStatusStopping, true},
		{TaskStatusStopped, false},
		{TaskStatusCompleted, false},
		{TaskStatusError, false},
	}

	for _, test := range tests {
		result := test.status.IsActive()
		if result != test.expected {
			t.Errorf("TaskStatus(%s).IsActive() = %v, expected %v", test.status, result, test.expected)
		}
	}
}

func TestTaskStatus_IsFinished(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected bool
	}{
		{TaskStatusPending, false},
		{TaskStatusStarting, false},
		{TaskStatusDownloading, false},
		{TaskStatusStopping, false},
		{TaskStatusStopped, true},
		{TaskStatusCompleted, true},
		{TaskStatusError, true},
	}

	for _, test := range tests {
		result := test.status.IsFinished()
		if result != test.expected {
			t.Errorf("TaskStatus(%s).IsFinished() = %v, expected %v", test.status, result, test.expected)
		}
	}
}

func TestTaskStatus_String(t *testing.T) {
	status := TaskStatusDownloading
	expected := "Downloading"
	result := status.String()

	if result != expected {
		t.Errorf("TaskStatus.String() = %s, expected %s", result, expected)
	}
}
