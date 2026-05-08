package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type BitableClient struct {
	appToken   string
	token      string
	httpClient *http.Client
}

func NewBitableClient(appToken, token string) *BitableClient {
	return &BitableClient{
		appToken:   appToken,
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (bc *BitableClient) GetRecords(ctx context.Context, pageSize int, pageToken string) (*BitableRecordsResponse, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/records?page_size=%d", bc.appToken, pageSize)
	if pageToken != "" {
		url += "&page_token=" + pageToken
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+bc.token)

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BitableRecordsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (bc *BitableClient) GetFields(ctx context.Context) ([]BitableField, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/default/fields", bc.appToken)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+bc.token)

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []BitableField `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data.Items, nil
}

type BitableField struct {
	FieldID   string `json:"field_id"`
	FieldName string `json:"field_name"`
	Type      int    `json:"type"`
}
