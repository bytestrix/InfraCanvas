package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressIndicator(t *testing.T) {
	var buf bytes.Buffer
	indicator := NewProgressIndicator("Testing...")
	indicator.writer = &buf

	indicator.Start()
	time.Sleep(250 * time.Millisecond) // Let it spin a few times
	indicator.Stop()

	output := buf.String()
	if !strings.Contains(output, "Testing...") {
		t.Error("Progress indicator should display the message")
	}
}

func TestProgressIndicatorUpdateMessage(t *testing.T) {
	indicator := NewProgressIndicator("Initial message")

	indicator.Start()
	time.Sleep(100 * time.Millisecond)
	
	indicator.UpdateMessage("Updated message")
	
	// Verify the message was updated
	indicator.mu.Lock()
	message := indicator.message
	indicator.mu.Unlock()
	
	indicator.Stop()

	if message != "Updated message" {
		t.Errorf("Progress indicator message = %q, want %q", message, "Updated message")
	}
}

func TestProgressTracker(t *testing.T) {
	var buf bytes.Buffer
	stages := []string{"Stage 1", "Stage 2", "Stage 3"}
	tracker := NewProgressTracker(stages)
	tracker.writer = &buf

	tracker.Start()
	time.Sleep(100 * time.Millisecond)

	tracker.NextStage()
	time.Sleep(100 * time.Millisecond)

	tracker.NextStage()
	time.Sleep(100 * time.Millisecond)

	tracker.Complete()

	output := buf.String()
	if !strings.Contains(output, "Completed") {
		t.Error("Progress tracker should display completion message")
	}
}

func TestProgressTrackerFail(t *testing.T) {
	var buf bytes.Buffer
	stages := []string{"Stage 1"}
	tracker := NewProgressTracker(stages)
	tracker.writer = &buf

	tracker.Start()
	time.Sleep(100 * time.Millisecond)

	tracker.Fail(nil)

	output := buf.String()
	if !strings.Contains(output, "Failed") {
		t.Error("Progress tracker should display failure message")
	}
}

func TestSimpleProgress(t *testing.T) {
	var buf bytes.Buffer
	progress := NewSimpleProgress()
	progress.writer = &buf

	progress.Show("Processing...")
	progress.Success("Done!")
	progress.Error("Failed!")
	progress.Warning("Warning!")

	output := buf.String()
	if !strings.Contains(output, "Processing...") {
		t.Error("SimpleProgress should display show message")
	}
	if !strings.Contains(output, "Done!") {
		t.Error("SimpleProgress should display success message")
	}
	if !strings.Contains(output, "Failed!") {
		t.Error("SimpleProgress should display error message")
	}
	if !strings.Contains(output, "Warning!") {
		t.Error("SimpleProgress should display warning message")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		contains string
	}{
		{500 * time.Millisecond, "ms"},
		{2 * time.Second, "s"},
		{90 * time.Second, "m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("formatDuration(%v) = %s, should contain %s", tt.duration, result, tt.contains)
		}
	}
}

func TestClearLine(t *testing.T) {
	result := clearLine()
	if result != "\033[2K" {
		t.Errorf("clearLine() = %q, want %q", result, "\033[2K")
	}
}

func TestProgressIndicatorDoubleStart(t *testing.T) {
	var buf bytes.Buffer
	indicator := NewProgressIndicator("Testing...")
	indicator.writer = &buf

	indicator.Start()
	indicator.Start() // Should not panic or cause issues
	time.Sleep(100 * time.Millisecond)
	indicator.Stop()

	// Should complete without error
}

func TestProgressIndicatorDoubleStop(t *testing.T) {
	var buf bytes.Buffer
	indicator := NewProgressIndicator("Testing...")
	indicator.writer = &buf

	indicator.Start()
	time.Sleep(100 * time.Millisecond)
	indicator.Stop()
	indicator.Stop() // Should not panic or cause issues

	// Should complete without error
}

func TestProgressTrackerEmptyStages(t *testing.T) {
	var buf bytes.Buffer
	tracker := NewProgressTracker([]string{})
	tracker.writer = &buf

	tracker.Start()
	tracker.Complete()

	// Should complete without error even with no stages
}

func TestProgressTrackerNextStageBeyondEnd(t *testing.T) {
	var buf bytes.Buffer
	stages := []string{"Stage 1"}
	tracker := NewProgressTracker(stages)
	tracker.writer = &buf

	tracker.Start()
	tracker.NextStage()
	tracker.NextStage() // Beyond the end
	tracker.Complete()

	// Should complete without error
}
