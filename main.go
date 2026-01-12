package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"cas-gateway/auth"
	"cas-gateway/auth/cas"
	"cas-gateway/config"
	"cas-gateway/middleware"
	"cas-gateway/proxy"
)

func main() {
	// 加载配置
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("配置加载成功，服务器端口: %d", cfg.Server.Port)
	log.Printf("CAS服务器: %s", cfg.CAS.BaseURL)
	log.Printf("已配置 %d 个路由", len(cfg.Routes))

	// 创建代理管理器
	proxyManager, err := proxy.NewProxyManager(cfg.Routes)
	if err != nil {
		log.Fatalf("创建代理管理器失败: %v", err)
	}

	// 创建CAS认证提供者
	var authProvider auth.Provider
	authProvider, err = cas.NewCASProvider()
	if err != nil {
		log.Fatalf("创建CAS认证提供者失败: %v", err)
	}

	// 创建认证中间件
	authMiddleware := middleware.NewAuthMiddleware(cfg.Server.SessionKey, proxyManager, authProvider)

	// 创建HTTP处理器
	mux := http.NewServeMux()

	// 为每个路由注册代理处理器
	for _, route := range cfg.Routes {
		proxy, exists := proxyManager.GetProxy(route.Path)
		if !exists {
			log.Printf("警告: 路由 %s 的代理不存在", route.Name)
			continue
		}

		// 创建代理处理器（包含路径重写）
		handler := http.StripPrefix(route.Path, proxy)

		// 使用闭包捕获route.Path
		path := route.Path

		// 注册带尾斜杠的路径（会匹配所有子路径）
		mux.Handle(path+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[路由处理] 处理请求: %s %s (路由: %s)", r.Method, r.URL.Path, path)
			handler.ServeHTTP(w, r)
		}))

		// 注册精确路径（用于匹配路径本身）
		mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[路由处理] 处理请求: %s %s (路由: %s)", r.Method, r.URL.Path, path)
			handler.ServeHTTP(w, r)
		}))

		log.Printf("路由已注册: %s -> %s", route.Path, route.Target)
	}

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// 登出端点
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.Logout(w, r)
		// 构建登出URL（参考原代码逻辑）
		service := r.Header.Get("Origin")
		if service == "" {
			service = r.Header.Get("Referer")
		}
		if service == "" {
			scheme := "http"
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			service = fmt.Sprintf("%s://%s", scheme, r.Host)
		}
		// 提取service参数（参考原代码）
		if match := regexp.MustCompile(`\?service=(.*)`).FindStringSubmatch(service); len(match) > 1 {
			service = match[1]
		}
		logoutURL := fmt.Sprintf("%s/cas2/logout?service=%s", cfg.CAS.BaseURL, service)
		log.Printf("[登出] 重定向到: %s", logoutURL)
		http.Redirect(w, r, logoutURL, http.StatusFound)
	})

	// 添加通配符路由，处理所有未匹配的请求（如 /static/... 等）
	// 中间件会根据 Referer 或路径前缀智能匹配路由，这里只需要提供一个兜底处理
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 跳过已经注册的路径
		if r.URL.Path == "/health" || r.URL.Path == "/logout" {
			return
		}
		// 查找匹配的路由
		route, exists := proxyManager.GetRoute(r.URL.Path)
		if !exists {
			// 如果找不到路由，返回404
			log.Printf("[默认路由] 未找到匹配的路由: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		proxy, exists := proxyManager.GetProxy(route.Path)
		if !exists {
			log.Printf("[默认路由] 路由 %s 的代理不存在", route.Path)
			http.NotFound(w, r)
			return
		}
		log.Printf("[默认路由] 处理请求: %s %s -> %s", r.Method, r.URL.Path, route.Target)
		// 直接使用代理，不剥离路径前缀（因为路径可能不包含路由前缀）
		proxy.ServeHTTP(w, r)
	})

	// 应用认证中间件
	handler := authMiddleware.Handler(mux)

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("CAS Gateway 启动在端口 %d", cfg.Server.Port)
	log.Printf("访问 http://localhost%s/finops 开始使用", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
