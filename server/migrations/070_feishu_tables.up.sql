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
