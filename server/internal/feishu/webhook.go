package feishu

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/util"
)

func (h *FeishuHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var event FeishuWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	eventType := event.Header.EventType
	slog.Info("feishu webhook received", "event_type", eventType)

	userID := chi.URLParam(r, "userId")
	workspaceID := chi.URLParam(r, "workspaceId")

	switch {
	case len(eventType) > 7 && eventType[:7] == "bitable":
		h.handleBitableWebhook(r.Context(), userID, workspaceID, event)
	case len(eventType) > 5 && eventType[:5] == "task.":
		h.handleTaskWebhook(r.Context(), userID, workspaceID, event)
	default:
		slog.Debug("unknown feishu event type", "event_type", eventType)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *FeishuHandler) handleBitableWebhook(ctx context.Context, userID, workspaceID string, event FeishuWebhookEvent) {
	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	h.syncService.SyncUserFeishuData(ctx, userUUID, wsUUID)
}

func (h *FeishuHandler) handleTaskWebhook(ctx context.Context, userID, workspaceID string, event FeishuWebhookEvent) {
	userUUID, _ := util.ParseUUID(userID)
	wsUUID, _ := util.ParseUUID(workspaceID)

	h.syncService.SyncUserFeishuData(ctx, userUUID, wsUUID)
}

type FeishuHandler struct {
	syncService *SyncService
}
