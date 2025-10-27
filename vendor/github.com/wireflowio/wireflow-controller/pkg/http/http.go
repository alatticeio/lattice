package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

type HttpClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
	timeout time.Duration
}

type ClientOption func(*HttpClient)

func WithTimtout(timeout time.Duration) ClientOption {
	return func(c *HttpClient) {
		c.timeout = timeout
		c.client.Timeout = timeout
	}
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *HttpClient) {
		c.baseURL = baseURL
	}
}

func WithHeaders(headers map[string]string) ClientOption {
	return func(c *HttpClient) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}

		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

func WithInsecureSkipVerify() ClientOption {
	return func(c *HttpClient) {
		transport := c.client.Transport.(*http.Transport)
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

// NewHttpClient 创建新的 HTTP 客户端
func NewHttpClient(opts ...ClientOption) *HttpClient {
	client := &HttpClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		headers: make(map[string]string),
		timeout: 30 * time.Second,
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(client)
	}

	// 设置默认 Content-Type
	if _, ok := client.headers["Content-Type"]; !ok {
		client.headers["Content-Type"] = "application/json"
	}

	return client
}

// Request HTTP 请求配置
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    interface{}
	Query   map[string]string
}

// Response HTTP 响应
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Do 执行 HTTP 请求
func (c *HttpClient) Do(ctx context.Context, req *Request) (*Response, error) {
	// 构建完整 URL
	url := req.URL
	if c.baseURL != "" && url[0] == '/' {
		url = c.baseURL + url
	}

	// 添加查询参数
	if len(req.Query) > 0 {
		url = c.addQueryParams(url, req.Query)
	}

	// 准备请求体
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := c.marshalBody(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认请求头
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	// 设置自定义请求头（会覆盖默认请求头）
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// 记录请求日志
	klog.V(4).Infof("HTTP Request: %s %s", req.Method, url)

	// 执行请求
	startTime := time.Now()
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 记录响应日志
	duration := time.Since(startTime)
	klog.V(4).Infof("HTTP Response: %s %s - Status: %d, Duration: %v, Size: %d bytes",
		req.Method, url, httpResp.StatusCode, duration, len(respBody))

	response := &Response{
		StatusCode: httpResp.StatusCode,
		Body:       respBody,
		Headers:    httpResp.Header,
	}

	return response, nil
}

// Get 发送 GET 请求
func (c *HttpClient) Get(ctx context.Context, url string, query map[string]string) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodGet,
		URL:    url,
		Query:  query,
	})
}

// Post 发送 POST 请求
func (c *HttpClient) Post(ctx context.Context, url string, body interface{}) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPost,
		URL:    url,
		Body:   body,
	})
}

// Put 发送 PUT 请求
func (c *HttpClient) Put(ctx context.Context, url string, body interface{}) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPut,
		URL:    url,
		Body:   body,
	})
}

// Patch 发送 PATCH 请求
func (c *HttpClient) Patch(ctx context.Context, url string, body interface{}) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPatch,
		URL:    url,
		Body:   body,
	})
}

// Delete 发送 DELETE 请求
func (c *HttpClient) Delete(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodDelete,
		URL:    url,
	})
}

// PostJSON 发送 JSON POST 请求并解析 JSON 响应
func (c *HttpClient) PostJSON(ctx context.Context, url string, reqBody, respBody interface{}) error {
	resp, err := c.Post(ctx, url, reqBody)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	if respBody != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// GetJSON 发送 GET 请求并解析 JSON 响应
func (c *HttpClient) GetJSON(ctx context.Context, url string, query map[string]string, respBody interface{}) error {
	resp, err := c.Get(ctx, url, query)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	if respBody != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// marshalBody 序列化请求体
func (c *HttpClient) marshalBody(body interface{}) ([]byte, error) {
	switch v := body.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return json.Marshal(body)
	}
}

// addQueryParams 添加查询参数
func (c *HttpClient) addQueryParams(url string, params map[string]string) string {
	if len(params) == 0 {
		return url
	}

	separator := "?"
	if bytes.Contains([]byte(url), []byte("?")) {
		separator = "&"
	}

	var buf bytes.Buffer
	buf.WriteString(url)
	buf.WriteString(separator)

	first := true
	for k, v := range params {
		if !first {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v)
		first = false
	}

	return buf.String()
}

// IsSuccessful 检查响应是否成功（2xx 状态码）
func (r *Response) IsSuccessful() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// JSON 解析 JSON 响应体
func (r *Response) JSON(v interface{}) error {
	if len(r.Body) == 0 {
		return fmt.Errorf("empty response body")
	}
	return json.Unmarshal(r.Body, v)
}

// String 返回响应体的字符串形式
func (r *Response) String() string {
	return string(r.Body)
}
