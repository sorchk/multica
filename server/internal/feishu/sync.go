package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
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
	slog.Info("sync: starting", "user_id", userID, "workspace_id", workspaceID)
	cfg, err := s.configStore.GetByUserAndWorkspace(ctx, userID, workspaceID)
	if err != nil {
		slog.Error("sync: config fetch error", "error", err)
		return fmt.Errorf("config fetch error: %w", err)
	}
	if cfg == nil {
		slog.Error("sync: config is nil")
		return fmt.Errorf("config is nil")
	}
	if !cfg.Enabled {
		slog.Error("sync: config disabled", "user_id", userID)
		return fmt.Errorf("config disabled")
	}
	slog.Info("sync: config loaded", "data_source", cfg.DataSource, "bitable_id", cfg.BitableID, "title_field", cfg.TitleField, "assignee_field", cfg.AssigneeField)

	user, err := s.queries.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	userEmail := user.Email

	secret, err := DecryptSecret(cfg.AppSecretEncrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret: %w", err)
	}
	token, err := s.tokenManager.GetToken(ctx, cfg.AppID, secret)
	if err != nil {
		slog.Error("sync: failed to get token", "error", err)
		return fmt.Errorf("failed to get token: %w", err)
	}
	slog.Info("sync: token obtained, starting bitable/tasks sync", "data_source", cfg.DataSource)

	if strings.Contains(cfg.DataSource, "bitable") && cfg.BitableID != nil {
		slog.Info("sync: calling syncBitable", "bitable_id", *cfg.BitableID)
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

	slog.Info("cfg.FilterConfig: ", "filter_config", string(cfg.FilterConfig), "length", len(cfg.FilterConfig))

	var result struct {
		FilterGroups []FilterGroup `json:"filter_groups"`
	}
	json.Unmarshal(cfg.FilterConfig, &result)
	filterGroups := result.FilterGroups
	slog.Info("bitable sync: fetched records", "count", len(resp.Data.Items), "bitable_id", *cfg.BitableID)

	slog.Info("filterGroups: ", "count", len(filterGroups))
	for _, record := range resp.Data.Items {
		slog.Info("bitable sync: checking record", "record_id", record.RecordID, "fields", fmt.Sprintf("%v", record.Fields))

		if len(filterGroups) > 0 {
			if !evaluateFilter(record.Fields, filterGroups) {
				slog.Info("bitable sync: skipping bitable (filter mismatch)", "record_id", record.RecordID)
				continue
			}
		}

		matchedUserID := s.findMatchingUser(ctx, record.Fields, cfg.AssigneeField, userEmail)
		slog.Info("bitable sync: matched user", "record_id", record.RecordID, "matched", matchedUserID.Valid)

		title := s.extractFieldValue(record.Fields, cfg.TitleField)
		content := s.extractContentFields(record.Fields, cfg.ContentFields)
		slog.Info("bitable sync: creating issue", "record_id", record.RecordID, "title", title)

		issueID, err := s.createOrUpdateIssue(ctx, cfg, record.RecordID, title, content, matchedUserID)
		if err != nil {
			slog.Error("failed to sync record", "record_id", record.RecordID, "error", err)
			continue
		}

		slog.Info("bitable sync: issue created", "record_id", record.RecordID, "issue_id", issueID)

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

func (s *SyncService) findMatchingUser(ctx context.Context, fields map[string]interface{}, assigneeField *string, userEmail string) pgtype.UUID {
	if assigneeField == nil {
		return pgtype.UUID{}
	}

	val, ok := fields[*assigneeField]
	if !ok {
		return pgtype.UUID{}
	}

	if str, ok := val.(string); ok {
		if strings.Contains(strings.ToLower(str), strings.ToLower(userEmail)) {
			user, err := s.queries.GetUserByEmail(ctx, userEmail)
			if err != nil {
				slog.Warn("failed to find multica user by email", "email", userEmail, "error", err)
				return pgtype.UUID{}
			}
			return user.ID
		}
		return pgtype.UUID{}
	}

	assigneeInfo := s.extractAssigneeInfoFromField(val)
	if assigneeInfo == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{}
}

func (s *SyncService) syncTasks(ctx context.Context, cfg *FeishuUserConfig, token, userEmail string) error {
	tasksClient := NewTasksClient(token)

	resp, err := tasksClient.GetTasks(ctx, 100, "")
	if err != nil {
		return fmt.Errorf("failed to fetch tasks: %w", err)
	}

	var filterGroups []FilterGroup
	if len(cfg.TasksFilterConfig) > 0 {
		json.Unmarshal(cfg.TasksFilterConfig, &filterGroups)
	}

	for _, task := range resp.Data.Items {
		taskFields := map[string]interface{}{
			"summary":     task.Summary,
			"description": task.Description,
		}
		if task.Due != nil {
			taskFields["due"] = task.Due.Timestamp
		}

		if len(filterGroups) > 0 {
			if !evaluateFilter(taskFields, filterGroups) {
				slog.Info("tasks sync: skipping task (filter mismatch)", "task_guid", task.GUID)
				continue
			}
		}

		issueID, err := s.createOrUpdateIssue(ctx, cfg, task.GUID, task.Summary, task.Description, pgtype.UUID{})
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

func (s *SyncService) createOrUpdateIssue(ctx context.Context, cfg *FeishuUserConfig, recordID, title, content string, assigneeID pgtype.UUID) (pgtype.UUID, error) {
	mapping, err := s.configStore.GetMapping(ctx, cfg.UserID, cfg.WorkspaceID, "bitable", recordID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	if mapping != nil {
		existing, err := s.queries.GetIssue(ctx, mapping.MulticaIssueID)
		if err != nil {
			return mapping.MulticaIssueID, err
		}
		if existing.Status != "backlog" {
			return mapping.MulticaIssueID, nil
		}
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

	var assigneeType pgtype.Text
	var assigneeIDParam pgtype.UUID
	if assigneeID.Valid {
		assigneeType = pgtype.Text{String: "member", Valid: true}
		assigneeIDParam = assigneeID
	}

	issueNumber, err := s.queries.IncrementIssueCounter(ctx, cfg.WorkspaceID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("failed to increment issue counter: %w", err)
	}

	issue, err := s.queries.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID:  cfg.WorkspaceID,
		Title:        title,
		Description:  pgtype.Text{String: content, Valid: content != ""},
		CreatorType:  "member",
		CreatorID:    cfg.UserID,
		Status:       "backlog",
		Priority:     "none",
		ProjectID:    projectID,
		Number:       issueNumber,
		AssigneeType: assigneeType,
		AssigneeID:   assigneeIDParam,
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

type AssigneeInfo struct {
	Persons []Person
}

type Person struct {
	ID    string
	Name  string
	Email string
}

func (s *SyncService) extractAssigneeInfoFromField(val interface{}) *AssigneeInfo {
	info := &AssigneeInfo{}

	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if person, ok := item.(map[string]interface{}); ok {
				personInfo := s.extractPersonFromObject(person)
				if personInfo != nil {
					info.Persons = append(info.Persons, *personInfo)
				}
			}
		}
	case map[string]interface{}:
		if personInfo := s.extractPersonFromObject(v); personInfo != nil {
			info.Persons = append(info.Persons, *personInfo)
		}
	}

	if len(info.Persons) == 0 {
		return nil
	}
	return info
}

func (s *SyncService) extractPersonFromObject(obj map[string]interface{}) *Person {
	id, _ := obj["id"].(string)
	name, _ := obj["name"].(string)
	email, _ := obj["email"].(string)
	if id == "" && name == "" {
		return nil
	}
	return &Person{ID: id, Name: name, Email: email}
}

func evaluateFilter(fields map[string]interface{}, groups []FilterGroup) bool {
	if len(groups) == 0 {
		return true
	}
	passed := evaluateGroup(fields, groups[0])
	for i := 1; i < len(groups); i++ {
		groupResult := evaluateGroup(fields, groups[i])
		outerLogic := groups[i-1].OuterLogic
		if outerLogic == "OR" {
			passed = passed || groupResult
		} else { // AND
			if !passed || !groupResult {
				return false
			}
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
	return false
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
	if arr, ok := val.([]interface{}); ok {
		condStr := strings.ToLower(getStringValue(condValue))
		for _, item := range arr {
			if personObj, ok := item.(map[string]interface{}); ok {
				if name, ok := personObj["name"].(string); ok {
					if strings.EqualFold(name, condStr) {
						return true
					}
				}
				if email, ok := personObj["email"].(string); ok {
					if strings.EqualFold(email, condStr) {
						return true
					}
				}
			}
		}
	}
	return strings.EqualFold(getStringValue(val), getStringValue(condValue))
}

func contains(val, condValue interface{}) bool {
	condStr := getStringValue(condValue)
	if arr, ok := val.([]interface{}); ok {
		for _, item := range arr {
			if personObj, ok := item.(map[string]interface{}); ok {
				if name, ok := personObj["name"].(string); ok {
					if strings.Contains(strings.ToLower(name), strings.ToLower(condStr)) {
						return true
					}
				}
				if email, ok := personObj["email"].(string); ok {
					if strings.Contains(strings.ToLower(email), strings.ToLower(condStr)) {
						return true
					}
				}
			}
		}
	}
	valStr := getStringValue(val)
	if strings.Contains(strings.ToLower(valStr), strings.ToLower(condStr)) {
		return true
	}
	return false
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
	return getStringValue(val) < getStringValue(condValue)
}

func dateAfter(val, condValue interface{}) bool {
	return getStringValue(val) > getStringValue(condValue)
}

func EvaluateRecord(fields map[string]interface{}, filterGroups []FilterGroup) bool {
	if len(filterGroups) == 0 {
		return true
	}
	return evaluateFilter(fields, filterGroups)
}

func ExtractFieldValue(fields map[string]interface{}, fieldName string) string {
	if fieldName == "" {
		return ""
	}
	if val, ok := fields[fieldName]; ok {
		return getStringValue(val)
	}
	return ""
}

func ExtractContentFields(fields map[string]interface{}, contentFields []string) string {
	var parts []string
	for _, field := range contentFields {
		if val, ok := fields[field]; ok {
			parts = append(parts, getStringValue(val))
		}
	}
	return strings.Join(parts, "\n\n")
}
