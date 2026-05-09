# 飞书多维表格过滤条件实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 允许用户配置分组过滤条件，决定哪些飞书多维表格记录才同步到多维表格

**Architecture:** 后端存储 filter_config JSON，增删改查复用现有 feishu_user_configs 表。同步时在 shouldSyncRecord 中增加 evaluateFilter 调用。前端在飞书设置页面增加过滤条件配置面板。

**Tech Stack:** Go (Chi/slog/pgx), React (Next.js), PostgreSQL (jsonb)

---

## 文件清单

### 新建
- `server/migrations/071_feishu_filter_config.up.sql`
- `server/migrations/071_feishu_filter_config.down.sql`

### 修改
- `server/internal/feishu/types.go` — 增加 FilterGroup, FilterCondition, FilterConfig 类型，FeishuUserConfig 增加 FilterConfig 字段
- `server/internal/feishu/config.go` — SELECT/UPSERT 语句增加 filter_config 列
- `server/internal/feishu/sync.go` — 增加 evaluateFilter 函数，修改 shouldSyncRecord 调用
- `server/internal/handler/feishu.go` — SaveConfig 的请求结构增加 FilterConfig 字段
- `packages/core/types/feishu.ts` — 前端类型增加 FilterGroup, FilterCondition, FilterConfig
- `packages/views/settings/components/feishu-tab.tsx` — 增加过滤条件 UI

---

## Task 1: 数据库迁移

**Files:**
- Create: `server/migrations/071_feishu_filter_config.up.sql`
- Create: `server/migrations/071_feishu_filter_config.down.sql`

- [ ] **Step 1: 创建迁移文件**

```sql
-- server/migrations/071_feishu_filter_config.up.sql
ALTER TABLE feishu_user_configs ADD COLUMN filter_config JSONB NOT NULL DEFAULT '[]';
```

```sql
-- server/migrations/071_feishu_filter_config.down.sql
ALTER TABLE feishu_user_configs DROP COLUMN filter_config;
```

- [ ] **Step 2: 应用迁移**

```bash
cd /xavier_ssd/datadisk/test/wdjh/server && go run ./cmd/migrate up
```

---

## Task 2: 后端类型定义

**Files:**
- Modify: `server/internal/feishu/types.go`

- [ ] **Step 1: 添加类型定义**

在 `types.go` 末尾添加：

```go
type FilterGroup struct {
    Logic      string            `json:"logic"`
    Conditions []FilterCondition `json:"conditions"`
}

type FilterCondition struct {
    Field    string      `json:"field"`
    Operator string      `json:"operator"`
    Value    interface{} `json:"value"`
}

type FilterConfig struct {
    FilterGroups []FilterGroup `json:"filter_groups"`
}
```

- [ ] **Step 2: FeishuUserConfig 增加字段**

在 `FeishuUserConfig` 结构体中添加：

```go
FilterConfig FilterConfig `json:"filter_config"`
```

---

## Task 3: ConfigStore 增删改查

**Files:**
- Modify: `server/internal/feishu/config.go:22-48` (GetByUserAndWorkspace)
- Modify: `server/internal/feishu/config.go:51-83` (Upsert)

- [ ] **Step 1: 修改 GetByUserAndWorkspace**

SELECT 语句增加 `filter_config` 列：

```go
row := s.pool.QueryRow(ctx, `
    SELECT id, user_id, workspace_id, app_id, app_secret_encrypted, data_source,
           bitable_id, title_field, assignee_field, content_fields,
           target_type, target_project_id, sync_interval_minutes,
           last_sync_at, enabled, filter_config, created_at, updated_at
    FROM feishu_user_configs
    WHERE user_id = $1 AND workspace_id = $2
`, userID, workspaceID)
```

Scan 增加 `&cfg.FilterConfig`（在 `cfg.Enabled` 之后），并添加：

```go
filterConfigJSON, _ := json.Marshal([]interface{}{})
json.Unmarshal(filterConfigJSON, &cfg.FilterConfig)
```

> 实际读取时用 `json.RawMessage` 更灵活：

修改 `FeishuUserConfig.FilterConfig` 类型为 `json.RawMessage`，然后：

```go
var filterConfigJSON []byte
err := row.Scan(
    ...existing fields...,
    &cfg.Enabled,
    &filterConfigJSON,
    &cfg.CreatedAt, &cfg.UpdatedAt,
)
json.Unmarshal(filterConfigJSON, &cfg.FilterConfig)
```

- [ ] **Step 2: 修改 Upsert**

INSERT/ON CONFLICT 语句增加 `filter_config` 字段，Upsert 函数签名不变，因为 FeishuUserConfig 已有 FilterConfig 字段。确保 `contentFieldsJSON` 之后有 `filterConfigJSON, _ := json.Marshal(cfg.FilterConfig)` 并传入占位符。

---

## Task 4: 过滤求值逻辑

**Files:**
- Modify: `server/internal/feishu/sync.go`

- [ ] **Step 1: 添加 evaluateFilter 函数**

在 `sync.go` 末尾或 shouldSyncRecord 附近添加：

```go
func evaluateFilter(fields map[string]interface{}, groups []FilterGroup) bool {
    if len(groups) == 0 {
        return true
    }

    for _, group := range groups {
        if !evaluateGroup(fields, group) {
            return false
        }
    }
    return true
}

func evaluateGroup(fields map[string]interface{}, group FilterGroup) bool {
    if len(group.Conditions) == 0 {
        return true
    }

    for _, cond := range group.Conditions {
        matched := evaluateCondition(fields, cond)
        if group.Logic == "AND" && !matched {
            return false
        }
        if group.Logic == "OR" && matched {
            return true
        }
    }

    if group.Logic == "AND" {
        return true
    }
    return false // OR with no matches
}

func evaluateCondition(fields map[string]interface{}, cond FilterCondition) bool {
    val, ok := fields[cond.Field]
    if !ok {
        return cond.Operator == "is_empty"
    }

    switch cond.Operator {
    case "equals":
        return equals(val, cond.Value)
    case "not_equals":
        return !equals(val, cond.Value)
    case "contains":
        return contains(val, cond.Value)
    case "not_contains":
        return !contains(val, cond.Value)
    case "is_empty":
        return isEmpty(val)
    case "is_not_empty":
        return !isEmpty(val)
    case "greater_than":
        return compare(val, cond.Value, ">")
    case "less_than":
        return compare(val, cond.Value, "<")
    case "greater_or_equal":
        return compare(val, cond.Value, ">=")
    case "less_or_equal":
        return compare(val, cond.Value, "<=")
    case "contains_any":
        return containsAny(val, cond.Value)
    case "contains_all":
        return containsAll(val, cond.Value)
    case "not_contains_any":
        return !containsAny(val, cond.Value)
    case "before":
        return dateBefore(val, cond.Value)
    case "after":
        return dateAfter(val, cond.Value)
    case "is_checked":
        b, _ := val.(bool)
        return b == true
    case "is_not_checked":
        b, _ := val.(bool)
        return b == false
    default:
        slog.Warn("unknown operator", "operator", cond.Operator)
        return true
    }
}

func getStringValue(v interface{}) string {
    switch val := v.(type) {
    case string:
        return val
    case float64:
        return fmt.Sprintf("%v", val)
    default:
        return fmt.Sprintf("%v", val)
    }
}

func equals(val, condValue interface{}) bool {
    sv := getStringValue(val)
    cv := getStringValue(condValue)
    return strings.EqualFold(sv, cv)
}

func contains(val, condValue interface{}) bool {
    sv := getStringValue(val)
    cv := getStringValue(condValue)
    return strings.Contains(strings.ToLower(sv), strings.ToLower(cv))
}

func isEmpty(val interface{}) bool {
    if val == nil {
        return true
    }
    switch v := val.(type) {
    case string:
        return strings.TrimSpace(v) == ""
    case []interface{}:
        return len(v) == 0
    case map[string]interface{}:
        return len(v) == 0
    }
    return false
}

func containsAny(val, condValue interface{}) bool {
    arr, ok := val.([]interface{})
    if !ok {
        return false
    }
    condStr := getStringValue(condValue)
    for _, item := range arr {
        if strings.EqualFold(getStringValue(item), condStr) {
            return true
        }
    }
    return false
}

func containsAll(val, condValue interface{}) bool {
    arr, ok := val.([]interface{})
    if !ok {
        return false
    }
    condStrs, ok := condValue.([]interface{})
    if !ok {
        condStrs = []interface{}{condValue}
    }
    matched := 0
    for _, cs := range condStrs {
        csStr := getStringValue(cs)
        for _, item := range arr {
            if strings.EqualFold(getStringValue(item), csStr) {
                matched++
                break
            }
        }
    }
    return matched == len(condStrs)
}

func compare(val, condValue interface{}, op string) bool {
    var fv, cv float64
    switch v := val.(type) {
    case float64:
        fv = v
    case int:
        fv = float64(v)
    case string:
        f, _ := strconv.ParseFloat(v, 64)
        fv = f
    default:
        return false
    }
    switch v := condValue.(type) {
    case float64:
        cv = v
    case int:
        cv = float64(v)
    case string:
        f, _ := strconv.ParseFloat(v, 64)
        cv = f
    default:
        return false
    }
    switch op {
    case ">":
        return fv > cv
    case "<":
        return fv < cv
    case ">=":
        return fv >= cv
    case "<=":
        return fv <= cv
    }
    return false
}

func dateBefore(val, condValue interface{}) bool {
    v := getStringValue(val)
    cv := getStringValue(condValue)
    return v < cv
}

func dateAfter(val, condValue interface{}) bool {
    v := getStringValue(val)
    cv := getStringValue(condValue)
    return v > cv
}
```

- [ ] **Step 2: 修改 shouldSyncRecord**

在 `shouldSyncRecord` 开头增加 filter_groups 判断：

```go
func (s *SyncService) shouldSyncRecord(ctx context.Context, fields map[string]interface{}, assigneeField *string, userEmail string, contactClient *ContactClient, filterGroups []FilterGroup) bool {
    // 先检查过滤条件
    if len(filterGroups) > 0 {
        if !evaluateFilter(fields, filterGroups) {
            return false
        }
    }

    // 现有的 assigneeField 判断逻辑...
```

- [ ] **Step 3: 修改 syncBitable 中的调用**

在 `syncBitable` 的记录遍历中：

```go
if !s.shouldSyncRecord(ctx, record.Fields, cfg.AssigneeField, userEmail, contactClient, cfg.FilterConfig.FilterGroups) {
```

---

## Task 5: 前端类型

**Files:**
- Modify: `packages/core/types/feishu.ts`

- [ ] **Step 1: 添加类型**

```typescript
export interface FilterCondition {
  field: string;
  operator: string;
  value: string | string[] | boolean | null;
}

export interface FilterGroup {
  logic: "AND" | "OR";
  conditions: FilterCondition[];
}

export interface FilterConfig {
  filter_groups: FilterGroup[];
}
```

- [ ] **Step 2: FeishuUserConfig 增加字段**

```typescript
export interface FeishuUserConfig {
  // ... existing fields ...
  filter_config: FilterConfig;
}
```

---

## Task 6: 前端配置 UI

**Files:**
- Modify: `packages/views/settings/components/feishu-tab.tsx`

- [ ] **Step 1: 初始化 filter_config 状态**

```typescript
const [config, setConfig] = useState<Partial<FeishuUserConfig>>({
  // ... existing defaults ...
  filter_config: { filter_groups: [] },
});
```

- [ ] **Step 2: 添加过滤条件面板组件**

在现有设置区域的适当位置添加折叠面板，包含：
- 遍历 `config.filter_config.filter_groups` 显示每个分组
- 每个分组内遍历 conditions 显示每行条件
- 字段选择下拉框（从 `fields` 列表读取 field_name）
- 运算符选择下拉框（根据字段 type 过滤可用运算符）
- 值输入框（根据运算符类型显示：文本/日期/下拉框）
- 添加条件按钮、删除条件按钮
- 添加分组按钮、分组间 AND/OR 选择器

- [ ] **Step 3: 运算符与字段类型映射**

```typescript
const OPERATORS_BY_TYPE: Record<number, string[]> = {
  1: ["equals", "not_equals", "contains", "not_contains", "is_empty", "is_not_empty"],     // text
  2: ["equals", "not_equals", "greater_than", "less_than", "is_empty", "is_not_empty"],      // number
  3: ["equals", "not_equals", "is_empty", "is_not_empty"],                                    // single select
  4: ["contains_any", "contains_all", "not_contains_any", "is_empty", "is_not_empty"],        // multi select
  5: ["equals", "before", "after", "is_empty", "is_not_empty"],                               // date
  7: ["is_checked", "is_not_checked", "is_empty", "is_not_empty"],                           // checkbox
  11: ["equals", "not_equals", "is_empty", "is_not_empty"],                                  // person
};
```

- [ ] **Step 4: 值输入组件**

根据运算符类型渲染：
- `is_empty` / `is_not_empty` / `is_checked` / `is_not_checked`：不显示输入框
- `equals` / `not_equals`（单选）：下拉框，选项从 fields 中读取
- 日期：date input
- 复选框：toggle switch
- 其他：text input

---

## Task 7: 保存和加载

**Files:**
- Modify: `packages/views/settings/components/feishu-tab.tsx`
- Modify: `server/internal/handler/feishu.go`

- [ ] **Step 1: 后端 SaveConfig 请求结构**

在 `feishu.go` SaveConfig 的请求 struct 中添加：

```go
FilterConfig struct {
    FilterGroups []FilterGroup `json:"filter_groups"`
} `json:"filter_config"`
```

并在构建 cfg 时设置 `cfg.FilterConfig = filterConfigJSON`（用 json.Marshal 序列化）。

- [ ] **Step 2: 加载时初始化**

在 feishu-tab.tsx 的 `useEffect` 加载 config 后，确保 `filter_config` 有默认值：

```typescript
if (cfg && !cfg.filter_config) {
  cfg.filter_config = { filter_groups: [] };
  setConfig(cfg);
}
```

---

## Task 8: 测试

- [ ] **Step 1: 启动服务**

```bash
make dev
```

- [ ] **Step 2: 配置过滤条件**

1. 打开飞书设置页面
2. 添加分组，选择字段、运算符、值
3. 保存配置
4. 点击立即同步
5. 确认日志中记录被正确过滤或同步

- [ ] **Step 3: 验证日志**

```bash
# 同步日志应显示：
bitable sync: evaluating filter record_id=xxx filter_groups_count=1 matched=true
```
