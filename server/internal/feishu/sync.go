package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/pkg/db/generated"
)

type SyncService struct {
	configStore  *ConfigStore
	tokenManager *TokenManager
	queries      *db.Queries
}

func NewSyncService(configStore *ConfigStore, tokenManager *TokenManager, queries *db.Queries) *SyncService {
	return &SyncService{
		configStore:  configStore,
		tokenManager: tokenManager,
		queries:      queries,
	}
}

func (s *SyncService) SyncUserFeishuData(ctx context.Context, userID, workspaceID pgtype.UUID) error {
	cfg, err := s.configStore.GetByUserAndWorkspace(ctx, userID, workspaceID)
	if err != nil || cfg == nil || !cfg.Enabled {
		return fmt.Errorf("no config or disabled")
	}

	user, err := s.queries.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	userEmail := user.Email

	token, err := s.tokenManager.GetToken(ctx, cfg.AppID, cfg.AppSecretEncrypted)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if strings.Contains(cfg.DataSource, "bitable") && cfg.BitableID != nil {
		if err := s.syncBitable(ctx, cfg, token, userEmail); err != nil {
			slog.Error("bitable sync failed", "error", err)
		}
	}

	if strings.Contains(cfg.DataSource, "tasks") {
		if err := s.syncTasks(ctx, cfg, token, userEmail); err != nil {
			slog.Error("tasks sync failed", "error", err)
		}
	}

	s.configStore.UpdateLastSync(ctx, userID, workspaceID)
	return nil
}

func (s *SyncService) syncBitable(ctx context.Context, cfg *FeishuUserConfig, token, userEmail string) error {
	bitable := NewBitableClient(*cfg.BitableID, token)

	resp, err := bitable.GetRecords(ctx, 100, "")
	if err != nil {
		return fmt.Errorf("failed to fetch bitable records: %w", err)
	}

	for _, record := range resp.Data.Items {
		assignee := s.extractFieldValue(record.Fields, cfg.AssigneeField)
		if !s.isAssignedToUser(assignee, userEmail) {
			continue
		}

		title := s.extractFieldValue(record.Fields, cfg.TitleField)
		content := s.extractContentFields(record.Fields, cfg.ContentFields)

		issueID, err := s.createOrUpdateIssue(ctx, cfg, record.RecordID, title, content)
		if err != nil {
			slog.Error("failed to sync record", "record_id", record.RecordID, "error", err)
			continue
		}

		mapping := &FeishuTaskMapping{
			ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
			UserID:         cfg.UserID,
			WorkspaceID:    cfg.WorkspaceID,
			FeishuRecordID: record.RecordID,
			SourceType:     "bitable",
			MulticaIssueID: issueID,
		}
		s.configStore.UpsertMapping(ctx, mapping)
	}

	return nil
}

func (s *SyncService) syncTasks(ctx context.Context, cfg *FeishuUserConfig, token, userEmail string) error {
	tasksClient := NewTasksClient(token)

	resp, err := tasksClient.GetTasks(ctx, 100, "")
	if err != nil {
		return fmt.Errorf("failed to fetch tasks: %w", err)
	}

	for _, task := range resp.Data.Items {
		issueID, err := s.createOrUpdateIssue(ctx, cfg, task.GUID, task.Summary, task.Description)
		if err != nil {
			slog.Error("failed to sync task", "task_guid", task.GUID, "error", err)
			continue
		}

		mapping := &FeishuTaskMapping{
			ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
			UserID:         cfg.UserID,
			WorkspaceID:    cfg.WorkspaceID,
			FeishuRecordID: task.GUID,
			FeishuTaskID:   &task.GUID,
			SourceType:     "tasks",
			MulticaIssueID: issueID,
		}
		s.configStore.UpsertMapping(ctx, mapping)
	}

	return nil
}

func (s *SyncService) createOrUpdateIssue(ctx context.Context, cfg *FeishuUserConfig, recordID, title, content string) (pgtype.UUID, error) {
	mapping, err := s.configStore.GetMapping(ctx, cfg.UserID, cfg.WorkspaceID, "bitable", recordID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	if mapping != nil {
		_, err = s.queries.UpdateIssue(ctx, db.UpdateIssueParams{
			ID:          mapping.MulticaIssueID,
			Title:       pgtype.Text{String: title, Valid: title != ""},
			Description: pgtype.Text{String: content, Valid: content != ""},
		})
		return mapping.MulticaIssueID, err
	}

	var projectID pgtype.UUID
	if cfg.TargetType == "project" && cfg.TargetProjectID != nil {
		projectID = *cfg.TargetProjectID
	}

	issue, err := s.queries.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID: cfg.WorkspaceID,
		Title:       title,
		Description: pgtype.Text{String: content, Valid: content != ""},
		CreatorType: "member",
		CreatorID:   cfg.UserID,
		Status:      "todo",
		ProjectID:   projectID,
	})
	if err != nil {
		return pgtype.UUID{}, err
	}

	return issue.ID, nil
}

func (s *SyncService) extractFieldValue(fields map[string]interface{}, fieldName *string) string {
	if fieldName == nil {
		return ""
	}
	if val, ok := fields[*fieldName]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (s *SyncService) extractContentFields(fields map[string]interface{}, contentFields []string) string {
	var parts []string
	for _, field := range contentFields {
		if val, ok := fields[field]; ok {
			if str, ok := val.(string); ok {
				parts = append(parts, str)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func (s *SyncService) isAssignedToUser(assignee, userEmail string) bool {
	return strings.Contains(assignee, userEmail)
}
