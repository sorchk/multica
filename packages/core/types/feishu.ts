export interface FilterCondition {
  field: string;
  operator: string;
  value: string | string[] | boolean | null;
}

export interface FilterGroup {
  logic: "AND" | "OR";
  outer_logic: "AND" | "OR";
  conditions: FilterCondition[];
}

export interface FilterConfig {
  filter_groups: FilterGroup[];
}

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
  filter_config: FilterConfig;
  tasks_filter_config: FilterConfig;
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
