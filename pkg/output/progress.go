package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ProgressIndicator displays progress during long-running operations
type ProgressIndicator struct {
	writer  io.Writer
	message string
	spinner []string
	index   int
	done    chan bool
	mu      sync.Mutex
	active  bool
}

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator(message string) *ProgressIndicator {
	return &ProgressIndicator{
		writer:  os.Stderr,
		message: message,
		spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:    make(chan bool),
	}
}

// Start begins displaying the progress indicator
func (p *ProgressIndicator) Start() {
	p.mu.Lock()
	if p.active {
		p.mu.Unlock()
		return
	}
	p.active = true
	p.mu.Unlock()

	go p.spin()
}

// Stop stops the progress indicator
func (p *ProgressIndicator) Stop() {
	p.mu.Lock()
	if !p.active {
		p.mu.Unlock()
		return
	}
	p.active = false
	p.mu.Unlock()

	p.done <- true
	// Clear the line
	fmt.Fprintf(p.writer, "\r%s\r", clearLine())
}

// UpdateMessage updates the progress message
func (p *ProgressIndicator) UpdateMessage(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.message = message
}

// spin runs the spinner animation
func (p *ProgressIndicator) spin() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()
			frame := p.spinner[p.index%len(p.spinner)]
			message := p.message
			p.index++
			p.mu.Unlock()

			fmt.Fprintf(p.writer, "\r%s %s", frame, message)
		}
	}
}

// clearLine returns ANSI escape code to clear the current line
func clearLine() string {
	return "\033[2K"
}

// ProgressTracker tracks progress across multiple stages
type ProgressTracker struct {
	writer     io.Writer
	stages     []string
	current    int
	indicator  *ProgressIndicator
	mu         sync.Mutex
	startTime  time.Time
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(stages []string) *ProgressTracker {
	return &ProgressTracker{
		writer:    os.Stderr,
		stages:    stages,
		current:   0,
		startTime: time.Now(),
	}
}

// Start begins tracking progress
func (pt *ProgressTracker) Start() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if len(pt.stages) == 0 {
		return
	}

	message := fmt.Sprintf("[1/%d] %s", len(pt.stages), pt.stages[0])
	pt.indicator = NewProgressIndicator(message)
	pt.indicator.Start()
}

// NextStage moves to the next stage
func (pt *ProgressTracker) NextStage() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.indicator != nil {
		pt.indicator.Stop()
	}

	pt.current++
	if pt.current >= len(pt.stages) {
		return
	}

	message := fmt.Sprintf("[%d/%d] %s", pt.current+1, len(pt.stages), pt.stages[pt.current])
	pt.indicator = NewProgressIndicator(message)
	pt.indicator.Start()
}

// Complete marks all stages as complete
func (pt *ProgressTracker) Complete() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.indicator != nil {
		pt.indicator.Stop()
	}

	duration := time.Since(pt.startTime)
	fmt.Fprintf(pt.writer, "✓ Completed in %s\n", formatDuration(duration))
}

// Fail marks the current stage as failed
func (pt *ProgressTracker) Fail(err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.indicator != nil {
		pt.indicator.Stop()
	}

	if err != nil {
		fmt.Fprintf(pt.writer, "✗ Failed: %s\n", err.Error())
	} else {
		fmt.Fprintf(pt.writer, "✗ Failed\n")
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// SimpleProgress displays a simple progress message
type SimpleProgress struct {
	writer io.Writer
}

// NewSimpleProgress creates a new simple progress indicator
func NewSimpleProgress() *SimpleProgress {
	return &SimpleProgress{
		writer: os.Stderr,
	}
}

// Show displays a progress message
func (sp *SimpleProgress) Show(message string) {
	fmt.Fprintf(sp.writer, "→ %s\n", message)
}

// Success displays a success message
func (sp *SimpleProgress) Success(message string) {
	fmt.Fprintf(sp.writer, "✓ %s\n", message)
}

// Error displays an error message
func (sp *SimpleProgress) Error(message string) {
	fmt.Fprintf(sp.writer, "✗ %s\n", message)
}

// Warning displays a warning message
func (sp *SimpleProgress) Warning(message string) {
	fmt.Fprintf(sp.writer, "⚠ %s\n", message)
}
