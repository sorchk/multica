"use client";

import React, { useState, useCallback } from "react";
import { useCurrentWorkspace } from "@multica/core/paths";
import { api } from "@multica/core/api";
import type { FeishuUserConfig, BitableField, FilterGroup, FilterCondition } from "@multica/core/types";
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { HelpCircle } from "lucide-react";
import { toast } from "sonner";
import { useT } from "../../i18n";

const OPERATORS_BY_TYPE: Record<number, { label: string; value: string }[]> = {
  1: [
    { label: "等于", value: "equals" },
    { label: "不等于", value: "not_equals" },
    { label: "包含", value: "contains" },
    { label: "不包含", value: "not_contains" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
  2: [
    { label: "等于", value: "equals" },
    { label: "不等于", value: "not_equals" },
    { label: "大于", value: "greater_than" },
    { label: "小于", value: "less_than" },
    { label: "大于等于", value: "greater_or_equal" },
    { label: "小于等于", value: "less_or_equal" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
  3: [
    { label: "等于", value: "equals" },
    { label: "不等于", value: "not_equals" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
  4: [
    { label: "包含任一", value: "contains_any" },
    { label: "包含全部", value: "contains_all" },
    { label: "不包含任一", value: "not_contains_any" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
  5: [
    { label: "等于", value: "equals" },
    { label: "早于", value: "before" },
    { label: "晚于", value: "after" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
  7: [
    { label: "是", value: "is_checked" },
    { label: "否", value: "is_not_checked" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
  11: [
    { label: "等于", value: "equals" },
    { label: "不等于", value: "not_equals" },
    { label: "为空", value: "is_empty" },
    { label: "不为空", value: "is_not_empty" },
  ],
};

const NO_VALUE_OPERATORS = ["is_empty", "is_not_empty", "is_checked", "is_not_checked"];

export function FeishuTab() {
  const { t } = useT("settings");
  const workspace = useCurrentWorkspace();

  const [config, setConfig] = useState<Partial<FeishuUserConfig>>({
    data_source: "bitable",
    target_type: "personal",
    sync_interval_minutes: 15,
    content_fields: [],
    enabled: true,
    filter_config: { filter_groups: [] },
  });
  const [fields, setFields] = useState<BitableField[]>([]);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [fieldsLoading, setFieldsLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewData, setPreviewData] = useState<{title: string; assignee: string; content: string}[]>([]);
  const [helpOpen, setHelpOpen] = useState(false);

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
        if (!cfg.filter_config) {
          cfg.filter_config = { filter_groups: [] };
        }
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

  const handleTest = async () => {
    if (!config.app_id || !config.app_secret) {
      toast.error(t(($) => $.feishu.toast_test_failed_no_creds));
      return;
    }
    setTesting(true);
    try {
      const testConfig = { ...config, data_source: "bitable", bitable_id: "test" } as FeishuUserConfig;
      await api.saveFeishuConfig(testConfig);
      const flds = await api.getFeishuBitableFields(config.bitable_id || "test");
      if (flds !== null) {
        toast.success(t(($) => $.feishu.toast_test_success));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.feishu.toast_test_failed));
    } finally {
      setTesting(false);
    }
  };

  const extractFieldValue = (val: unknown): string => {
    if (val === null || val === undefined) return "";
    if (typeof val === "string") return val;
    if (typeof val === "number") return String(val);
    if (Array.isArray(val)) {
      return val.map((v) => extractFieldValue(v)).filter(Boolean).join(", ");
    }
    if (typeof val === "object") {
      const obj = val as Record<string, unknown>;
      if (obj.name) return String(obj.name);
      return JSON.stringify(val);
    }
    return String(val);
  };

  const handlePreview = async () => {
    if (!config.bitable_id) {
      toast.error(t(($) => $.feishu.toast_preview_failed_no_bitable));
      return;
    }
    setPreviewLoading(true);
    setPreviewOpen(true);
    try {
      const records = await api.getFeishuBitableRecords(config.bitable_id);
      const processed = records.map((r: {record_id: string; fields: Record<string, unknown>}) => ({
        title: config.title_field ? extractFieldValue(r.fields[config.title_field]) : "",
        assignee: config.assignee_field ? extractFieldValue(r.fields[config.assignee_field]) : "",
        content: (config.content_fields || []).map((f: string) => extractFieldValue(r.fields[f])).filter(Boolean).join("\n\n"),
      }));
      setPreviewData(processed);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.feishu.toast_preview_failed));
      setPreviewOpen(false);
    } finally {
      setPreviewLoading(false);
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

  const addFilterGroup = () => {
    const groups = config.filter_config?.filter_groups || [];
    setConfig({
      ...config,
      filter_config: {
        filter_groups: [
          ...groups,
          { logic: "AND" as const, conditions: [] },
        ],
      },
    });
  };

  const removeFilterGroup = (groupIndex: number) => {
    const groups = config.filter_config?.filter_groups || [];
    setConfig({
      ...config,
      filter_config: {
        filter_groups: groups.filter((_, i) => i !== groupIndex),
      },
    });
  };

  const updateFilterGroup = (groupIndex: number, updates: Partial<FilterGroup>) => {
    const groups = config.filter_config?.filter_groups || [];
    const newGroups = [...groups];
    newGroups[groupIndex] = { ...newGroups[groupIndex], ...updates };
    setConfig({
      ...config,
      filter_config: { filter_groups: newGroups },
    });
  };

  const addFilterCondition = (groupIndex: number) => {
    const groups = config.filter_config?.filter_groups || [];
    const newConditions = [
      ...(groups[groupIndex]?.conditions || []),
      { field: "", operator: "equals", value: "" },
    ];
    updateFilterGroup(groupIndex, { conditions: newConditions });
  };

  const removeFilterCondition = (groupIndex: number, condIndex: number) => {
    const groups = config.filter_config?.filter_groups || [];
    const group = groups[groupIndex];
    if (!group) return;
    setConfig({
      ...config,
      filter_config: {
        filter_groups: groups.map((g, i) =>
          i === groupIndex
            ? { ...g, conditions: g.conditions.filter((_, j) => j !== condIndex) }
            : g
        ),
      },
    });
  };

  const updateFilterCondition = (groupIndex: number, condIndex: number, updates: Partial<FilterCondition>) => {
    const groups = config.filter_config?.filter_groups || [];
    setConfig({
      ...config,
      filter_config: {
        filter_groups: groups.map((g, i) =>
          i === groupIndex
            ? {
                ...g,
                conditions: g.conditions.map((c, j) =>
                  j === condIndex ? { ...c, ...updates } : c
                ),
              }
            : g
        ),
      },
    });
  };

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
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">{t(($) => $.feishu.title)}</h2>
          <p className="text-sm text-muted-foreground mt-1">
            {t(($) => $.feishu.description)}
          </p>
        </div>
        <Button variant="ghost" size="icon" onClick={() => setHelpOpen(true)}>
          <HelpCircle className="h-4 w-4" />
        </Button>
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
          <div className="flex justify-end">
            <Button
              variant="outline"
              size="sm"
              onClick={handleTest}
              disabled={testing || !config.app_id || !config.app_secret}
            >
              {testing ? t(($) => $.feishu.testing) : t(($) => $.feishu.test_credentials)}
            </Button>
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
                    {fields?.map((f) => (
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
                    {fields?.map((f) => (
                      <SelectItem key={f.field_id} value={f.field_name}>
                        {f.field_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            {fields && fields.length > 0 && (
              <div className="space-y-1.5">
                <Label>{t(($) => $.feishu.content_fields)}</Label>
                <div className="flex flex-wrap gap-3">
                  {fields?.map((f) => (
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
            {config.bitable_id && fields && fields.length > 0 && (
              <Button variant="outline" size="sm" onClick={handlePreview}>
                {t(($) => $.feishu.preview_tasks)}
              </Button>
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

      {showBitableSettings && (
        <Card>
          <CardContent className="space-y-4 pt-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium">{t(($) => $.feishu.filter_conditions)}</h3>
              <Button variant="outline" size="sm" onClick={addFilterGroup}>
                {t(($) => $.feishu.add_filter_group)}
              </Button>
            </div>

            {(config.filter_config?.filter_groups || []).length === 0 && (
              <p className="text-xs text-muted-foreground">{t(($) => $.feishu.filter_conditions_hint)}</p>
            )}

            {(config.filter_config?.filter_groups || []).map((group, groupIndex) => (
              <div key={groupIndex}>
                {groupIndex > 0 && (
                  <div className="flex items-center justify-center py-2">
                    <Select
                      value={(config.filter_config?.filter_groups?.[groupIndex - 1]?.logic || "AND")}
                      onValueChange={(v) => {
                        const groups = config.filter_config?.filter_groups || [];
                        const newGroups = [...groups];
                        newGroups[groupIndex - 1] = { ...newGroups[groupIndex - 1], logic: v as "AND" | "OR" };
                        setConfig({ ...config, filter_config: { filter_groups: newGroups } });
                      }}
                    >
                      <SelectTrigger className="w-20">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="AND">AND</SelectItem>
                        <SelectItem value="OR">OR</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}

                <div className="border rounded-lg p-3 space-y-3">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">{t(($) => $.feishu.filter_group_logic)}</span>
                    <Select
                      value={group.logic}
                      onValueChange={(v) => updateFilterGroup(groupIndex, { logic: v as "AND" | "OR" })}
                    >
                      <SelectTrigger className="w-32">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="AND">{t(($) => $.feishu.filter_and)}</SelectItem>
                        <SelectItem value="OR">{t(($) => $.feishu.filter_or)}</SelectItem>
                      </SelectContent>
                    </Select>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="ml-auto text-xs text-destructive"
                      onClick={() => removeFilterGroup(groupIndex)}
                    >
                      {t(($) => $.feishu.delete_group)}
                    </Button>
                  </div>

                  {group.conditions.map((cond, condIndex) => {
                    const selectedField = fields?.find((f) => f.field_name === cond.field);
                    const fieldType = selectedField?.type || 1;
                    const operators = OPERATORS_BY_TYPE[fieldType] || OPERATORS_BY_TYPE[1];
                    const needsValue = !NO_VALUE_OPERATORS.includes(cond.operator);

                    return (
                      <div key={condIndex} className="flex items-center gap-2">
                        <Select
                          value={cond.field}
                          onValueChange={(v) => {
                            const f = fields?.find((field) => field.field_name === v);
                            const newType = f?.type || 1;
                            const newOps = OPERATORS_BY_TYPE[newType] || OPERATORS_BY_TYPE[1];
                            const newOp = newOps.some((o) => o.value === cond.operator) ? cond.operator : newOps[0].value;
                            updateFilterCondition(groupIndex, condIndex, { field: v, operator: newOp, value: needsValue ? cond.value : "" });
                          }}
                        >
                          <SelectTrigger className="flex-1">
                            <SelectValue placeholder={t(($) => $.feishu.select_field)} />
                          </SelectTrigger>
                          <SelectContent>
                            {fields?.map((f) => (
                              <SelectItem key={f.field_id} value={f.field_name}>
                                {f.field_name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>

                        <Select
                          value={cond.operator}
                          onValueChange={(v) => updateFilterCondition(groupIndex, condIndex, { operator: v })}
                        >
                          <SelectTrigger className="w-32">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {operators.map((op) => (
                              <SelectItem key={op.value} value={op.value}>
                                {op.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>

                        {needsValue && (
                          fieldType === 5 ? (
                            <Input
                              type="date"
                              className="flex-1"
                              value={cond.value as string || ""}
                              onChange={(e) => updateFilterCondition(groupIndex, condIndex, { value: e.target.value })}
                            />
                          ) : fieldType === 7 ? (
                            <Select
                              value={cond.value as string || "true"}
                              onValueChange={(v) => updateFilterCondition(groupIndex, condIndex, { value: v === "true" })}
                            >
                              <SelectTrigger className="flex-1">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="true">{t(($) => $.feishu.yes)}</SelectItem>
                                <SelectItem value="false">{t(($) => $.feishu.no)}</SelectItem>
                              </SelectContent>
                            </Select>
                          ) : (
                            <Input
                              className="flex-1"
                              value={cond.value as string || ""}
                              onChange={(e) => updateFilterCondition(groupIndex, condIndex, { value: e.target.value })}
                              placeholder={t(($) => $.feishu.enter_value)}
                            />
                          )
                        )}

                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-destructive shrink-0"
                          onClick={() => removeFilterCondition(groupIndex, condIndex)}
                        >
                          <span className="text-xs">✕</span>
                        </Button>
                      </div>
                    );
                  })}

                  <Button
                    variant="outline"
                    size="sm"
                    className="text-xs"
                    onClick={() => addFilterCondition(groupIndex)}
                  >
                    {t(($) => $.feishu.add_condition)}
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

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

      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t(($) => $.feishu.preview_title)}</DialogTitle>
          </DialogHeader>
          {previewLoading ? (
            <p className="text-sm text-muted-foreground">{t(($) => $.feishu.loading_preview)}</p>
          ) : previewData.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t(($) => $.feishu.no_records)}</p>
          ) : (
            <div className="space-y-4">
              {previewData.map((item, idx) => (
                <Card key={idx}>
                  <CardContent className="space-y-2 pt-4">
                    <div className="font-medium">{item.title || "(无标题)"}</div>
                    {item.assignee && (
                      <p className="text-xs text-muted-foreground">
                        {t(($) => $.feishu.assignee)}: {item.assignee}
                      </p>
                    )}
                    {item.content && (
                      <p className="text-sm whitespace-pre-wrap">{item.content}</p>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={helpOpen} onOpenChange={setHelpOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t(($) => $.feishu.help_title)}</DialogTitle>
          </DialogHeader>
          <div className="space-y-6 text-sm">
            <section>
              <h4 className="font-medium mb-2">{t(($) => $.feishu.help_permissions_title)}</h4>
              <p className="text-muted-foreground mb-2">{t(($) => $.feishu.help_permissions_desc)}</p>
              <ul className="list-disc list-inside space-y-1 text-muted-foreground">
                <li><code>bitable:app</code> - {t(($) => $.feishu.help_perm_bitable)}</li>
                <li><code>task:app:readonly</code> - {t(($) => $.feishu.help_perm_task)}</li>
              </ul>
            </section>

            <section>
              <h4 className="font-medium mb-2">{t(($) => $.feishu.help_app_id_title)}</h4>
              <p className="text-muted-foreground">{t(($) => $.feishu.help_app_id_desc)}</p>
            </section>

            <section>
              <h4 className="font-medium mb-2">{t(($) => $.feishu.help_app_secret_title)}</h4>
              <p className="text-muted-foreground">{t(($) => $.feishu.help_app_secret_desc)}</p>
            </section>

            <section>
              <h4 className="font-medium mb-2">{t(($) => $.feishu.help_bitable_id_title)}</h4>
              <p className="text-muted-foreground">{t(($) => $.feishu.help_bitable_id_desc)}</p>
              <p className="text-muted-foreground mt-1">
                <code>https://xxx.feishu.cn/base/</code><span className="bg-muted px-1">L6X0bz3awasV8ssuSxMcN00hn6b</span><code>?table=</code><span className="bg-muted px-1">tblxxx</span>
              </p>
            </section>

            <section>
              <h4 className="font-medium mb-2">{t(($) => $.feishu.help_field_mapping_title)}</h4>
              <p className="text-muted-foreground">{t(($) => $.feishu.help_field_mapping_desc)}</p>
            </section>

            <section>
              <h4 className="font-medium mb-2">{t(($) => $.feishu.help_webhook_title)}</h4>
              <p className="text-muted-foreground">{t(($) => $.feishu.help_webhook_desc)}</p>
            </section>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
