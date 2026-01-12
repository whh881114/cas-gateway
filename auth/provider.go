package auth

import "net/http"

// Provider 认证提供者接口
type Provider interface {
	// GetLoginURL 获取登录URL
	GetLoginURL(serviceURL string) string

	// ValidateTicket 验证ticket，返回用户信息
	ValidateTicket(ticket, serviceURL string) (*UserInfo, error)

	// ExtractTicket 从URL中提取ticket参数
	ExtractTicket(rawURL string) (string, error)

	// IsLoginPath 判断是否为登录回调路径（包含ticket参数）
	IsLoginPath(rawURL string) bool

	// BuildServiceURL 构建服务URL（用于回调）
	BuildServiceURL(req *http.Request, path string) string
}
