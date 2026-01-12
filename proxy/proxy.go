package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"cas-gateway/models"
)

// ProxyManager 代理管理器
type ProxyManager struct {
	proxies map[string]*httputil.ReverseProxy
	routes  []models.RouteConfig
}

// NewProxyManager 创建代理管理器
func NewProxyManager(routes []models.RouteConfig) (*ProxyManager, error) {
	pm := &ProxyManager{
		proxies: make(map[string]*httputil.ReverseProxy),
		routes:  routes,
	}

	// 为每个路由创建反向代理
	for _, route := range routes {
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

		pm.proxies[route.Path] = proxy
	}

	return pm, nil
}

// GetProxy 获取指定路径的代理
func (pm *ProxyManager) GetProxy(path string) (*httputil.ReverseProxy, bool) {
	proxy, exists := pm.proxies[path]
	return proxy, exists
}

// GetRoute 根据请求路径获取路由配置
func (pm *ProxyManager) GetRoute(reqPath string) (*models.RouteConfig, bool) {
	// 找到最长匹配的路径
	var matchedRoute *models.RouteConfig
	maxLen := 0

	for i := range pm.routes {
		route := &pm.routes[i]
		if len(route.Path) > maxLen && 
		   (reqPath == route.Path || strings.HasPrefix(reqPath, route.Path+"/")) {
			matchedRoute = route
			maxLen = len(route.Path)
		}
	}

	if matchedRoute != nil {
		return matchedRoute, true
	}

	return nil, false
}

// GetAllRoutes 获取所有路由配置
func (pm *ProxyManager) GetAllRoutes() []models.RouteConfig {
	return pm.routes
}
