# 飞书任务同步实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将飞书多维表格和飞书任务中分配给用户的任务同步创建为 Multica Issue，支持 Webhook、定时同步和手动触发三种方式。

**Architecture:** 在 Go 后端 `server/internal/feishu/` 创建独立模块处理飞书 API 调用、Webhook 接收和同步逻辑；前端在 `packages/views/settings/` 添加飞书配置 Tab。

**Tech Stack:** Go (Chi router), PostgreSQL, React (shadcn/UI), TanStack Query

---

## 文件结构

### Backend (Go)
```
server/internal/feishu/
├── config.go              # 用户配置 CRUD（数据库操作）
├── types.go               # 飞书 API 响应类型定义
├── token.go               # tenant_access_token 管理（缓存+刷新）
├── bitable.go             # 飞书多维表格 API 客户端
├── tasks.go               # 飞书统一任务视图 API 客户端
├── sync.go                # 同步核心逻辑
├── scheduler.go           # 定时同步调度器
├── webhook.go             # Webhook 端点 + 事件处理

server/internal/handler/feishu.go    # 新建，飞书相关 HTTP handler
server/migrations/070_feishu_tables.up.sql
server/migrations/070_feishu_tables.down.sql
server/cmd/server/router.go         # 修改，注册飞书路由
```

### Frontend (TypeScript)
```
packages/views/settings/components/
├── feishu-tab.tsx         # 新建，飞书配置 Tab 组件
packages/core/types/index.ts  # 修改，添加飞书相关类型
packages/core/api/client.ts  # 修改，添加飞书相关 API 方法
```

---

## Task 1: 数据库迁移

**Files:**
- Create: `server/migrations/070_feishu_tables.up.sql`
- Create: `server/migrations/070_feishu_tables.down.sql`

- [ ] **Step 1: 创建 feishu_user_configs 表**

```sql
CREATE TABLE feishu_user_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    app_id VARCHAR(255) NOT NULL,
    app_secret_encrypted VARCHAR(512) NOT NULL,
    data_source VARCHAR(50) NOT NULL DEFAULT 'bitable',
    bitable_id VARCHAR(255),
    title_field VARCHAR(255),
    assignee_field VARCHAR(255),
    content_fields JSONB DEFAULT '[]',
    target_type VARCHAR(50) NOT NULL DEFAULT 'personal',
    target_project_id UUID,
    sync_interval_minutes INT NOT NULL DEFAULT 15,
    last_sync_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, workspace_id)
);

CREATE INDEX idx_feishu_user_configs_user_ws ON feishu_user_configs(user_id, workspace_id);
```

- [ ] **Step 2: 创建 feishu_task_mappings 表**

```sql
CREATE TABLE feishu_task_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    feishu_record_id VARCHAR(255) NOT NULL,
    feishu_task_id VARCHAR(255),
    source_type VARCHAR(50) NOT NULL,
    multica_issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, workspace_id, source_type, feishu_record_id)
);

CREATE INDEX idx_feishu_task_mappings_user_ws ON feishu_task_mappings(user_id, workspace_id);
CREATE INDEX idx_feishu_task_mappings_issue ON feishu_task_mappings(multica_issue_id);
```

- [ ] **Step 3: 创建回滚迁移**

`server/migrations/070_feishu_tables.down.sql`:
```sql
DROP TABLE IF EXISTS feishu_task_mappings;
DROP TABLE IF EXISTS feishu_user_configs;
```

- [ ] **Step 4: 运行迁移验证**

```bash
cd server && go run ./cmd/server migrate up
```

---

## Task 2: 后端类型定义

**Files:**
- Create: `server/internal/feishu/types.go`

- [ ] **Step 1: 定义 FeishuUserConfig 结构**

```go
package feishu

import (
    "time"
    "github.com/jackc/pgx/v5/pgtype"
)

type FeishuUserConfig struct {
    ID                   pgtype.UUID `json:"id"`
    UserID               pgtype.UUID `json:"user_id"`
    WorkspaceID          pgtype.UUID `json:"workspace_id"`
    AppID                string      `json:"app_id"`
    AppSecretEncrypted   string      `json:"app_secret_encrypted"`
    DataSource           string      `json:"data_source"` // "bitable", "tasks", "both"
    BitableID            *string     `json:"bitable_id"`
    TitleField           *string     `json:"title_field"`
    AssigneeField        *string     `json:"assignee_field"`
    ContentFields        []string    `json:"content_fields"`
    TargetType           string      `json:"target_type"` // "project", "personal"
    TargetProjectID      *pgtype.UUID `json:"target_project_id"`
    SyncIntervalMinutes  int         `json:"sync_interval_minutes"`
    LastSyncAt           *time.Time  `json:"last_sync_at"`
    Enabled              bool        `json:"enabled"`
    CreatedAt            time.Time   `json:"created_at"`
    UpdatedAt            time.Time   `json:"updated_at"`
}
```

- [ ] **Step 2: 定义 FeishuTaskMapping 结构**

```go
type FeishuTaskMapping struct {
    ID              pgtype.UUID `json:"id"`
    UserID          pgtype.UUID `json:"user_id"`
    WorkspaceID     pgtype.UUID `json:"workspace_id"`
    FeishuRecordID  string      `json:"feishu_record_id"`
    FeishuTaskID    *string     `json:"feishu_task_id"`
    SourceType      string      `json:"source_type"` // "bitable", "tasks"
    MulticaIssueID  pgtype.UUID `json:"multica_issue_id"`
    CreatedAt       time.Time   `json:"created_at"`
    UpdatedAt       time.Time   `json:"updated_at"`
}
```

- [ ] **Step 3: 定义飞书 API 响应类型**

```go
// Bitable API types
type BitableRecord struct {
    RecordID  string                 `json:"record_id"`
    Fields    map[string]interface{} `json:"fields"`
    CreatedAt string                 `json:"created_time"`
    UpdatedAt string                 `json:"last_modified_time"`
}

type BitableRecordsResponse struct {
    Code   int             `json:"code"`
    Msg    string          `json:"msg"`
    Data   BitableData     `json:"data"`
}

type BitableData struct {
    Items []BitableRecord `json:"items"`
    Page  BitablePage     `json:"page"`
}

type BitablePage struct {
    Total    int  `json:"total"`
    PageSize int  `json:"page_size"`
}

// Tasks API types
type FeishuTask struct {
    GUID       string    `json:"guid"`
    Summary    string    `json:"summary"`
    Description string   `json:"description"`
    Due        *FeishuTaskDue `json:"due"`
    Origin     FeishuTaskOrigin `json:"origin"`
    CreatedAt  string    `json:"created_at"`
    UpdatedAt  string    `json:"updated_at"`
}

type FeishuTasksResponse struct {
    Code    int         `json:"code"`
    Msg     string      `json:"msg"`
    Data    FeishuTasksData `json:"data"`
}

type FeishuTasksData struct {
    Items           []FeishuTask `json:"items"`
    PageToken       string       `json:"page_token"`
    HasMore         bool         `json:"has_more"`
}

// Webhook event types
type FeishuWebhookEvent struct {
    Schema string `json:"schema"`
    Header WebhookHeader `json:"header"`
    Event  interface{} `json:"event"`
}

type WebhookHeader struct {
    EventID   string `json:"event_id"`
    EventType string `json:"event_type"`
    CreateTime string `json:"create_time"`
    Token      string `json:"token"`
    AppID      string `json:"app_id"`
    TenantKey  string `json:"tenant_key"`
}

type BitableWebhookEvent struct {
    Schema    string `json:"schema"`
    Header    WebhookHeader `json:"header"`
    Event     BitableEventData `json:"event"`
}

type BitableEventData struct {
    AppToken   string `json:"app_token"`
    TableID    string `json:"table_id"`
    RecordID   string `json:"record_id"`
    ChangeType string `json:"change_type"` // "add_record", "update_record", "delete_record"
}

type TaskWebhookEvent struct {
    Schema string `json:"schema"`
    Header WebhookHeader `json:"header"`
    Event  TaskEventData `json:"event"`
}

type TaskEventData struct {
    TaskGUID string `json:"task_guid"`
    OpenID   string `json:"open_id"`
    ActorID  string `json:"actor_id"`
    EventType string `json:"event_type"` // "task.created", "task.updated", "task.deleted"
}
```

---

## Task 3: Token 管理

**Files:**
- Create: `server/internal/feishu/token.go`

- [ ] **Step 1: 实现 TokenManager 结构**

```go
package feishu

import (
    "context"
    "sync"
    "time"
    "encoding/json"
    "net/http"
    "fmt"
)

type TokenManager struct {
    pool     *pgxpool.Pool
    cache    map[string]*cachedToken
    mu       sync.RWMutex
}

type cachedToken struct {
    token     string
    expiresAt time.Time
}

func NewTokenManager(pool *pgxpool.Pool) *TokenManager {
    return &TokenManager{pool: pool, cache: make(map[string]*cachedToken)}
}

func (tm *TokenManager) GetToken(ctx context.Context, appID, appSecret string) (string, error) {
    tm.mu.RLock()
    if cached, ok := tm.cache[appID]; ok && time.Now().Before(cached.expiresAt.Add(-time.Minute)) {
        tm.mu.RUnlock()
        return cached.token, nil
    }
    tm.mu.RUnlock()

    token, expiresAt, err := tm.fetchToken(ctx, appID, appSecret)
    if err != nil {
        return "", err
    }

    tm.mu.Lock()
    tm.cache[appID] = &cachedToken{token: token, expiresAt: expiresAt}
    tm.mu.Unlock()

    return token, nil
}

func (tm *TokenManager) fetchToken(ctx context.Context, appID, appSecret string) (string, time.Time, error) {
    url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
    body := map[string]string{"app_id": appID, "app_secret": appSecret}
    bodyBytes, _ := json.Marshal(body)

    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", time.Time{}, err
    }
    defer resp.Body.Close()

    var result struct {
        Code              int    `json:"code"`
        Msg               string `json:"msg"`
        TenantAccessToken string `json:"tenant_access_token"`
        Expire            int    `json:"expire"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    if result.Code != 0 {
        return "", time.Time{}, fmt.Errorf("feishu token error: %s", result.Msg)
    }

    expiresAt := time.Now().Add(time.Duration(result.Expire) * time.Second)
    return result.TenantAccessToken, expiresAt, nil
}
```

---

## Task 4: Bitable API 客户端

**Files:**
- Create: `server/internal/feishu/bitable.go`

- [ ] **Step 1: 实现 BitableClient 结构**

```go
package feishu

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type BitableClient struct {
    appToken  string
    token     string
    httpClient *http.Client
}

func NewBitableClient(appToken, token string) *BitableClient {
    return &BitableClient{
        appToken:   appToken,
        token:      token,
        httpClient: http.DefaultClient,
    }
}

func (bc *BitableClient) GetRecords(ctx context.Context, pageSize int, pageToken string) (*BitableRecordsResponse, error) {
    url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/records?page_size=%d", bc.appToken, pageSize)
    if pageToken != "" {
        url += "&page_token=" + pageToken
    }

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+bc.token)

    resp, err := bc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result BitableRecordsResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return &result, nil
}

func (bc *BitableClient) GetFields(ctx context.Context) ([]BitableField, error) {
    url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/default/fields", bc.appToken)

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+bc.token)

    resp, err := bc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Code int `json:"code"`
        Msg  string `json:"msg"`
        Data struct {
            Items []BitableField `json:"items"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Data.Items, nil
}

type BitableField struct {
    FieldID   string `json:"field_id"`
    FieldName string `json:"field_name"`
    Type      int    `json:"type"`
}
```

---

## Task 5: Tasks API 客户端

**Files:**
- Create: `server/internal/feishu/tasks.go`

- [ ] **Step 1: 实现 TasksClient 结构**

```go
package feishu

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type TasksClient struct {
    token     string
    httpClient *http.Client
}

func NewTasksClient(token string) *TasksClient {
    return &TasksClient{
        token:      token,
        httpClient: http.DefaultClient,
    }
}

func (tc *TasksClient) GetTasks(ctx context.Context, pageSize int, pageToken string) (*FeishuTasksResponse, error) {
    url := fmt.Sprintf("https://open.feishu.cn/open-apis/tasks/v1/tasks?page_size=%d", pageSize)
    if pageToken != "" {
        url += "&page_token=" + pageToken
    }

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+tc.token)

    resp, err := tc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result FeishuTasksResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return &result, nil
}
```

---

## Task 6: 配置 CRUD

**Files:**
- Create: `server/internal/feishu/config.go`

- [ ] **Step 1: 实现 ConfigStore 结构**

```go
package feishu

import (
    "context"
    "encoding/json"
    "time"

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
               last_sync_at, enabled, created_at, updated_at
        FROM feishu_user_configs
        WHERE user_id = $1 AND workspace_id = $2
    `, userID, workspaceID)

    var cfg FeishuUserConfig
    var contentFieldsJSON []byte
    err := row.Scan(
        &cfg.ID, &cfg.UserID, &cfg.WorkspaceID, &cfg.AppID, &cfg.AppSecretEncrypted,
        &cfg.DataSource, &cfg.BitableID, &cfg.TitleField, &cfg.AssigneeField,
        &contentFieldsJSON, &cfg.TargetType, &cfg.TargetProjectID,
        &cfg.SyncIntervalMinutes, &cfg.LastSyncAt, &cfg.Enabled,
        &cfg.CreatedAt, &cfg.UpdatedAt,
    )
    if err == pgx.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    json.Unmarshal(contentFieldsJSON, &cfg.ContentFields)
    return &cfg, nil
}

func (s *ConfigStore) Upsert(ctx context.Context, cfg *FeishuUserConfig) error {
    contentFieldsJSON, _ := json.Marshal(cfg.ContentFields)
    _, err := s.pool.Exec(ctx, `
        INSERT INTO feishu_user_configs
            (id, user_id, workspace_id, app_id, app_secret_encrypted, data_source,
             bitable_id, title_field, assignee_field, content_fields,
             target_type, target_project_id, sync_interval_minutes, enabled)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
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
            updated_at = now()
    `, cfg.ID, cfg.UserID, cfg.WorkspaceID, cfg.AppID, cfg.AppSecretEncrypted,
        cfg.DataSource, cfg.BitableID, cfg.TitleField, cfg.AssigneeField,
        contentFieldsJSON, cfg.TargetType, cfg.TargetProjectID,
        cfg.SyncIntervalMinutes, cfg.Enabled)
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
    return &m, err
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
```

---

## Task 7: 同步核心逻辑

**Files:**
- Create: `server/internal/feishu/sync.go`

- [ ] **Step 1: 实现 SyncService 结构**

```go
package feishu

import (
    "context"
    "fmt"
    "log/slog"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"
    "github.com/multica-ai/multica/server/internal/feishu"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type SyncService struct {
    configStore *ConfigStore
    tokenManager *TokenManager
    queries     *db.Queries
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

    // Get user email for filtering
    user, err := s.queries.GetUser(ctx, userID)
    if err != nil {
        return err
    }
    userEmail := user.Email.String

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
            ID:             uuid.New(),
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
            ID:             uuid.New(),
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
            ID:      mapping.MulticaIssueID,
            Title:   title,
            Content: content,
        })
        return mapping.MulticaIssueID, err
    }

    var projectID *pgtype.UUID
    if cfg.TargetType == "project" && cfg.TargetProjectID != nil {
        projectID = cfg.TargetProjectID
    }

    issue, err := s.queries.CreateIssue(ctx, db.CreateIssueParams{
        ID:          uuid.New(),
        WorkspaceID: cfg.WorkspaceID,
        Title:       title,
        Content:     content,
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
```

---

## Task 8: 定时调度器

**Files:**
- Create: `server/internal/feishu/scheduler.go`

- [ ] **Step 1: 实现 Scheduler 结构**

```go
package feishu

import (
    "context"
    "fmt"
    "sync"
    "time"

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

func (s *Scheduler) Start(ctx context.Context) {
    s.cron.Start()
    s.loadAllJobs(ctx)
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
```

---

## Task 9: Webhook 处理器

**Files:**
- Create: `server/internal/feishu/webhook.go`

- [ ] **Step 1: 实现 HandleWebhook 函数**

```go
package feishu

import (
    "encoding/json"
    "io"
    "log/slog"
    "net/http"
)

func (h *FeishuHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    var event FeishuWebhookEvent
    if err := json.Unmarshal(body, &event); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    eventType := event.Header.EventType
    slog.Info("feishu webhook received", "event_type", eventType)

    userID := chi.URLParam(r, "userId")
    workspaceID := chi.URLParam(r, "workspaceId")

    switch {
    case len(eventType) > 7 && eventType[:7] == "bitable":
        h.handleBitableWebhook(r.Context(), userID, workspaceID, event)
    case len(eventType) > 5 && eventType[:5] == "task.":
        h.handleTaskWebhook(r.Context(), userID, workspaceID, event)
    default:
        slog.Debug("unknown feishu event type", "event_type", eventType)
    }

    w.WriteHeader(http.StatusOK)
}

func (h *FeishuHandler) handleBitableWebhook(ctx context.Context, userID, workspaceID string, event FeishuWebhookEvent) {
    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    h.syncService.SyncUserFeishuData(ctx, userUUID, wsUUID)
}

func (h *FeishuHandler) handleTaskWebhook(ctx context.Context, userID, workspaceID string, event FeishuWebhookEvent) {
    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    h.syncService.SyncUserFeishuData(ctx, userUUID, wsUUID)
}
```

---

## Task 10: HTTP Handler

**Files:**
- Create: `server/internal/handler/feishu.go`

- [ ] **Step 1: 实现 FeishuHandler 结构**

```go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgtype"
    "github.com/multica-ai/multica/server/internal/feishu"
    "github.com/multica-ai/multica/server/internal/util"
)

type FeishuHandler struct {
    configStore  *feishu.ConfigStore
    syncService  *feishu.SyncService
    tokenManager *feishu.TokenManager
    queries      *db.Queries
}

func NewFeishuHandler(configStore *feishu.ConfigStore, syncService *feishu.SyncService, tokenManager *feishu.TokenManager, queries *db.Queries) *FeishuHandler {
    return &FeishuHandler{
        configStore:  configStore,
        syncService:  syncService,
        tokenManager: tokenManager,
        queries:      queries,
    }
}

func (h *FeishuHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    userID := requireUserID(r)
    workspaceID := h.resolveWorkspaceID(r)

    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    cfg, err := h.configStore.GetByUserAndWorkspace(r.Context(), userUUID, wsUUID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    if cfg == nil {
        writeJSON(w, http.StatusOK, nil)
        return
    }

    cfg.AppSecretEncrypted = "***"
    writeJSON(w, http.StatusOK, cfg)
}

func (h *FeishuHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
    userID := requireUserID(r)
    workspaceID := h.resolveWorkspaceID(r)

    var req struct {
        AppID               string   `json:"app_id"`
        AppSecret           string   `json:"app_secret"`
        DataSource          string   `json:"data_source"`
        BitableID           string   `json:"bitable_id"`
        TitleField          string   `json:"title_field"`
        AssigneeField       string   `json:"assignee_field"`
        ContentFields       []string `json:"content_fields"`
        TargetType          string   `json:"target_type"`
        TargetProjectID     string   `json:"target_project_id"`
        SyncIntervalMinutes int      `json:"sync_interval_minutes"`
        Enabled             bool     `json:"enabled"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, err.Error())
        return
    }

    encryptedSecret := encrypt(req.AppSecret)

    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    var projectID *pgtype.UUID
    if req.TargetProjectID != "" {
        pid, _ := util.ParseUUID(req.TargetProjectID)
        projectID = &pid
    }

    cfg := &feishu.FeishuUserConfig{
        UserID:              userUUID,
        WorkspaceID:         wsUUID,
        AppID:               req.AppID,
        AppSecretEncrypted:  encryptedSecret,
        DataSource:          req.DataSource,
        TargetType:          req.TargetType,
        TargetProjectID:     projectID,
        SyncIntervalMinutes: req.SyncIntervalMinutes,
        Enabled:             req.Enabled,
    }

    if req.BitableID != "" {
        cfg.BitableID = &req.BitableID
    }
    if req.TitleField != "" {
        cfg.TitleField = &req.TitleField
    }
    if req.AssigneeField != "" {
        cfg.AssigneeField = &req.AssigneeField
    }
    cfg.ContentFields = req.ContentFields

    if err := h.configStore.Upsert(r.Context(), cfg); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }

    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FeishuHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
    userID := requireUserID(r)
    workspaceID := h.resolveWorkspaceID(r)

    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    if err := h.configStore.Delete(r.Context(), userUUID, wsUUID); err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }

    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FeishuHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
    userID := requireUserID(r)
    workspaceID := h.resolveWorkspaceID(r)

    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    go h.syncService.SyncUserFeishuData(context.Background(), userUUID, wsUUID)

    writeJSON(w, http.StatusOK, map[string]string{"status": "sync started"})
}

func (h *FeishuHandler) GetBitableFields(w http.ResponseWriter, r *http.Request) {
    bitableID := chi.URLParam(r, "bitableId")
    if bitableID == "" {
        writeError(w, http.StatusBadRequest, "bitable_id required")
        return
    }

    userID := requireUserID(r)
    workspaceID := h.resolveWorkspaceID(r)

    userUUID, _ := util.ParseUUID(userID)
    wsUUID, _ := util.ParseUUID(workspaceID)

    cfg, err := h.configStore.GetByUserAndWorkspace(r.Context(), userUUID, wsUUID)
    if err != nil || cfg == nil {
        writeError(w, http.StatusBadRequest, "feishu not configured")
        return
    }

    token, _ := h.tokenManager.GetToken(r.Context(), cfg.AppID, cfg.AppSecretEncrypted)
    bitable := feishu.NewBitableClient(bitableID, token)

    fields, err := bitable.GetFields(r.Context())
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }

    writeJSON(w, http.StatusOK, fields)
}

func (h *FeishuHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    // Implementation in webhook.go
}
```

---

## Task 11: 注册路由

**Files:**
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: 在 router.go 添加飞书路由**

在 workspace-scoped routes 中添加：

```go
// Feishu integration
r.Route("/api/feishu", func(r chi.Router) {
    r.Get("/config", h.GetFeishuConfig)
    r.Put("/config", h.SaveFeishuConfig)
    r.Delete("/config", h.DeleteFeishuConfig)
    r.Post("/sync", h.TriggerFeishuSync)
    r.Get("/bitable/{bitableId}/fields", h.GetFeishuBitableFields)
    r.Get("/mappings", h.ListFeishuMappings)
    r.Delete("/mappings/{sourceType}/{feishuRecordId}", h.DeleteFeishuMapping)
})

// Public webhook endpoint
r.Post("/api/feishu/webhook/{userId}/{workspaceId}", h.HandleFeishuWebhook)
```

---

## Task 12: 前端类型和 API

**Files:**
- Modify: `packages/core/types/index.ts`
- Modify: `packages/core/api/client.ts`

- [ ] **Step 1: 添加 Feishu 相关类型**

```typescript
// packages/core/types/index.ts
export interface FeishuUserConfig {
  id: string;
  user_id: string;
  workspace_id: string;
  app_id: string;
  app_secret?: string;
  data_source: 'bitable' | 'tasks' | 'both';
  bitable_id?: string;
  title_field?: string;
  assignee_field?: string;
  content_fields: string[];
  target_type: 'project' | 'personal';
  target_project_id?: string;
  sync_interval_minutes: number;
  last_sync_at?: string;
  enabled: boolean;
}

export interface FeishuTaskMapping {
  id: string;
  user_id: string;
  workspace_id: string;
  feishu_record_id: string;
  feishu_task_id?: string;
  source_type: 'bitable' | 'tasks';
  multica_issue_id: string;
  created_at: string;
  updated_at: string;
}

export interface BitableField {
  field_id: string;
  field_name: string;
  type: number;
}
```

- [ ] **Step 2: 在 API client 中添加方法**

```typescript
async getFeishuConfig(): Promise<FeishuUserConfig | null> {
  return this.request(`/api/feishu/config`);
}

async saveFeishuConfig(config: FeishuUserConfig): Promise<void> {
  await this.request(`/api/feishu/config`, { method: 'PUT', body: JSON.stringify(config) });
}

async deleteFeishuConfig(): Promise<void> {
  await this.request(`/api/feishu/config`, { method: 'DELETE' });
}

async triggerFeishuSync(): Promise<void> {
  await this.request(`/api/feishu/sync`, { method: 'POST' });
}

async getFeishuBitableFields(bitableId: string): Promise<BitableField[]> {
  return this.request(`/api/feishu/bitable/${bitableId}/fields`);
}

async getFeishuMappings(): Promise<FeishuTaskMapping[]> {
  return this.request(`/api/feishu/mappings`);
}

async deleteFeishuMapping(sourceType: string, feishuRecordId: string): Promise<void> {
  await this.request(`/api/feishu/mappings/${sourceType}/${feishuRecordId}`, { method: 'DELETE' });
}
```

---

## Task 13: 飞书配置 Tab UI

**Files:**
- Create: `packages/views/settings/components/feishu-tab.tsx`

- [ ] **Step 1: 实现 FeishuTab 组件**

```typescript
"use client";

import React, { useState } from "react";
import { useCurrentWorkspace } from "@multica/core/paths";
import { useT } from "../../i18n";
import { api } from "@multica/core/api";
import type { FeishuUserConfig, BitableField } from "@multica/core/types";

export function FeishuTab() {
  const { t } = useT("settings");
  const workspace = useCurrentWorkspace();

  const [config, setConfig] = useState<Partial<FeishuUserConfig>>({
    data_source: "bitable",
    target_type: "personal",
    sync_interval_minutes: 15,
    content_fields: [],
    enabled: true,
  });
  const [fields, setFields] = useState<BitableField[]>([]);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);

  React.useEffect(() => {
    api.getFeishuConfig().then((cfg) => {
      if (cfg) {
        setConfig(cfg);
        if (cfg.bitable_id) {
          loadFields(cfg.bitable_id);
        }
      }
    });
  }, []);

  const loadFields = async (bitableId: string) => {
    const flds = await api.getFeishuBitableFields(bitableId);
    setFields(flds);
  };

  const handleSave = async () => {
    setLoading(true);
    await api.saveFeishuConfig(config as FeishuUserConfig);
    setLoading(false);
  };

  const handleSync = async () => {
    setSyncing(true);
    await api.triggerFeishuSync();
    setSyncing(false);
  };

  const webhookUrl = `${window.location.origin}/api/feishu/webhook/${workspace?.id}`;

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t(($) => $.feishu.title)}</h2>
        <p className="text-sm text-muted-foreground">{t(($) => $.feishu.description)}</p>
      </div>

      {/* App Credentials */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium">{t(($) => $.feishu.credentials)}</h3>
        <div className="grid gap-4">
          <div>
            <label className="text-sm">{t(($) => $.feishu.app_id)}</label>
            <input
              type="text"
              className="w-full p-2 border rounded"
              value={config.app_id || ""}
              onChange={(e) => setConfig({ ...config, app_id: e.target.value })}
            />
          </div>
          <div>
            <label className="text-sm">{t(($) => $.feishu.app_secret)}</label>
            <input
              type="password"
              className="w-full p-2 border rounded"
              value={config.app_secret || ""}
              onChange={(e) => setConfig({ ...config, app_secret: e.target.value })}
            />
          </div>
        </div>
      </div>

      {/* Data Source */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium">{t(($) => $.feishu.data_source)}</h3>
        <div className="flex gap-4">
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={config.data_source?.includes("bitable")}
              onChange={(e) => {
                const sources = config.data_source?.split(",") || [];
                if (e.target.checked) sources.push("bitable");
                else sources.splice(sources.indexOf("bitable"), 1);
                setConfig({ ...config, data_source: sources.join(",") });
              }}
            />
            {t(($) => $.feishu.bitable)}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={config.data_source?.includes("tasks")}
              onChange={(e) => {
                const sources = config.data_source?.split(",") || [];
                if (e.target.checked) sources.push("tasks");
                else sources.splice(sources.indexOf("tasks"), 1);
                setConfig({ ...config, data_source: sources.join(",") });
              }}
            />
            {t(($) => $.feishu.tasks)}
          </label>
        </div>
      </div>

      {/* Bitable Settings */}
      {config.data_source?.includes("bitable") && (
        <div className="space-y-4">
          <h3 className="text-sm font-medium">{t(($) => $.feishu.bitable_settings)}</h3>
          <div className="grid gap-4">
            <div>
              <label className="text-sm">{t(($) => $.feishu.bitable_id)}</label>
              <input
                type="text"
                className="w-full p-2 border rounded"
                value={config.bitable_id || ""}
                onChange={(e) => {
                  setConfig({ ...config, bitable_id: e.target.value });
                  if (e.target.value) loadFields(e.target.value);
                }}
              />
            </div>
            <div>
              <label className="text-sm">{t(($) => $.feishu.title_field)}</label>
              <select
                className="w-full p-2 border rounded"
                value={config.title_field || ""}
                onChange={(e) => setConfig({ ...config, title_field: e.target.value })}
              >
                <option value="">--</option>
                {fields.map((f) => (
                  <option key={f.field_id} value={f.field_name}>{f.field_name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-sm">{t(($) => $.feishu.assignee_field)}</label>
              <select
                className="w-full p-2 border rounded"
                value={config.assignee_field || ""}
                onChange={(e) => setConfig({ ...config, assignee_field: e.target.value })}
              >
                <option value="">--</option>
                {fields.map((f) => (
                  <option key={f.field_id} value={f.field_name}>{f.field_name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-sm">{t(($) => $.feishu.content_fields)}</label>
              <div className="flex flex-wrap gap-2">
                {fields.map((f) => (
                  <label key={f.field_id} className="flex items-center gap-1">
                    <input
                      type="checkbox"
                      checked={config.content_fields?.includes(f.field_name)}
                      onChange={(e) => {
                        const cfs = config.content_fields || [];
                        if (e.target.checked) cfs.push(f.field_name);
                        else cfs.splice(cfs.indexOf(f.field_name), 1);
                        setConfig({ ...config, content_fields: cfs });
                      }}
                    />
                    {f.field_name}
                  </label>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Sync Settings */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium">{t(($) => $.feishu.sync_settings)}</h3>
        <div className="grid gap-4">
          <div>
            <label className="text-sm">{t(($) => $.feishu.sync_interval)}</label>
            <input
              type="number"
              className="w-full p-2 border rounded"
              value={config.sync_interval_minutes || 15}
              onChange={(e) => setConfig({ ...config, sync_interval_minutes: parseInt(e.target.value) })}
            />
          </div>
          <div>
            <label className="text-sm">{t(($) => $.feishu.target_type)}</label>
            <select
              className="w-full p-2 border rounded"
              value={config.target_type || "personal"}
              onChange={(e) => setConfig({ ...config, target_type: e.target.value as "project" | "personal" })}
            >
              <option value="personal">{t(($) => $.feishu.personal)}</option>
              <option value="project">{t(($) => $.feishu.project)}</option>
            </select>
          </div>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={config.enabled}
              onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
            />
            {t(($) => $.feishu.enabled)}
          </label>
        </div>
      </div>

      {/* Webhook URL */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium">{t(($) => $.feishu.webhook)}</h3>
        <div className="p-3 bg-muted rounded font-mono text-sm break-all">
          {webhookUrl}
        </div>
        <p className="text-xs text-muted-foreground">
          {t(($) => $.feishu.webhook_hint)}
        </p>
      </div>

      {/* Actions */}
      <div className="flex gap-4">
        <button
          className="px-4 py-2 bg-primary text-primary-foreground rounded"
          onClick={handleSave}
          disabled={loading}
        >
          {loading ? t(($) => $.feishu.saving) : t(($) => $.feishu.save)}
        </button>
        <button
          className="px-4 py-2 border rounded"
          onClick={handleSync}
          disabled={syncing}
        >
          {syncing ? t(($) => $.feishu.syncing) : t(($) => $.feishu.sync_now)}
        </button>
      </div>

      {config.last_sync_at && (
        <p className="text-xs text-muted-foreground">
          {t(($) => $.feishu.last_sync)}: {new Date(config.last_sync_at).toLocaleString()}
        </p>
      )}
    </div>
  );
}
```

---

## Task 14: 注册 Tab 到设置页面

**Files:**
- Modify: `packages/views/settings/components/settings-page.tsx`

- [ ] **Step 1: 添加飞书 Tab**

在 WORKSPACE_TAB_KEYS 数组中添加 "feishu"，导入 FeishuTab 组件，并添加对应的 TabsTrigger 和 TabsContent。

```typescript
import { FeishuTab } from "./feishu-tab";

const WORKSPACE_TAB_KEYS = ["general", "repositories", "labs", "members", "feishu"] as const;

const WORKSPACE_TAB_ICONS = {
  general: Settings,
  repositories: FolderGit2,
  labs: FlaskConical,
  members: Users,
  feishu: LinkIcon, // import from lucide-react
} as const;

const WORKSPACE_TAB_VALUES = {
  general: "workspace",
  repositories: "repositories",
  labs: "labs",
  members: "members",
  feishu: "feishu",
} as const;
```

添加 TabsContent:
```tsx
<TabsContent value="feishu"><FeishuTab /></TabsContent>
```

---

## Task 15: 国际化

**Files:**
- Create: `packages/views/locales/zh-CN/feishu.json`
- Create: `packages/views/locales/en/feishu.json`

- [ ] **Step 1: 添加翻译**

```json
// zh-CN
{
  "feishu": {
    "title": "飞书集成",
    "description": "从飞书多维表格和任务同步任务到 Multica",
    "credentials": "飞书应用凭证",
    "app_id": "App ID",
    "app_secret": "App Secret",
    "data_source": "数据来源",
    "bitable": "多维表格",
    "tasks": "飞书任务",
    "bitable_settings": "多维表格设置",
    "bitable_id": "多维表格 ID",
    "title_field": "标题列",
    "assignee_field": "负责人列",
    "content_fields": "内容列（可多选）",
    "sync_settings": "同步设置",
    "sync_interval": "同步间隔（分钟）",
    "target_type": "同步目标",
    "personal": "个人工作区",
    "project": "指定项目",
    "enabled": "启用同步",
    "webhook": "Webhook 地址",
    "webhook_hint": "在飞书开放平台配置此 URL 为 Webhook 地址",
    "save": "保存",
    "saving": "保存中...",
    "sync_now": "立即同步",
    "syncing": "同步中...",
    "last_sync": "上次同步"
  }
}
```

---

## Task 16: 验证和测试

- [ ] **Step 1: 运行 typecheck**

```bash
pnpm typecheck
```

- [ ] **Step 2: 运行 Go 编译**

```bash
cd server && go build ./...
```

- [ ] **Step 3: 运行迁移**

```bash
cd server && go run ./cmd/server migrate up
```

- [ ] **Step 4: 启动服务并手动测试**

```bash
make dev
```

---

## 实施顺序

1. Task 1: 数据库迁移
2. Task 2: 后端类型定义
3. Task 3-6: Token、Bitable、Tasks API 客户端
4. Task 7: 配置 CRUD
5. Task 8: 同步核心逻辑
6. Task 9: 定时调度器
7. Task 10: Webhook 处理器
8. Task 11: HTTP Handler
9. Task 12: 注册路由
10. Task 13-15: 前端 UI
11. Task 16: 验证和测试

---

## 备注

- App Secret 使用 AES-256-GCM 加密存储
- Webhook 端点可选择验证飞书签名
- 同步操作建议在后台 goroutine 中执行，避免阻塞 HTTP 请求
- 定时调度使用 robfig/cron 库（需添加到 go.mod）