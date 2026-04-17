package actions

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BatchExecutor handles execution of actions across multiple targets
type BatchExecutor struct {
	executor *ActionExecutor
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor() (*BatchExecutor, error) {
	executor, err := NewActionExecutor()
	if err != nil {
		return nil, err
	}

	return &BatchExecutor{
		executor: executor,
	}, nil
}

// ExecuteBatch executes an action across multiple targets
func (b *BatchExecutor) ExecuteBatch(ctx context.Context, batch *BatchAction, progressChan chan<- *ActionProgress) (*BatchResult, error) {
	result := &BatchResult{
		BatchID:      batch.ID,
		TotalTargets: len(batch.Targets),
		Results:      make(map[string]*ActionResult),
		StartTime:    time.Now(),
	}

	// Set default max parallel if not specified
	maxParallel := batch.Options.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 10 // Default to 10 concurrent executions
	}

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Send initial progress
	if progressChan != nil {
		progressChan <- &ActionProgress{
			ActionID:  batch.ID,
			Status:    "in_progress",
			Progress:  0,
			Message:   fmt.Sprintf("Starting batch operation on %d targets", len(batch.Targets)),
			Timestamp: time.Now(),
		}
	}

	// Execute on each target
	for i, target := range batch.Targets {
		wg.Add(1)
		go func(idx int, tgt ActionTarget) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create individual action
			action := &Action{
				ID:          fmt.Sprintf("%s-%d", batch.ID, idx),
				Type:        batch.Type,
				Target:      tgt,
				Parameters:  batch.Parameters,
				RequestedBy: batch.RequestedBy,
				RequestedAt: batch.RequestedAt,
			}

			// Send progress update
			if progressChan != nil {
				progressChan <- &ActionProgress{
					ActionID:  action.ID,
					Status:    "in_progress",
					Progress:  0,
					Message:   fmt.Sprintf("Executing on %s", tgt.EntityID),
					Timestamp: time.Now(),
				}
			}

			// Execute the action
			actionResult, err := b.executor.ExecuteAction(ctx, action)
			if err != nil && actionResult == nil {
				actionResult = &ActionResult{
					Success:   false,
					Message:   "Execution failed",
					Error:     err.Error(),
					StartTime: time.Now(),
					EndTime:   time.Now(),
				}
			}

			// Store result
			mu.Lock()
			targetKey := fmt.Sprintf("%s/%s", tgt.Namespace, tgt.EntityID)
			result.Results[targetKey] = actionResult
			if actionResult.Success {
				result.Successful++
			} else {
				result.Failed++
			}

			// Calculate progress
			completed := result.Successful + result.Failed
			progress := int((float64(completed) / float64(result.TotalTargets)) * 100)

			mu.Unlock()

			// Send progress update
			if progressChan != nil {
				status := "success"
				if !actionResult.Success {
					status = "failed"
				}

				progressChan <- &ActionProgress{
					ActionID:  action.ID,
					Status:    status,
					Progress:  progress,
					Message:   actionResult.Message,
					Timestamp: time.Now(),
				}
			}

			// Stop on first failure if configured
			if !actionResult.Success && batch.Options.StopOnFirstFailure {
				// Cancel context to stop other operations
				// Note: This requires the context to be cancellable
			}
		}(i, target)
	}

	// Wait for all operations to complete
	wg.Wait()
	result.EndTime = time.Now()

	// Send final progress
	if progressChan != nil {
		finalStatus := "success"
		if result.Failed > 0 {
			finalStatus = "partial_success"
			if result.Successful == 0 {
				finalStatus = "failed"
			}
		}

		progressChan <- &ActionProgress{
			ActionID:  batch.ID,
			Status:    finalStatus,
			Progress:  100,
			Message:   fmt.Sprintf("Completed: %d successful, %d failed", result.Successful, result.Failed),
			Timestamp: time.Now(),
		}
	}

	return result, nil
}

// ExecuteBatchUpdateImage is a convenience method for updating images across multiple deployments
func (b *BatchExecutor) ExecuteBatchUpdateImage(ctx context.Context, targets []ActionTarget, newImage string, options BatchOptions, progressChan chan<- *ActionProgress) (*BatchResult, error) {
	batch := &BatchAction{
		ID:      fmt.Sprintf("batch-%d", time.Now().Unix()),
		Type:    ActionBatchUpdateImage,
		Targets: targets,
		Parameters: map[string]string{
			"image": newImage,
		},
		Options:     options,
		RequestedBy: "system",
		RequestedAt: time.Now(),
	}

	return b.ExecuteBatch(ctx, batch, progressChan)
}

// ValidateBatchAction validates a batch action before execution
func (b *BatchExecutor) ValidateBatchAction(batch *BatchAction) error {
	if len(batch.Targets) == 0 {
		return fmt.Errorf("no targets specified")
	}

	if batch.Type == "" {
		return fmt.Errorf("action type is required")
	}

	// Validate each target
	for i, target := range batch.Targets {
		if target.Layer == "" {
			return fmt.Errorf("target %d: layer is required", i)
		}
		if target.EntityID == "" {
			return fmt.Errorf("target %d: entity ID is required", i)
		}
	}

	return nil
}

// DryRunBatch performs a dry run of a batch action
func (b *BatchExecutor) DryRunBatch(ctx context.Context, batch *BatchAction) ([]string, error) {
	if err := b.ValidateBatchAction(batch); err != nil {
		return nil, err
	}

	var messages []string
	for _, target := range batch.Targets {
		msg := fmt.Sprintf("Would execute %s on %s/%s", batch.Type, target.Namespace, target.EntityID)
		messages = append(messages, msg)
	}

	return messages, nil
}
