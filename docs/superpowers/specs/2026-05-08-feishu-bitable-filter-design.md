# 飞书多维表格同步过滤条件设计

## 概述

允许用户配置过滤条件，决定哪些飞书多维表格记录才同步到多维表格。采用分组 + 条件的树形结构，支持 AND/OR 嵌套逻辑。

## 数据模型

### 过滤条件结构 (JSON，存储在 `feishu_user_configs` 表)

```json
{
  "filter_groups": [
    {
      "logic": "AND",
      "conditions": [
        {
          "field": "状态",
          "operator": "equals",
          "value": "待启动"
        },
        {
          "field": "创建人",
          "operator": "contains",
          "value": "马鑫琼"
        }
      ]
    },
    {
      "logic": "OR",
      "conditions": [
        {
          "field": "优先级",
          "operator": "equals",
          "value": "高"
        }
      ]
    }
  ]
}
```

**求值逻辑：**
- 记录匹配 = 所有分组都匹配
- 分组匹配 = 分组内 `logic` 为 `AND` 时所有条件都匹配，为 `OR` 时任一条件匹配
- 条件不匹配 = 跳过该记录

### 运算符定义

| 字段类型 | 支持的运算符 |
|---------|------------|
| 文本 | equals, not_equals, contains, not_contains, is_empty, is_not_empty |
| 数字 | equals, not_equals, greater_than, less_than, greater_or_equal, less_or_equal, is_empty, is_not_empty |
| 单选 | equals, not_equals, is_empty, is_not_empty |
| 多选 | contains_any, contains_all, not_contains_any, is_empty, is_not_empty |
| 日期 | equals, before, after, is_empty, is_not_empty |
| 复选框 | is_checked, is_not_checked, is_empty, is_not_empty |
| 人员 | equals, not_equals, is_empty, is_not_empty |

## 后端实现

### 类型定义

```go
type FilterGroup struct {
    Logic      string          `json:"logic"` // "AND" | "OR"
    Conditions []FilterCondition `json:"conditions"`
}

type FilterCondition struct {
    Field    string      `json:"field"`
    Operator string      `json:"operator"`
    Value    interface{} `json:"value"` // string | []string | bool | nil
}

type FilterConfig struct {
    FilterGroups []FilterGroup `json:"filter_groups"`
}
```

### 过滤求值

在 `syncBitable` 的 `shouldSyncRecord` 中增加过滤条件判断：

```
if !evaluateFilter(record.Fields, cfg.FilterGroups):
    skip record
```

`evaluateFilter`:
1. 遍历每个 FilterGroup
2. 对每个分组，遍历 conditions，按 logic (AND/OR) 求值
3. 运算符函数读取 `fields[field_name]`，与 condition.value 比较
4. 所有分组都返回 true → 同步该记录

### 数据库变更

- `feishu_user_configs` 表增加 `filter_config jsonb DEFAULT '[]'` 字段
- `SaveConfig` 时保存 filter_config
- `GetByUserAndWorkspace` 时读取 filter_config

## 前端实现

### 配置界面

在现有飞书设置页面中，「同步设置」区域增加「过滤条件」折叠面板：

1. 点击「添加过滤条件」展开过滤条件配置区
2. 显示已配置的分组列表（分组间 connector 选择 AND/OR）
3. 每个分组内显示条件行列表（字段选择 + 运算符选择 + 值输入）
4. 每行可删除，分组可删除（至少保留一个分组）
5. 「添加分组」按钮增加新分组

### 字段选择

下拉框列出该多维表格的所有字段名（从已加载的 fields 数据中读取）。

### 运算符选择

根据所选字段类型显示对应的运算符列表：

- 首次选择字段后，根据字段 type 过滤可用的运算符
- 切换字段类型时清空已选值

### 值输入

- 文本/数字：文本输入框
- 单选：下拉框（选项值从 bitable fields 中读取 type-specific options）
- 多选：多选下拉框
- 日期：日期选择器
- 复选框：是/否切换
- 人员：文本输入（按 open_id 匹配）
- 为空/不为空：不显示值输入框

### 保存

过滤条件随 FeishuConfig 一起保存到后端。

## 同步日志增强

```
slog.Info("bitable sync: evaluating filter",
    "record_id", record.RecordID,
    "filter_groups_count", len(cfg.FilterGroups),
    "matched", matched)
```
