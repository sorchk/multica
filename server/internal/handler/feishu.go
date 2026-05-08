package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/feishu"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type FeishuHandler struct {
	configStore  *feishu.ConfigStore
	syncService  *feishu.SyncService
	tokenManager *feishu.TokenManager
	queries      *db.Queries
}

func NewFeishuHandler(configStore *feishu.ConfigStore, syncService *feishu.SyncService, tokenManager *feishu.TokenManager, queries *db.Queries) *FeishuHandler {
	return &FeishuHandler{
		configStore:  configStore,
		syncService:  syncService,
		tokenManager: tokenManager,
		queries:      queries,
	}
}

func (h *FeishuHandler) resolveWorkspaceID(r *http.Request) string {
	return middleware.ResolveWorkspaceIDFromRequest(r, h.queries)
}

func encrypt(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(plaintext))
}

func decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (h *FeishuHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)

	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	cfg, err := h.configStore.GetByUserAndWorkspace(r.Context(), userUUID, wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	cfg.AppSecretEncrypted = "***"
	writeJSON(w, http.StatusOK, cfg)
}

func (h *FeishuHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)

	var req struct {
		AppID               string   `json:"app_id"`
		AppSecret           string   `json:"app_secret"`
		DataSource          string   `json:"data_source"`
		BitableID           string   `json:"bitable_id"`
		TitleField          string   `json:"title_field"`
		AssigneeField       string   `json:"assignee_field"`
		ContentFields       []string `json:"content_fields"`
		TargetType          string   `json:"target_type"`
		TargetProjectID     string   `json:"target_project_id"`
		SyncIntervalMinutes int      `json:"sync_interval_minutes"`
		Enabled             bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	encryptedSecret := encrypt(req.AppSecret)

	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	var projectID *pgtype.UUID
	if req.TargetProjectID != "" {
		pid, _ := util.ParseUUID(req.TargetProjectID)
		projectID = &pid
	}

	cfg := &feishu.FeishuUserConfig{
		UserID:              userUUID,
		WorkspaceID:         wsUUID,
		AppID:               req.AppID,
		AppSecretEncrypted:  encryptedSecret,
		DataSource:          req.DataSource,
		TargetType:          req.TargetType,
		TargetProjectID:     projectID,
		SyncIntervalMinutes: req.SyncIntervalMinutes,
		Enabled:             req.Enabled,
	}

	if req.BitableID != "" {
		cfg.BitableID = &req.BitableID
	}
	if req.TitleField != "" {
		cfg.TitleField = &req.TitleField
	}
	if req.AssigneeField != "" {
		cfg.AssigneeField = &req.AssigneeField
	}
	cfg.ContentFields = req.ContentFields

	if err := h.configStore.Upsert(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FeishuHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)

	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	if err := h.configStore.Delete(r.Context(), userUUID, wsUUID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FeishuHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)

	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	go h.syncService.SyncUserFeishuData(context.Background(), userUUID, wsUUID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "sync started"})
}

func (h *FeishuHandler) GetBitableFields(w http.ResponseWriter, r *http.Request) {
	bitableID := chi.URLParam(r, "bitableId")
	if bitableID == "" {
		writeError(w, http.StatusBadRequest, "bitable_id required")
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)

	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	cfg, err := h.configStore.GetByUserAndWorkspace(r.Context(), userUUID, wsUUID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusBadRequest, "feishu not configured")
		return
	}

	token, err := h.tokenManager.GetToken(r.Context(), cfg.AppID, cfg.AppSecretEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get token: "+err.Error())
		return
	}

	bitable := feishu.NewBitableClient(bitableID, token)

	fields, err := bitable.GetFields(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, fields)
}

func (h *FeishuHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Implementation in webhook.go
}
