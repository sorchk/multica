package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type FeishuAPIError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (bc *BitableClient) doRequest(ctx context.Context, url string) (json.RawMessage, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+bc.token)

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiErr FeishuAPIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return nil, err
	}

	if apiErr.Code != 0 {
		return nil, parseFeishuError(body, apiErr)
	}

	return body, nil
}

func parseFeishuError(body []byte, apiErr FeishuAPIError) error {
	var fullResp struct {
		FeishuAPIError
		Error struct {
			PermissionViolations []struct {
				Subject string `json:"subject"`
			} `json:"permission_violations"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &fullResp); err == nil {
		if len(fullResp.Error.PermissionViolations) > 0 {
			var scopes []string
			for _, v := range fullResp.Error.PermissionViolations {
				scopes = append(scopes, v.Subject)
			}
			return fmt.Errorf("缺少飞书权限: %s，请到飞书开放平台开通", strings.Join(scopes, ", "))
		}
	}
	return fmt.Errorf("飞书 API 错误: %s", apiErr.Msg)
}

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
	tableID, err := bc.getFirstTableID(ctx)
	if err != nil {
		return nil, err
	}
	if tableID == "" {
		return nil, fmt.Errorf("no tables found in bitable")
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records?page_size=%d", bc.appToken, tableID, pageSize)
	if pageToken != "" {
		url += "&page_token=" + pageToken
	}

	body, err := bc.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var result BitableRecordsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (bc *BitableClient) GetFields(ctx context.Context) ([]BitableField, error) {
	tableID, err := bc.getFirstTableID(ctx)
	if err != nil {
		return nil, err
	}
	if tableID == "" {
		return nil, fmt.Errorf("no tables found in bitable")
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/fields", bc.appToken, tableID)

	body, err := bc.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []BitableField `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data.Items, nil
}

func (bc *BitableClient) getFirstTableID(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables", bc.appToken)

	body, err := bc.doRequest(ctx, url)
	if err != nil {
		return "", err
	}

	var result BitableTablesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Data.Items) == 0 {
		return "", nil
	}

	return result.Data.Items[0].TableID, nil
}

type BitableField struct {
	FieldID   string `json:"field_id"`
	FieldName string `json:"field_name"`
	Type      int    `json:"type"`
}
