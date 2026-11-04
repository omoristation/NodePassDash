package nodepass

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// EndpointUpdateCheck 是主控 /update/check 的返回结果。
type EndpointUpdateCheck struct {
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	HasUpdate   bool   `json:"hasUpdate"`
	Name        string `json:"name,omitempty"`
	Notes       string `json:"notes,omitempty"`
	PublishedAt string `json:"publishedAt,omitempty"`
	ReleaseURL  string `json:"releaseUrl,omitempty"`
	Asset       string `json:"asset,omitempty"`
	Supported   bool   `json:"supported"`
	Message     string `json:"message,omitempty"`
}

// EndpointUpdateStatus 是主控 /update/status 的返回结果。主控在替换二进制
// 后会自动重启，因此 phase 会经过 "restarting"，重启完成后变为 "done"。
type EndpointUpdateStatus struct {
	Running  bool   `json:"running"`
	Phase    string `json:"phase"`
	Message  string `json:"message,omitempty"`
	Current  string `json:"current"`
	Latest   string `json:"latest,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// updateClient 与 createRestyClient 等价，但把超时放宽到 30s：主控检查
// GitHub release 最多需要 20s，默认的 15s 会让面板先于主控超时。
func updateClient() *resty.Client {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	client.SetProxy("")
	client.RemoveProxy()
	return client
}

// updateRequest 与 request 相同，只是使用更长的超时，并在非 2xx 时尽量
// 取出主控返回的 error 文本。
func updateRequest(method, url, apiKey string, dest interface{}) error {
	client := updateClient()
	req := client.R().SetHeader("X-API-Key", apiKey)
	if dest != nil {
		req.SetResult(dest)
	}

	var resp *resty.Response
	var err error
	switch method {
	case "GET":
		resp, err = req.Get(url)
	case "POST":
		resp, err = req.Post(url)
	default:
		return fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}
	if err != nil {
		return err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(resp.Body(), &apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("%s", apiErr.Error)
		}
		return fmt.Errorf("NodePass API 返回错误: %d", resp.StatusCode())
	}
	return nil
}

// CheckEndpointUpdate 请求主控检查 omoristation/nodepass 的最新版本。
func CheckEndpointUpdate(endpointID int64) (*EndpointUpdateCheck, error) {
	baseURL, apiKey, exists := GetCache().Get(fmt.Sprintf("%d", endpointID))
	if !exists {
		return nil, fmt.Errorf("endpoint not found in cache")
	}

	var resp EndpointUpdateCheck
	if err := updateRequest("GET", fmt.Sprintf("%s/update/check", baseURL), apiKey, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ApplyEndpointUpdate 触发主控下载并替换自身二进制。主控立即返回并在后台
// 执行，进度通过 GetEndpointUpdateStatus 轮询。
func ApplyEndpointUpdate(endpointID int64) error {
	baseURL, apiKey, exists := GetCache().Get(fmt.Sprintf("%d", endpointID))
	if !exists {
		return fmt.Errorf("endpoint not found in cache")
	}
	return updateRequest("POST", fmt.Sprintf("%s/update/apply", baseURL), apiKey, nil)
}

// GetEndpointUpdateStatus 返回主控当前的自更新状态。
func GetEndpointUpdateStatus(endpointID int64) (*EndpointUpdateStatus, error) {
	baseURL, apiKey, exists := GetCache().Get(fmt.Sprintf("%d", endpointID))
	if !exists {
		return nil, fmt.Errorf("endpoint not found in cache")
	}

	var resp EndpointUpdateStatus
	if err := updateRequest("GET", fmt.Sprintf("%s/update/status", baseURL), apiKey, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
