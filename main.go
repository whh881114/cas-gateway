package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
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
	log.Printf("路由配置: %s -> %s", cfg.Route.Path, cfg.Route.Target)

	// 创建代理管理器
	proxyManager, err := proxy.NewProxyManager(&cfg.Route)
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

	// 注册路由处理器（所有请求都转发到同一个后端）
	proxyHandler := proxyManager.GetProxy()
	route := proxyManager.GetRoute()
	
	// 如果配置了路径前缀，注册带路径前缀的路由
	if route.Path != "" && route.Path != "/" {
		// 注册带尾斜杠的路径（会匹配所有子路径）
		mux.Handle(route.Path+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[路由处理] 处理请求: %s %s (路由: %s)", r.Method, r.URL.Path, route.Path)
			// 剥离路径前缀
			r.URL.Path = strings.TrimPrefix(r.URL.Path, route.Path)
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
			proxyHandler.ServeHTTP(w, r)
		}))

		// 注册精确路径（用于匹配路径本身）
		mux.Handle(route.Path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[路由处理] 处理请求: %s %s (路由: %s)", r.Method, r.URL.Path, route.Path)
			r.URL.Path = "/"
			proxyHandler.ServeHTTP(w, r)
		}))
	}

	log.Printf("路由已注册: %s -> %s", route.Path, route.Target)

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

	// 添加通配符路由，处理所有未匹配的请求（如 /api/...、/static/... 等）
	// 所有请求都转发到同一个后端服务
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 跳过已经注册的路径
		if r.URL.Path == "/health" || r.URL.Path == "/logout" {
			return
		}
		// 如果请求路径包含路由前缀，需要剥离前缀
		if route.Path != "" && route.Path != "/" && strings.HasPrefix(r.URL.Path, route.Path) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, route.Path)
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}
		log.Printf("[默认路由] 处理请求: %s %s -> %s", r.Method, r.URL.Path, route.Target)
		proxyHandler.ServeHTTP(w, r)
	})

	// 应用认证中间件
	handler := authMiddleware.Handler(mux)

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("CAS Gateway 启动在端口 %d", cfg.Server.Port)
	if route.Path != "" && route.Path != "/" {
		log.Printf("访问 http://localhost%s%s 开始使用", addr, route.Path)
	} else {
		log.Printf("访问 http://localhost%s 开始使用", addr)
	}

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
