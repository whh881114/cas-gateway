package cas

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"cas-gateway/auth"
	"cas-gateway/config"
)

// CASProvider CAS认证提供者
type CASProvider struct {
	baseURL      string
	loginPath    string
	validatePath string
	useJSON      bool
}

// NewCASProvider 创建CAS认证提供者
func NewCASProvider() (*CASProvider, error) {
	cfg := config.AppConfig
	if cfg == nil {
		return nil, fmt.Errorf("配置未加载")
	}

	validatePath := cfg.CAS.ValidatePath
	if validatePath == "" {
		validatePath = "/p3/serviceValidate" // 默认值
	}

	loginPath := cfg.CAS.LoginPath
	if loginPath == "" {
		loginPath = "/login" // 默认值
	}

	return &CASProvider{
		baseURL:      cfg.CAS.BaseURL,
		loginPath:    loginPath,
		validatePath: validatePath,
		useJSON:      cfg.CAS.UseJSON,
	}, nil
}

// GetLoginURL 获取CAS登录URL
func (p *CASProvider) GetLoginURL(serviceURL string) string {
	loginURL := p.baseURL + p.loginPath
	u, err := url.Parse(loginURL)
	if err != nil {
		return loginURL
	}

	q := u.Query()
	q.Set("service", serviceURL)
	u.RawQuery = q.Encode()

	return u.String()
}

// ValidateTicket 验证 CAS ticket，返回用户信息（优先使用oaid）
func (p *CASProvider) ValidateTicket(ticket, serviceURL string) (*auth.UserInfo, error) {
	// 构建验证URL
	validateURL := p.baseURL + p.validatePath
	u, err := url.Parse(validateURL)
	if err != nil {
		return nil, fmt.Errorf("解析验证URL失败: %w", err)
	}

	q := u.Query()
	q.Set("ticket", ticket)
	q.Set("service", serviceURL)

	// 如果配置使用JSON格式，添加format=json参数
	if p.useJSON {
		q.Set("format", "json")
	}

	u.RawQuery = q.Encode()

	// 发送验证请求
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("验证请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 根据配置选择解析JSON或XML
	if p.useJSON {
		return p.parseJSONResponse(body)
	}
	return p.parseXMLResponse(body)
}

// ExtractTicket 从URL中提取ticket参数
func (p *CASProvider) ExtractTicket(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	ticket := u.Query().Get("ticket")
	if ticket == "" {
		return "", fmt.Errorf("URL中未找到ticket参数")
	}

	return ticket, nil
}

// IsLoginPath 判断是否为登录路径（包含ticket参数）
func (p *CASProvider) IsLoginPath(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	ticket := u.Query().Get("ticket")
	return ticket != ""
}

// BuildServiceURL 构建服务URL（用于CAS回调）
func (p *CASProvider) BuildServiceURL(req *http.Request, path string) string {
	scheme := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	host := req.Host
	if host == "" {
		host = req.Header.Get("Host")
	}

	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// parseJSONResponse 解析JSON格式的CAS响应
func (p *CASProvider) parseJSONResponse(body []byte) (*auth.UserInfo, error) {
	var jsonResp JSONServiceResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %w", err)
	}

	// 检查失败响应
	if jsonResp.ServiceResponse.AuthenticationFailure != nil {
		fail := jsonResp.ServiceResponse.AuthenticationFailure
		return nil, fmt.Errorf("CAS验证失败 [%s]: %s", fail.Code, fail.Description)
	}

	// 检查成功响应
	success := jsonResp.ServiceResponse.AuthenticationSuccess
	if success == nil {
		return nil, fmt.Errorf("CAS验证响应格式错误：未找到authenticationSuccess")
	}

	userInfo := &auth.UserInfo{
		Extra: make(map[string]interface{}),
	}

	// 优先使用oaid作为用户标识（文档要求）
	if len(success.Attributes.Oaid) > 0 {
		userInfo.Oaid = success.Attributes.Oaid[0]
	} else if success.User != "" {
		// 如果没有oaid，使用user字段作为fallback
		userInfo.Oaid = success.User
	} else {
		return nil, fmt.Errorf("CAS验证响应格式错误：未找到用户标识（oaid或user）")
	}

	// 获取员工姓名
	if len(success.Attributes.EmployeeName) > 0 {
		userInfo.EmployeeName = success.Attributes.EmployeeName[0]
	}

	return userInfo, nil
}

// parseXMLResponse 解析XML格式的CAS响应
func (p *CASProvider) parseXMLResponse(body []byte) (*auth.UserInfo, error) {
	var serviceResp ServiceResponse
	if err := xml.Unmarshal(body, &serviceResp); err != nil {
		return nil, fmt.Errorf("解析XML响应失败: %w", err)
	}

	if serviceResp.Failure != nil {
		return nil, fmt.Errorf("CAS验证失败 [%s]: %s", serviceResp.Failure.Code, serviceResp.Failure.Message)
	}

	if serviceResp.Success == nil || serviceResp.Success.User == "" {
		return nil, fmt.Errorf("CAS验证响应格式错误：未找到用户信息")
	}

	userInfo := &auth.UserInfo{
		Oaid:  serviceResp.Success.User, // XML格式使用user字段
		Extra: make(map[string]interface{}),
	}

	// XML格式的属性解析（如果后续需要）
	if serviceResp.Success.Attributes != nil {
		userInfo.EmployeeName = serviceResp.Success.Attributes.DisplayName
	}

	return userInfo, nil
}
