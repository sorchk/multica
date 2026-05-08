package feishu

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	configStore  *ConfigStore
	syncService  *SyncService
	cron         *cron.Cron
	jobs         map[string]cron.EntryID
	mu           sync.RWMutex
}

func NewScheduler(configStore *ConfigStore, syncService *SyncService) *Scheduler {
	return &Scheduler{
		configStore: configStore,
		syncService: syncService,
		cron:        cron.New(),
		jobs:        make(map[string]cron.EntryID),
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) ScheduleJob(userID, workspaceID pgtype.UUID, intervalMinutes int) {
	key := fmt.Sprintf("%s:%s", userID, workspaceID)
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.jobs[key]; ok {
		s.cron.Remove(entryID)
	}

	spec := fmt.Sprintf("*/%d * * * *", intervalMinutes)
	entryID, _ := s.cron.AddFunc(spec, func() {
		s.syncService.SyncUserFeishuData(context.Background(), userID, workspaceID)
	})
	s.jobs[key] = entryID
}

func (s *Scheduler) RemoveJob(userID, workspaceID pgtype.UUID) {
	key := fmt.Sprintf("%s:%s", userID, workspaceID)
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.jobs[key]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, key)
	}
}
