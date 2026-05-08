"use client";

import React, { useState, useCallback } from "react";
import { useCurrentWorkspace } from "@multica/core/paths";
import { api } from "@multica/core/api";
import type { FeishuUserConfig, BitableField } from "@multica/core/types";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { Label } from "@multica/ui/components/ui/label";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { toast } from "sonner";
import { useT } from "../../i18n";

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
  const [fieldsLoading, setFieldsLoading] = useState(false);

  const loadFields = useCallback(async (bitableId: string) => {
    setFieldsLoading(true);
    try {
      const flds = await api.getFeishuBitableFields(bitableId);
      setFields(flds);
    } catch {
      toast.error(t(($) => $.feishu.toast_load_fields_failed));
    } finally {
      setFieldsLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    api.getFeishuConfig().then((cfg) => {
      if (cfg) {
        setConfig(cfg);
        if (cfg.bitable_id) {
          loadFields(cfg.bitable_id);
        }
      }
    });
  }, [loadFields]);

  const handleSave = async () => {
    setLoading(true);
    try {
      await api.saveFeishuConfig(config as FeishuUserConfig);
      toast.success(t(($) => $.feishu.toast_saved));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.feishu.toast_save_failed));
    } finally {
      setLoading(false);
    }
  };

  const handleSync = async () => {
    setSyncing(true);
    try {
      await api.triggerFeishuSync();
      toast.success(t(($) => $.feishu.toast_sync_started));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.feishu.toast_sync_failed));
    } finally {
      setSyncing(false);
    }
  };

  const webhookUrl = `${typeof window !== "undefined" ? window.location.origin : ""}/api/feishu/webhook/${workspace?.id}`;

  const handleBitableToggle = (checked: boolean) => {
    const sources = config.data_source?.split(",").filter(Boolean) || [];
    if (checked) {
      if (!sources.includes("bitable")) sources.push("bitable");
    } else {
      const idx = sources.indexOf("bitable");
      if (idx > -1) sources.splice(idx, 1);
    }
    setConfig({ ...config, data_source: sources.join(",") as "bitable" | "tasks" | "both" });
  };

  const handleTasksToggle = (checked: boolean) => {
    const sources = config.data_source?.split(",").filter(Boolean) || [];
    if (checked) {
      if (!sources.includes("tasks")) sources.push("tasks");
    } else {
      const idx = sources.indexOf("tasks");
      if (idx > -1) sources.splice(idx, 1);
    }
    setConfig({ ...config, data_source: sources.join(",") as "bitable" | "tasks" | "both" });
  };

  const handleContentFieldToggle = (fieldName: string, checked: boolean) => {
    const cfs = config.content_fields || [];
    if (checked) {
      if (!cfs.includes(fieldName)) cfs.push(fieldName);
    } else {
      const idx = cfs.indexOf(fieldName);
      if (idx > -1) cfs.splice(idx, 1);
    }
    setConfig({ ...config, content_fields: cfs });
  };

  const showBitableSettings = config.data_source?.includes("bitable");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t(($) => $.feishu.title)}</h2>
        <p className="text-sm text-muted-foreground mt-1">
          {t(($) => $.feishu.description)}
        </p>
      </div>

      <Card>
        <CardContent className="space-y-4 pt-4">
          <h3 className="text-sm font-medium">{t(($) => $.feishu.credentials)}</h3>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="app-id">{t(($) => $.feishu.app_id)}</Label>
              <Input
                id="app-id"
                type="text"
                value={config.app_id || ""}
                onChange={(e) => setConfig({ ...config, app_id: e.target.value })}
                placeholder={t(($) => $.feishu.app_id_placeholder)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="app-secret">{t(($) => $.feishu.app_secret)}</Label>
              <Input
                id="app-secret"
                type="password"
                value={config.app_secret || ""}
                onChange={(e) => setConfig({ ...config, app_secret: e.target.value })}
                placeholder={t(($) => $.feishu.app_secret_placeholder)}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="space-y-4 pt-4">
          <h3 className="text-sm font-medium">{t(($) => $.feishu.data_source)}</h3>
          <div className="flex gap-6">
            <div className="flex items-center gap-2">
              <Checkbox
                id="bitable-source"
                checked={config.data_source?.includes("bitable") ?? false}
                onCheckedChange={handleBitableToggle}
              />
              <Label htmlFor="bitable-source" className="font-normal cursor-pointer">
                {t(($) => $.feishu.bitable)}
              </Label>
            </div>
            <div className="flex items-center gap-2">
              <Checkbox
                id="tasks-source"
                checked={config.data_source?.includes("tasks") ?? false}
                onCheckedChange={handleTasksToggle}
              />
              <Label htmlFor="tasks-source" className="font-normal cursor-pointer">
                {t(($) => $.feishu.tasks)}
              </Label>
            </div>
          </div>
        </CardContent>
      </Card>

      {showBitableSettings && (
        <Card>
          <CardContent className="space-y-4 pt-4">
            <h3 className="text-sm font-medium">{t(($) => $.feishu.bitable_settings)}</h3>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="bitable-id">{t(($) => $.feishu.bitable_id)}</Label>
                <Input
                  id="bitable-id"
                  type="text"
                  value={config.bitable_id || ""}
                  onChange={(e) => {
                    setConfig({ ...config, bitable_id: e.target.value });
                    if (e.target.value) {
                      loadFields(e.target.value);
                    }
                  }}
                  placeholder={t(($) => $.feishu.bitable_id_placeholder)}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="title-field">{t(($) => $.feishu.title_field)}</Label>
                <Select
                  value={config.title_field || ""}
                  onValueChange={(v) => setConfig({ ...config, title_field: v || undefined })}
                >
                  <SelectTrigger id="title-field">
                    <SelectValue placeholder="--" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">--</SelectItem>
                    {fields.map((f) => (
                      <SelectItem key={f.field_id} value={f.field_name}>
                        {f.field_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="assignee-field">{t(($) => $.feishu.assignee_field)}</Label>
                <Select
                  value={config.assignee_field || ""}
                  onValueChange={(v) => setConfig({ ...config, assignee_field: v || undefined })}
                >
                  <SelectTrigger id="assignee-field">
                    <SelectValue placeholder="--" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">--</SelectItem>
                    {fields.map((f) => (
                      <SelectItem key={f.field_id} value={f.field_name}>
                        {f.field_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            {fields.length > 0 && (
              <div className="space-y-1.5">
                <Label>{t(($) => $.feishu.content_fields)}</Label>
                <div className="flex flex-wrap gap-3">
                  {fields.map((f) => (
                    <div key={f.field_id} className="flex items-center gap-1.5">
                      <Checkbox
                        id={`field-${f.field_id}`}
                        checked={config.content_fields?.includes(f.field_name) ?? false}
                        onCheckedChange={(checked) => handleContentFieldToggle(f.field_name, !!checked)}
                      />
                      <Label htmlFor={`field-${f.field_id}`} className="font-normal cursor-pointer text-sm">
                        {f.field_name}
                      </Label>
                    </div>
                  ))}
                </div>
              </div>
            )}
            {fieldsLoading && (
              <p className="text-xs text-muted-foreground">{t(($) => $.feishu.loading_fields)}</p>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="space-y-4 pt-4">
          <h3 className="text-sm font-medium">{t(($) => $.feishu.sync_settings)}</h3>
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-1.5">
              <Label htmlFor="sync-interval">{t(($) => $.feishu.sync_interval)}</Label>
              <Input
                id="sync-interval"
                type="number"
                min={1}
                value={config.sync_interval_minutes || 15}
                onChange={(e) => setConfig({ ...config, sync_interval_minutes: parseInt(e.target.value) || 15 })}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="target-type">{t(($) => $.feishu.target_type)}</Label>
              <Select
                value={config.target_type || "personal"}
                onValueChange={(v) => setConfig({ ...config, target_type: v as "project" | "personal" })}
              >
                <SelectTrigger id="target-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="personal">{t(($) => $.feishu.personal)}</SelectItem>
                  <SelectItem value="project">{t(($) => $.feishu.project)}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Switch
              id="enabled"
              checked={config.enabled ?? true}
              onCheckedChange={(checked) => setConfig({ ...config, enabled: checked })}
            />
            <Label htmlFor="enabled" className="font-normal cursor-pointer">
              {t(($) => $.feishu.enabled)}
            </Label>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="space-y-3 pt-4">
          <h3 className="text-sm font-medium">{t(($) => $.feishu.webhook)}</h3>
          <div className="p-3 bg-muted rounded font-mono text-sm break-all">
            {webhookUrl}
          </div>
          <p className="text-xs text-muted-foreground">
            {t(($) => $.feishu.webhook_hint)}
          </p>
        </CardContent>
      </Card>

      <div className="flex gap-3">
        <Button onClick={handleSave} disabled={loading}>
          {loading ? t(($) => $.feishu.saving) : t(($) => $.feishu.save)}
        </Button>
        <Button variant="outline" onClick={handleSync} disabled={syncing}>
          {syncing ? t(($) => $.feishu.syncing) : t(($) => $.feishu.sync_now)}
        </Button>
      </div>

      {config.last_sync_at && (
        <p className="text-xs text-muted-foreground">
          {t(($) => $.feishu.last_sync)}: {new Date(config.last_sync_at).toLocaleString()}
        </p>
      )}
    </div>
  );
}
