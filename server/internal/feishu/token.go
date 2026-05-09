package feishu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func DecryptSecret(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type TokenManager struct {
	pool  *pgxpool.Pool
	cache map[string]*cachedToken
	mu    sync.RWMutex
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

func NewTokenManager(pool *pgxpool.Pool) *TokenManager {
	return &TokenManager{pool: pool, cache: make(map[string]*cachedToken)}
}

func (tm *TokenManager) GetToken(ctx context.Context, appID, appSecret string) (string, error) {
	tm.mu.RLock()
	if cached, ok := tm.cache[appID]; ok && time.Now().Before(cached.expiresAt.Add(-time.Minute)) {
		tm.mu.RUnlock()
		return cached.token, nil
	}
	tm.mu.RUnlock()

	token, expiresAt, err := tm.fetchToken(ctx, appID, appSecret)
	if err != nil {
		return "", err
	}

	tm.mu.Lock()
	tm.cache[appID] = &cachedToken{token: token, expiresAt: expiresAt}
	tm.mu.Unlock()

	return token, nil
}

func (tm *TokenManager) fetchToken(ctx context.Context, appID, appSecret string) (string, time.Time, error) {
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	body := map[string]string{"app_id": appID, "app_secret": appSecret}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 0 {
		return "", time.Time{}, fmt.Errorf("feishu token error: %s", result.Msg)
	}

	expiresAt := time.Now().Add(time.Duration(result.Expire) * time.Second)
	return result.TenantAccessToken, expiresAt, nil
}
