package services

import (
	"capyflow/api/models"
	"capyflow/api/websocket"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CronService periodically executes flows whose trigger node is of type
// "cron-trigger". The actual interval is defined per-node via the
// intervalMinutes parameter.
type CronService struct {
	db       *gorm.DB
	executor *ExecutorService
	// lastRun keeps track of the last time each flow was executed.
	lastRun map[string]time.Time
	stopCh  chan struct{}
}

// NewCronService creates a new scheduler service instance.
func NewCronService(db *gorm.DB, hub *websocket.Hub) *CronService {
	return &CronService{
		db:       db,
		executor: NewExecutorService(db, hub),
		lastRun:  make(map[string]time.Time),
		stopCh:   make(chan struct{}),
	}
}

// Start runs the scheduler loop in the current goroutine.
// It should typically be started in its own goroutine.
func (s *CronService) Start() {
	log.Println("[cron] CronService started")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.runTick(); err != nil {
				log.Printf("[cron] tick error: %v", err)
			}
		case <-s.stopCh:
			log.Println("[cron] CronService stopped")
			return
		}
	}
}

// Stop signals the scheduler loop to stop.
func (s *CronService) Stop() {
	close(s.stopCh)
}

// runTick finds all active flows with a cron-trigger node and executes
// those whose interval has elapsed since the last run.
func (s *CronService) runTick() error {
	var flows []models.Flow
	if err := s.db.Preload("Nodes").Preload("Edges").
		Where("status = ?", models.FlowStatusActive).
		Find(&flows).Error; err != nil {
		return err
	}

	now := time.Now()

	for i := range flows {
		flow := flows[i]

		cronNode, intervalMinutes, ok := findCronTrigger(&flow)
		if !ok || cronNode == nil {
			continue
		}

		// Default interval safeguard
		if intervalMinutes <= 0 {
			intervalMinutes = 60
		}

		last, hasLast := s.lastRun[flow.ID]
		if hasLast {
			if now.Sub(last) < time.Duration(intervalMinutes)*time.Minute {
				// Not yet time to run again
				continue
			}
		}

		// Record next run time before executing to avoid double-runs
		s.lastRun[flow.ID] = now

		go s.executeCronFlow(&flow, intervalMinutes)
	}

	return nil
}

// findCronTrigger locates the cron-trigger node in the flow (if any) and
// extracts its configured interval in minutes.
func findCronTrigger(flow *models.Flow) (*models.Node, int, bool) {
	for i := range flow.Nodes {
		n := &flow.Nodes[i]
		if n.Type != "cron-trigger" {
			continue
		}

		// Default interval
		interval := 60
		if n.Parameters != "" {
			var params struct {
				IntervalMinutes int `json:"intervalMinutes"`
			}
			if err := json.Unmarshal([]byte(n.Parameters), &params); err == nil {
				if params.IntervalMinutes > 0 {
					interval = params.IntervalMinutes
				}
			}
		}

		return n, interval, true
	}

	return nil, 0, false
}

// executeCronFlow creates an execution record and runs the flow through the
// existing ExecutorService, marking the trigger type as "cron".
func (s *CronService) executeCronFlow(flow *models.Flow, intervalMinutes int) {
	userID := flow.UserID
	executionID := uuid.New().String()
	now := time.Now()

	execution := models.Execution{
		ID:          executionID,
		FlowID:      flow.ID,
		UserID:      userID,
		Status:      models.ExecutionStatusRunning,
		StartedAt:   now,
		TriggerType: "cron",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.db.Create(&execution).Error; err != nil {
		log.Printf("[cron] failed to create execution record for flow %s: %v", flow.ID, err)
		return
	}

	// Provide a minimal initial context for the flow
	initialContext := map[string]map[string]interface{}{
		"cron": {
			"timestamp":       now.Unix(),
			"intervalMinutes": intervalMinutes,
		},
	}

	result := s.executor.ExecuteFlowWithContext(flow, initialContext, userID, executionID)
	finish := time.Now()

	resultsJSON, _ := json.Marshal(result.Results)
	executedNodesJSON, _ := json.Marshal(result.ExecutedNodes)

	execution.Status = models.ExecutionStatus(result.Status)
	execution.FinishedAt = &finish
	execution.DurationMs = result.DurationMs
	execution.Results = string(resultsJSON)
	execution.ExecutedNodes = string(executedNodesJSON)
	execution.ErrorMessage = result.ErrorMessage
	execution.UpdatedAt = finish

	if err := s.db.Save(&execution).Error; err != nil {
		log.Printf("[cron] failed to update execution record %s: %v", executionID, err)
	}
}
