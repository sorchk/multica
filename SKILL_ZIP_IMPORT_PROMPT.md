# Skill ZIP 导入功能实现提示词

## 背景分析

### 现有 Skills 导入机制

**前端结构** (`packages/views/skills/`):
- `create-skill-dialog.tsx`: 创建技能对话框，支持三种方式（手动/URL导入/Runtime导入）
- `runtime-local-skill-import-panel.tsx`: 从本地 runtime 导入技能的面板
- 使用现有 UI 组件库 (`@multica/ui/components/ui/*`)

**后端结构** (`server/internal/handler/`):
- `skill.go`: 主要的技能 CRUD 和导入处理器
- `ImportSkill()` 处理 URL 导入，调用 `fetchFromClawHub()` 或 `fetchFromSkillsSh()`
- 使用 `createSkillWithFiles()` 创建技能

**API 客户端** (`packages/core/api/client.ts`):
- `importSkill({ url })`: 通过 URL 导入技能
- 需要添加 `importSkillFromZip(zipFile): Promise<Skill[]>` 方法

**数据库结构**
```sql
skill(id, workspace_id, name, description, content, config, created_by, created_at, updated_at)
skill_file(id, skill_id, path, content, created_at, updated_at)
```

---

## 功能需求

### ZIP 解析逻辑

1. **单技能导入**：压缩包根目录有 `SKILL.md`
   - 提取 `SKILL.md` 及其同目录的所有支持文件
   - 忽略：`LICENSE*`, `readme*`, `.DS_Store`

2. **批量导入**：压缩包根目录无 `SKILL.md`，但有多个子目录
   - 查找每个包含 `SKILL.md` 的子目录
   - 每个子目录作为一个独立技能导入
   - 忽略：`LICENSE*`, `readme*`, `.DS_Store`

---

## 实现步骤

### 1. 后端实现

**新增文件**: `server/internal/handler/skill_zip_import.go`

主要逻辑：
1. 解析 multipart form 上传的 zip 文件
2. 解压到临时目录
3. 递归扫描目录结构
4. 判断是单技能还是批量导入
5. 提取每个技能的文件
6. 调用 `createSkillWithFiles()` 创建技能
7. 清理临时文件

**API 端点**: `POST /api/skills/import/zip`

**关键函数**:
- `parseZipFile(r)` - 解析上传的 zip 文件
- `extractSkillsFromZip(rootDir)` - 从解压目录提取技能
- `isSingleSkillImport(rootDir)` - 判断是单技能还是批量导入
- `collectSkillFiles(dir, skillName)` - 收集技能的所有支持文件

### 2. 前端实现

**修改文件**: `packages/views/skills/components/create-skill-dialog.tsx`

1. 在 Method 类型中添加 "zip"
2. 在 MethodChooser 中添加 Zip 卡片
3. 添加 ZipForm 组件

**修改文件**: `packages/core/api/client.ts`

添加 `importSkillFromZip(zipFile: File): Promise<Skill[]>` 方法

---

## 技术要点

### Zip 处理（Go）
- 使用 `archive/zip` 包读取 zip 文件
- 使用 `os.MkdirTemp()` 创建临时目录
- 使用 `zip.OpenReader()` 读取压缩包
- 递归遍历目录结构
- 文件路径验证：`validateFilePath()`

### 前端文件上传
- 使用 HTML5 `<input type="file" accept=".zip">`
- 使用 `FormData` 构建 multipart form
- 显示文件选择和上传进度

### 错误处理
- 文件过大的情况
- ZIP 格式无效
- 压缩包中没有任何有效技能
- 技能名称冲突（409 Conflict）

---

## 现有代码参考

### UI 组件使用示例
```typescript
import { Dialog, DialogContent, DialogTitle } from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useScrollFade } from "@multica/ui/hooks/use-scroll-fade";
import { cn } from "@multica/ui/lib/utils";
```

### API 路由注册
```go
// server/cmd/server/router.go
router.Route("/api/skills", func(r chi.Router) {
    r.Post("/import/zip", h.ImportSkillZip)
})
```

### 技能创建
```go
// 使用现有的 createSkillWithFiles 函数
resp, err := h.createSkillWithFiles(ctx, skillCreateInput{
    WorkspaceID: workspaceID,
    CreatorID:   creatorID,
    Name:        name,
    Description: description,
    Content:     content,
    Config:      map[string]any{"origin": "zip"},
    Files:       files,
})
```

---

## 文件变更清单

### 新增文件
1. `server/internal/handler/skill_zip_import.go` - ZIP 导入处理器
2. `packages/views/skills/components/zip-skill-import-form.tsx` - ZIP 导入表单组件

### 修改文件
1. `server/cmd/server/router.go` - 注册新路由
2. `packages/core/api/client.ts` - 添加 `importSkillFromZip` API 方法
3. `packages/views/skills/components/create-skill-dialog.tsx` - 添加 ZIP 导入选项
4. `packages/core/types/*.ts` - 扩展类型定义（如需要）