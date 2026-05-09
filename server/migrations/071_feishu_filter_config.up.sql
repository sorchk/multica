ALTER TABLE feishu_user_configs ADD COLUMN filter_config JSONB NOT NULL DEFAULT '[]';
