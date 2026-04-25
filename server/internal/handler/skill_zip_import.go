package handler

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ImportSkillZipResponse struct {
	Skills []SkillWithFilesResponse `json:"skills"`
}

func (h *Handler) ImportSkillZip(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)

	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB limit
		writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zip file: "+err.Error())
		return
	}

	tmpDir, err := os.MkdirTemp("", "skill-zip-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp directory")
		return
	}
	defer os.RemoveAll(tmpDir)

	if err := extractZip(zipReader, tmpDir); err != nil {
		writeError(w, http.StatusBadRequest, "failed to extract zip: "+err.Error())
		return
	}

	skills, err := extractSkillsFromDir(tmpDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(skills) == 0 {
		writeError(w, http.StatusBadRequest, "no valid skills found in zip file")
		return
	}

	var responses []SkillWithFilesResponse
	for _, skill := range skills {
		files := make([]CreateSkillFileRequest, 0, len(skill.Files))
		for _, f := range skill.Files {
			if !validateFilePath(f.Path) {
				continue
			}
			files = append(files, CreateSkillFileRequest{
				Path:    f.Path,
				Content: f.Content,
			})
		}

		resp, err := h.createSkillWithFiles(r.Context(), skillCreateInput{
			WorkspaceID: workspaceID,
			CreatorID:   creatorID,
			Name:        skill.Name,
			Description: skill.Description,
			Content:     skill.Content,
			Config:      map[string]any{"origin": "zip"},
			Files:       files,
		})
		if err != nil {
			if isUniqueViolation(err) {
				slog.Warn("skill zip import: skill already exists, skipping", "name", skill.Name)
				continue
			}
			slog.Warn("skill zip import: failed to create skill", "name", skill.Name, "error", err)
			continue
		}
		responses = append(responses, resp)
	}

	if len(responses) == 0 {
		writeError(w, http.StatusConflict, "no skills could be imported (all names conflict)")
		return
	}

	actorType, actorID := h.resolveActor(r, creatorID, workspaceID)
	for _, resp := range responses {
		h.publish(protocol.EventSkillCreated, workspaceID, actorType, actorID, map[string]any{"skill": resp})
	}

	writeJSON(w, http.StatusCreated, ImportSkillZipResponse{Skills: responses})
}

type extractedSkill struct {
	Name        string
	Description string
	Content     string
	Files       []extractedFile
}

type extractedFile struct {
	Path    string
	Content string
}

func extractZip(zipReader *zip.Reader, destDir string) error {
	for _, f := range zipReader.File {
		path := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(filepath.Separator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractSkillsFromDir(rootDir string) ([]extractedSkill, error) {
	rootEntries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	// Check if root has SKILL.md (single skill mode)
	for _, entry := range rootEntries {
		if entry.Name() == "SKILL.md" {
			return extractSingleSkill(rootDir, "")
		}
	}

	// Batch mode: find all subdirectories with SKILL.md
	var skills []extractedSkill
	for _, entry := range rootEntries {
		if !entry.IsDir() {
			continue
		}
		if isIgnoredDir(entry.Name()) {
			continue
		}

		skillDir := filepath.Join(rootDir, entry.Name())
		skillMdPath := filepath.Join(skillDir, "SKILL.md")

		if _, err := os.Stat(skillMdPath); err == nil {
			subSkills, err := extractSingleSkill(skillDir, entry.Name())
			if err != nil {
				slog.Warn("zip import: failed to extract skill", "dir", entry.Name(), "error", err)
				continue
			}
			skills = append(skills, subSkills...)
		}
	}

	return skills, nil
}

func extractSingleSkill(baseDir string, fallbackName string) ([]extractedSkill, error) {
	skillMdPath := filepath.Join(baseDir, "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return nil, err
	}

	name, description := parseSkillFrontmatter(string(content))
	if name == "" {
		name = fallbackName
	}
	if name == "" {
		name = filepath.Base(baseDir)
	}

	var files []extractedFile
	entries, err := os.ReadDir(baseDir)
	if err == nil {
		for _, entry := range entries {
			if entry.Name() == "SKILL.md" {
				continue
			}
			if isIgnoredFile(entry.Name()) {
				continue
			}
			if entry.IsDir() {
				subFiles, err := collectFilesRecursive(filepath.Join(baseDir, entry.Name()), entry.Name()+"/")
				if err != nil {
					slog.Warn("zip import: failed to read subdirectory", "path", entry.Name())
					continue
				}
				files = append(files, subFiles...)
				continue
			}
			fileContent, err := os.ReadFile(filepath.Join(baseDir, entry.Name()))
			if err != nil {
				continue
			}
			files = append(files, extractedFile{
				Path:    entry.Name(),
				Content: string(fileContent),
			})
		}
	}

	return []extractedSkill{{
		Name:        name,
		Description: description,
		Content:     string(content),
		Files:       files,
	}}, nil
}

func collectFilesRecursive(dir string, prefix string) ([]extractedFile, error) {
	var files []extractedFile
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if isIgnoredFile(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			subFiles, err := collectFilesRecursive(path, prefix+entry.Name()+"/")
			if err != nil {
				continue
			}
			files = append(files, subFiles...)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		files = append(files, extractedFile{
			Path:    prefix + entry.Name(),
			Content: string(content),
		})
	}
	return files, nil
}

func isIgnoredFile(name string) bool {
	nameLower := strings.ToLower(name)
	if nameLower == ".ds_store" {
		return true
	}
	if strings.HasPrefix(nameLower, "license") {
		return true
	}
	if strings.HasPrefix(nameLower, "readme") {
		return true
	}
	return false
}

func isIgnoredDir(name string) bool {
	nameLower := strings.ToLower(name)
	if nameLower == ".ds_store" {
		return true
	}
	return false
}