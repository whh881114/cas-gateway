package middleware

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"cas-gateway/auth"
	"cas-gateway/models"
	"cas-gateway/proxy"

	"github.com/gorilla/sessions"
)

const (
	SessionName        = "cas_gateway_session"
	UserKey            = "user"
	IsAuthenticatedKey = "authenticated"
)

var (
	// staticFileRegex 静态文件扩展名正则表达式（参考原 Node.js 版本）
	staticFileRegex = regexp.MustCompile(`\.(ico|jpg|jpeg|png|gif|svg|js|css|swf|eot|ttf|otf|woff|woff2)$`)
)

// isStaticFile 判断是否为静态文件
func isStaticFile(path string) bool {
	return staticFileRegex.MatchString(path)
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	store        *sessions.CookieStore
	proxyManager *proxy.ProxyManager
	authProvider auth.Provider
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(sessionKey string, pm *proxy.ProxyManager, authProvider auth.Provider) *AuthMiddleware {
	store := sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7天
		HttpOnly: true,
		Secure:   false, // 在生产环境建议设为true（HTTPS）
		SameSite: http.SameSiteLaxMode,
	}

	return &AuthMiddleware{
		store:        store,
		proxyManager: pm,
		authProvider: authProvider,
	}
}

// Handler 认证处理函数（参考原 Node.js 版本的逻辑）
func (am *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 打印请求日志
		log.Printf("[请求] %s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

		// 特殊路径直接处理（不转发到后端）
		if r.URL.Path == "/health" || r.URL.Path == "/logout" {
			next.ServeHTTP(w, r)
			return
		}

		// 获取路由配置（检查路由是否存在）
		route, exists := am.proxyManager.GetRoute(r.URL.Path)
		
		// 如果没有匹配到路由，尝试根据 Referer 或路径前缀智能匹配
		if !exists {
			route = am.findRouteByReferer(r)
			if route != nil {
				log.Printf("[路由] 根据 Referer 匹配到路由: %s -> %s (请求: %s)", route.Path, route.Target, r.URL.Path)
			} else {
				// 如果还是找不到，尝试根据路径前缀匹配（如 /static/ 可能属于某个路由）
				route = am.findRouteByPathPrefix(r.URL.Path)
				if route != nil {
					log.Printf("[路由] 根据路径前缀匹配到路由: %s -> %s (请求: %s)", route.Path, route.Target, r.URL.Path)
				} else {
					allRoutes := am.proxyManager.GetAllRoutes()
					if len(allRoutes) > 0 {
						route = &allRoutes[0]
						log.Printf("[路由] 未匹配到路由，使用第一个路由作为默认: %s -> %s (请求: %s)", route.Path, route.Target, r.URL.Path)
					} else {
						log.Printf("[路由] 未匹配到路由且无可用路由: %s", r.URL.Path)
						next.ServeHTTP(w, r)
						return
					}
				}
			}
		} else {
			log.Printf("[路由] 匹配到路由: %s -> %s", route.Path, route.Target)
		}

		// 静态文件直接转发到后端系统，不进行认证检查（参考原 Node.js 版本的逻辑）
		if isStaticFile(r.URL.Path) {
			log.Printf("[静态文件] 直接转发: %s -> %s", r.URL.Path, route.Target)
			// 直接使用匹配到的路由的代理转发，不调用next
			proxy, exists := am.proxyManager.GetProxy(route.Path)
			if exists {
				// 对于静态文件，不剥离路径前缀，直接转发完整路径
				proxy.ServeHTTP(w, r)
			} else {
				log.Printf("[静态文件] 路由 %s 的代理不存在", route.Path)
				http.NotFound(w, r)
			}
			return
		}

		// 如果路由配置了跳过认证，直接转发（如Grafana等自带认证的系统）
		if route.SkipAuth {
			log.Printf("[认证] 路由 %s 跳过CAS认证，直接转发", route.Path)
			proxy, exists := am.proxyManager.GetProxy(route.Path)
			if exists {
				// 如果请求路径包含路由前缀，需要剥离前缀
				if strings.HasPrefix(r.URL.Path, route.Path+"/") {
					r.URL.Path = strings.TrimPrefix(r.URL.Path, route.Path)
					if r.URL.Path == "" {
						r.URL.Path = "/"
					}
				}
				proxy.ServeHTTP(w, r)
			} else {
				log.Printf("[认证] 路由 %s 的代理不存在", route.Path)
				http.NotFound(w, r)
			}
			return
		}

		// 获取session
		session, _ := am.store.Get(r, SessionName)

		// 检查是否已认证（参考原代码：检查cookie中的token）
		authenticated, ok := session.Values[IsAuthenticatedKey].(bool)
		if ok && authenticated {
			// 已认证，继续处理（参考原代码：设置请求头并转发）
			user := am.GetUser(r)
			if user != "" {
				r.Header.Set("X-User", user)
				if employeeName, ok := session.Values["employeeName"].(string); ok && employeeName != "" {
					r.Header.Set("X-Employee-Name", employeeName)
				}
			}
			log.Printf("[认证] 已认证用户: %s, 转发请求: %s", user, r.URL.Path)
			next.ServeHTTP(w, r)
			return
		}

		// 检查是否为登录回调（包含ticket）
		if am.authProvider.IsLoginPath(r.URL.String()) {
			ticket, err := am.authProvider.ExtractTicket(r.URL.String())
			if err == nil {
				// 验证ticket（使用原始请求路径构建service URL）
				serviceURL := am.authProvider.BuildServiceURL(r, r.URL.Path)
				userInfo, err := am.authProvider.ValidateTicket(ticket, serviceURL)
				if err == nil {
					// 验证成功，保存session（使用oaid作为用户标识）
					session.Values[UserKey] = userInfo.Oaid
					if userInfo.EmployeeName != "" {
						session.Values["employeeName"] = userInfo.EmployeeName
					}
					session.Values[IsAuthenticatedKey] = true
					if err := session.Save(r, w); err == nil {
						// 重定向到原始路径（去除ticket参数）
						u := *r.URL
						q := u.Query()
						q.Del("ticket")
						u.RawQuery = q.Encode()
						log.Printf("[认证] 认证成功，重定向到: %s", u.String())
						http.Redirect(w, r, u.String(), http.StatusFound)
						return
					}
				} else {
					log.Printf("[认证] Ticket验证失败: %v", err)
				}
			}
		}

		// 未认证，跳转到登录页（参考原代码逻辑）
		serviceURL := am.authProvider.BuildServiceURL(r, r.URL.Path)
		loginURL := am.authProvider.GetLoginURL(serviceURL)
		log.Printf("[认证] 未认证，跳转到登录页: %s", loginURL)
		http.Redirect(w, r, loginURL, http.StatusFound)
	})
}

// GetUser 从请求中获取当前用户
func (am *AuthMiddleware) GetUser(r *http.Request) string {
	session, _ := am.store.Get(r, SessionName)
	if user, ok := session.Values[UserKey].(string); ok {
		return user
	}
	return ""
}

// Logout 登出
func (am *AuthMiddleware) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := am.store.Get(r, SessionName)
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1
	session.Save(r, w)
}

// findRouteByReferer 根据 Referer 头查找对应的路由
func (am *AuthMiddleware) findRouteByReferer(r *http.Request) *models.RouteConfig {
	referer := r.Header.Get("Referer")
	if referer == "" {
		return nil
	}

	refererURL, err := url.Parse(referer)
	if err != nil {
		return nil
	}

	// 从 Referer 中提取路径，查找匹配的路由
	refererPath := refererURL.Path
	allRoutes := am.proxyManager.GetAllRoutes()
	
	for i := range allRoutes {
		route := &allRoutes[i]
		if strings.HasPrefix(refererPath, route.Path+"/") || refererPath == route.Path {
			return route
		}
	}

	return nil
}

// findRouteByPathPrefix 根据路径前缀查找路由（用于处理 /static/ 等路径）
func (am *AuthMiddleware) findRouteByPathPrefix(reqPath string) *models.RouteConfig {
	allRoutes := am.proxyManager.GetAllRoutes()
	
	// 找到最长匹配的路由前缀
	var matchedRoute *models.RouteConfig
	maxLen := 0
	
	for i := range allRoutes {
		route := &allRoutes[i]
		// 如果请求路径以路由路径开头，或者路由路径是请求路径的前缀
		if strings.HasPrefix(reqPath, route.Path) && len(route.Path) > maxLen {
			matchedRoute = route
			maxLen = len(route.Path)
		}
	}
	
	return matchedRoute
}
