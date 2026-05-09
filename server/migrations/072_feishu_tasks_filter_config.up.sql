ALTER TABLE feishu_user_configs ADD COLUMN tasks_filter_config JSONB NOT NULL DEFAULT '[]';
