"use client";

import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AlertCircle, Archive, CheckCircle2, FileArchive, Loader2, Upload } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import type { Skill } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  skillDetailOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { useScrollFade } from "@multica/ui/hooks/use-scroll-fade";
import { cn } from "@multica/ui/lib/utils";

interface ImportedSkillInfo {
  id: string;
  name: string;
}

function seedAfterImport(
  qc: ReturnType<typeof useQueryClient>,
  wsId: string,
  skills: Skill[],
) {
  for (const skill of skills) {
    qc.setQueryData(skillDetailOptions(wsId, skill.id).queryKey, skill);
  }
  qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
  qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
}

interface ZipFormProps {
  onImported: (skill: Skill) => void;
  onCancel: () => void;
}

export function ZipSkillImportForm({ onImported, onCancel }: ZipFormProps) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState("");
  const [importedSkills, setImportedSkills] = useState<ImportedSkillInfo[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const fadeStyle = useScrollFade(scrollRef);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] ?? null;
    setSelectedFile(file);
    setError("");
    setImportedSkills([]);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    const file = e.dataTransfer.files?.[0] ?? null;
    if (file && file.name.endsWith(".zip")) {
      setSelectedFile(file);
      setError("");
      setImportedSkills([]);
    } else {
      setError("Please drop a valid ZIP file");
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  const handleImport = async () => {
    if (!selectedFile) return;
    setImporting(true);
    setError("");
    try {
      const skills = await api.importSkillFromZip(selectedFile);
      setImportedSkills(skills.map((s) => ({ id: s.id, name: s.name })));
      seedAfterImport(qc, wsId, skills);
      toast.success(`Imported ${skills.length} skill${skills.length === 1 ? "" : "s"}`);
      if (skills.length > 0) {
        onImported(skills[0]!);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Import failed");
    } finally {
      setImporting(false);
    }
  };

  const canImport = !!selectedFile && !importing;

  return (
    <>
      <div
        ref={scrollRef}
        style={fadeStyle}
        className="flex-1 min-h-0 space-y-4 overflow-y-auto px-5 py-4"
      >
        <div
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onClick={() => fileInputRef.current?.click()}
          className={cn(
            "relative flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-8 cursor-pointer transition-colors",
            selectedFile
              ? "border-primary bg-primary/5"
              : "border-muted-foreground/25 hover:border-primary/40 hover:bg-accent/40",
          )}
        >
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip"
            onChange={handleFileChange}
            className="hidden"
          />
          {selectedFile ? (
            <>
              <FileArchive className="h-10 w-10 text-primary mb-3" />
              <p className="text-sm font-medium">{selectedFile.name}</p>
              <p className="text-xs text-muted-foreground mt-1">
                {(selectedFile.size / 1024 / 1024).toFixed(2)} MB
              </p>
            </>
          ) : (
            <>
              <Upload className="h-10 w-10 text-muted-foreground mb-3" />
              <p className="text-sm text-muted-foreground">
                Drop a ZIP file here or click to browse
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                Supports single skill and batch import
              </p>
            </>
          )}
        </div>

        <div className="rounded-md bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
          <p className="font-medium text-foreground">Import modes:</p>
          <ul className="mt-1 list-disc list-inside space-y-0.5">
            <li>
              <strong>Single skill:</strong> ZIP root contains SKILL.md
            </li>
            <li>
              <strong>Batch import:</strong> Subdirectories each contain a
              SKILL.md
            </li>
          </ul>
          <p className="mt-2">
            Ignored files: LICENSE*, README*, .DS_Store
          </p>
        </div>

        {importedSkills.length > 0 && (
          <div className="rounded-md bg-green-500/10 px-3 py-2 text-xs text-green-600 dark:text-green-400">
            <div className="flex items-center gap-2 font-medium">
              <CheckCircle2 className="h-3.5 w-3.5" />
              Successfully imported {importedSkills.length} skill
              {importedSkills.length === 1 ? "" : "s"}
            </div>
            <ul className="mt-1 list-disc list-inside">
              {importedSkills.map((s) => (
                <li key={s.id}>{s.name}</li>
              ))}
            </ul>
          </div>
        )}

        {error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive"
          >
            <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onCancel}
          disabled={importing}
        >
          {importedSkills.length > 0 ? "Close" : "Cancel"}
        </Button>
        {importedSkills.length === 0 && (
          <Button
            type="button"
            size="sm"
            onClick={handleImport}
            disabled={!canImport}
          >
            {importing ? (
              <>
                <Loader2 className="h-3 w-3 animate-spin" />
                Importing…
              </>
            ) : (
              <>
                <Archive className="h-3 w-3" />
                Import ZIP
              </>
            )}
          </Button>
        )}
      </div>
    </>
  );
}