package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"cas-gateway/models"
)

// ProxyManager 代理管理器
type ProxyManager struct {
	proxy *httputil.ReverseProxy
	route *models.RouteConfig
}

// NewProxyManager 创建代理管理器
func NewProxyManager(route *models.RouteConfig) (*ProxyManager, error) {
	targetURL, err := url.Parse(route.Target)
	if err != nil {
		return nil, fmt.Errorf("解析目标URL失败 [%s]: %w", route.Name, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	
	// 自定义Director以修改请求
	originalDirector := proxy.Director
	target := targetURL.String()
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// 可以在这里添加自定义的请求头等
		req.Header.Set("X-Forwarded-By", "cas-gateway")
		log.Printf("[代理] 转发请求: %s %s -> %s%s", req.Method, req.URL.Path, target, req.URL.Path)
	}

	return &ProxyManager{
		proxy: proxy,
		route: route,
	}, nil
}

// GetProxy 获取代理
func (pm *ProxyManager) GetProxy() *httputil.ReverseProxy {
	return pm.proxy
}

// GetRoute 获取路由配置
func (pm *ProxyManager) GetRoute() *models.RouteConfig {
	return pm.route
}
