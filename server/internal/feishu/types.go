package feishu

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type FeishuUserConfig struct {
	ID                   pgtype.UUID  `json:"id"`
	UserID               pgtype.UUID  `json:"user_id"`
	WorkspaceID          pgtype.UUID  `json:"workspace_id"`
	AppID                string       `json:"app_id"`
	AppSecretEncrypted   string       `json:"app_secret_encrypted"`
	DataSource           string       `json:"data_source"`
	BitableID            *string      `json:"bitable_id"`
	TitleField           *string      `json:"title_field"`
	AssigneeField        *string      `json:"assignee_field"`
	ContentFields        []string     `json:"content_fields"`
	TargetType           string       `json:"target_type"`
	TargetProjectID      *pgtype.UUID `json:"target_project_id"`
	SyncIntervalMinutes  int          `json:"sync_interval_minutes"`
	LastSyncAt           *time.Time   `json:"last_sync_at"`
	Enabled              bool         `json:"enabled"`
	FilterConfig         json.RawMessage `json:"filter_config"`
	TasksFilterConfig   json.RawMessage `json:"tasks_filter_config"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
}

type FeishuTaskMapping struct {
	ID             pgtype.UUID  `json:"id"`
	UserID         pgtype.UUID  `json:"user_id"`
	WorkspaceID    pgtype.UUID  `json:"workspace_id"`
	FeishuRecordID string       `json:"feishu_record_id"`
	FeishuTaskID   *string      `json:"feishu_task_id"`
	SourceType     string       `json:"source_type"`
	MulticaIssueID pgtype.UUID  `json:"multica_issue_id"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type BitableRecord struct {
	RecordID  string                 `json:"record_id"`
	Fields    map[string]interface{} `json:"fields"`
	CreatedAt string                 `json:"created_time"`
	UpdatedAt string                 `json:"last_modified_time"`
}

type BitableRecordsResponse struct {
	Code int        `json:"code"`
	Msg  string     `json:"msg"`
	Data BitableData `json:"data"`
}

type BitableData struct {
	Items []BitableRecord `json:"items"`
	Page  BitablePage    `json:"page"`
}

type BitablePage struct {
	Total    int `json:"total"`
	PageSize int `json:"page_size"`
}

type BitableTable struct {
	TableID   string `json:"table_id"`
	Name      string `json:"name"`
	DateCreated string `json:"date_created"`
	DateModified string `json:"date_modified"`
}

type BitableTablesResponse struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items []BitableTable `json:"items"`
	} `json:"data"`
}

type FeishuTask struct {
	GUID        string            `json:"guid"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Due         *FeishuTaskDue    `json:"due"`
	Origin      FeishuTaskOrigin  `json:"origin"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type FeishuTaskDue struct {
	Timestamp string `json:"timestamp"`
	IsAllDay  bool   `json:"is_all_day"`
}

type FeishuTaskOrigin struct {
	PlatformType string `json:"platform_type"`
	TaskGUID     string `json:"task_guid"`
}

type FeishuTasksResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data FeishuTasksData `json:"data"`
}

type FeishuTasksData struct {
	Items     []FeishuTask `json:"items"`
	PageToken string       `json:"page_token"`
	HasMore   bool         `json:"has_more"`
}

type FeishuWebhookEvent struct {
	Schema string         `json:"schema"`
	Header WebhookHeader  `json:"header"`
	Event  interface{}    `json:"event"`
}

type WebhookHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token     string `json:"token"`
	AppID     string `json:"app_id"`
	TenantKey string `json:"tenant_key"`
}

type BitableWebhookEvent struct {
	Schema string          `json:"schema"`
	Header WebhookHeader   `json:"header"`
	Event  BitableEventData `json:"event"`
}

type BitableEventData struct {
	AppToken   string `json:"app_token"`
	TableID    string `json:"table_id"`
	RecordID   string `json:"record_id"`
	ChangeType string `json:"change_type"`
}

type TaskWebhookEvent struct {
	Schema string          `json:"schema"`
	Header WebhookHeader   `json:"header"`
	Event  TaskEventData   `json:"event"`
}

type TaskEventData struct {
	TaskGUID  string `json:"task_guid"`
	OpenID    string `json:"open_id"`
	ActorID   string `json:"actor_id"`
	EventType string `json:"event_type"`
}

type FilterCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type FilterGroup struct {
	Logic      string            `json:"logic"`
	Conditions []FilterCondition `json:"conditions"`
}
