package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ContactClient struct {
	token      string
	httpClient *http.Client
}

func NewContactClient(token string) *ContactClient {
	return &ContactClient{
		token:      token,
		httpClient: http.DefaultClient,
	}
}

type FeishuUserInfo struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		User struct {
			Email     string `json:"email"`
			Name      string `json:"name"`
			OpenID    string `json:"open_id"`
			UserID    string `json:"user_id"`
		} `json:"user"`
	} `json:"data"`
}

func (cc *ContactClient) GetUserByOpenID(ctx context.Context, openID string) (*FeishuUserInfo, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/contact/v3/users/%s?user_id_type=open_id", openID)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cc.token)

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result FeishuUserInfo
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("feishu contact API error: %s", result.Msg)
	}

	return &result, nil
}