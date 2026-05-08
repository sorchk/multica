package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type TasksClient struct {
	token     string
	httpClient *http.Client
}

func NewTasksClient(token string) *TasksClient {
	return &TasksClient{
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (tc *TasksClient) GetTasks(ctx context.Context, pageSize int, pageToken string) (*FeishuTasksResponse, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/tasks/v1/tasks?page_size=%d", pageSize)
	if pageToken != "" {
		url += "&page_token=" + pageToken
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+tc.token)

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result FeishuTasksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}