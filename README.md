# CAS Gateway

基于 Go 的 CAS 单点登录网关代理服务，用于代理内部系统并统一进行 CAS 认证。

## 功能特性

- 🔐 集成 CAS 单点登录系统
- 🔄 反向代理多个后端服务
- 🛡️ 统一的 CAS 认证中间件
- 📦 可扩展的路由配置
- 💾 Session 会话管理

## 快速开始

### 配置

复制 `config.example.yaml` 为 `config.yaml` 并修改配置：

```yaml
server:
  port: 8080
  session_key: "your-secret-key-here" # 32字节密钥

cas:
  base_url: "https://cas.example.com/"
  login_path: "/login"              # 可选，默认为 "/login"
  validate_path: "/p3/serviceValidate"  # 可选，默认为 "/p3/serviceValidate"
  use_json: true  # 使用JSON格式（添加format=json参数），推荐使用

routes:
  - name: finops
    path: "/finops"
    target: "http://127.0.0.1:8000"
  - name: grafana
    path: "/grafana"
    target: "http://127.0.0.1:3000"
```

#### 配置说明

**`server`** - 服务器配置
- `port`: 服务监听端口
- `session_key`: 会话加密密钥（必须至少 32 字节）

**`cas`** - CAS 认证配置
- `base_url`: CAS 服务器基础 URL（必须以 `/` 结尾）
- `login_path`: CAS 登录路径，默认为 `/login`
- `validate_path`: CAS ticket 验证路径，默认为 `/p3/serviceValidate`
- `use_json`: 是否使用 JSON 格式验证（推荐启用）

**`routes`** - 路由配置列表
- `name`: 路由名称（用于日志标识）
- `path`: 路由路径前缀（如 `/finops`）
- `target`: 后端服务目标地址

**`session_key` 生成方式**：
```bash
# Linux/Mac
openssl rand -base64 32

# Windows PowerShell
[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))

# Python
python -c "import secrets; print(secrets.token_urlsafe(32))"
```

**安全提示**：
- 生产环境务必使用强随机密钥
- 不要将真实密钥提交到代码仓库
- 多个服务器实例应使用相同的 `session_key` 以共享会话

### 运行

```bash
go mod download
go run main.go
```

### 构建

```bash
go build -o cas-gateway main.go
./cas-gateway
```

## 项目结构

```
.
├── main.go              # 程序入口
├── config/              # 配置管理
│   └── config.go
├── auth/                # 认证模块
│   ├── provider.go      # 认证提供者接口
│   └── cas/             # CAS 认证实现
│       ├── cas_provider.go
│       └── types.go
├── proxy/               # 反向代理
│   └── proxy.go
├── middleware/          # 中间件
│   └── auth.go
└── models/              # 数据模型
    └── config.go
```

## 扩展路由

在 `config.yaml` 中添加新的路由配置即可：

```yaml
routes:
  - name: prometheus
    path: "/prometheus"
    target: "http://127.0.0.1:9090"
```

## License

MIT
