package feishu

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConfigStore struct {
	pool *pgxpool.Pool
}

func NewConfigStore(pool *pgxpool.Pool) *ConfigStore {
	return &ConfigStore{pool: pool}
}

func (s *ConfigStore) GetByUserAndWorkspace(ctx context.Context, userID, workspaceID pgtype.UUID) (*FeishuUserConfig, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, workspace_id, app_id, app_secret_encrypted, data_source,
			   bitable_id, title_field, assignee_field, content_fields,
			   target_type, target_project_id, sync_interval_minutes,
			   last_sync_at, enabled, filter_config, tasks_filter_config, created_at, updated_at
		FROM feishu_user_configs
		WHERE user_id = $1 AND workspace_id = $2
	`, userID, workspaceID)

	var cfg FeishuUserConfig
	var contentFieldsJSON []byte
	var filterConfigJSON []byte
	var tasksFilterConfigJSON []byte
	err := row.Scan(
		&cfg.ID, &cfg.UserID, &cfg.WorkspaceID, &cfg.AppID, &cfg.AppSecretEncrypted,
		&cfg.DataSource, &cfg.BitableID, &cfg.TitleField, &cfg.AssigneeField,
		&contentFieldsJSON, &cfg.TargetType, &cfg.TargetProjectID,
		&cfg.SyncIntervalMinutes, &cfg.LastSyncAt, &cfg.Enabled,
		&filterConfigJSON, &tasksFilterConfigJSON,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(contentFieldsJSON, &cfg.ContentFields)
	cfg.FilterConfig = filterConfigJSON
	cfg.TasksFilterConfig = tasksFilterConfigJSON
	return &cfg, nil
}

func (s *ConfigStore) Upsert(ctx context.Context, cfg *FeishuUserConfig) error {
	contentFieldsJSON, _ := json.Marshal(cfg.ContentFields)
	filterConfigJSON := cfg.FilterConfig
	tasksFilterConfigJSON := cfg.TasksFilterConfig

	id := cfg.ID
	if !id.Valid {
		id = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO feishu_user_configs
			(id, user_id, workspace_id, app_id, app_secret_encrypted, data_source,
			 bitable_id, title_field, assignee_field, content_fields,
			 target_type, target_project_id, sync_interval_minutes, enabled, filter_config, tasks_filter_config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (user_id, workspace_id) DO UPDATE SET
			app_id = EXCLUDED.app_id,
			app_secret_encrypted = EXCLUDED.app_secret_encrypted,
			data_source = EXCLUDED.data_source,
			bitable_id = EXCLUDED.bitable_id,
			title_field = EXCLUDED.title_field,
			assignee_field = EXCLUDED.assignee_field,
			content_fields = EXCLUDED.content_fields,
			target_type = EXCLUDED.target_type,
			target_project_id = EXCLUDED.target_project_id,
			sync_interval_minutes = EXCLUDED.sync_interval_minutes,
			enabled = EXCLUDED.enabled,
			filter_config = EXCLUDED.filter_config,
			tasks_filter_config = EXCLUDED.tasks_filter_config,
			updated_at = now()
	`, id, cfg.UserID, cfg.WorkspaceID, cfg.AppID, cfg.AppSecretEncrypted,
		cfg.DataSource, cfg.BitableID, cfg.TitleField, cfg.AssigneeField,
		contentFieldsJSON, cfg.TargetType, cfg.TargetProjectID,
		cfg.SyncIntervalMinutes, cfg.Enabled, filterConfigJSON, tasksFilterConfigJSON)
	return err
}

func (s *ConfigStore) Delete(ctx context.Context, userID, workspaceID pgtype.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM feishu_user_configs WHERE user_id = $1 AND workspace_id = $2
	`, userID, workspaceID)
	return err
}

func (s *ConfigStore) UpdateLastSync(ctx context.Context, userID, workspaceID pgtype.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE feishu_user_configs SET last_sync_at = $3, updated_at = now()
		WHERE user_id = $1 AND workspace_id = $2
	`, userID, workspaceID, time.Now())
	return err
}

func (s *ConfigStore) GetMapping(ctx context.Context, userID, workspaceID pgtype.UUID, sourceType, feishuRecordID string) (*FeishuTaskMapping, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, workspace_id, feishu_record_id, feishu_task_id,
			   source_type, multica_issue_id, created_at, updated_at
		FROM feishu_task_mappings
		WHERE user_id = $1 AND workspace_id = $2 AND source_type = $3 AND feishu_record_id = $4
	`, userID, workspaceID, sourceType, feishuRecordID)

	var m FeishuTaskMapping
	err := row.Scan(&m.ID, &m.UserID, &m.WorkspaceID, &m.FeishuRecordID,
		&m.FeishuTaskID, &m.SourceType, &m.MulticaIssueID, &m.CreatedAt, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *ConfigStore) UpsertMapping(ctx context.Context, m *FeishuTaskMapping) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO feishu_task_mappings
			(id, user_id, workspace_id, feishu_record_id, feishu_task_id, source_type, multica_issue_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, workspace_id, source_type, feishu_record_id) DO UPDATE SET
			feishu_task_id = EXCLUDED.feishu_task_id,
			multica_issue_id = EXCLUDED.multica_issue_id,
			updated_at = now()
	`, m.ID, m.UserID, m.WorkspaceID, m.FeishuRecordID, m.FeishuTaskID, m.SourceType, m.MulticaIssueID)
	return err
}
