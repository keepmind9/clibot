# 网络代理配置

本指南介绍如何在受限网络环境中配置网络代理以访问 IM 平台。

## 快速开始

### 使用环境变量（最简单）

```bash
export HTTP_PROXY="http://127.0.0.1:7890"
export HTTPS_PROXY="http://127.0.0.1:7890"
clibot serve
```

### 使用配置文件

在 `config.yaml` 中添加：

```yaml
proxy:
  enabled: true
  type: "http"
  url: "http://127.0.0.1:7890"
```

## 配置优先级

clibot 使用三层回退系统：

1. **Bot 级别代理**（最高优先级）
2. **全局代理**（中等优先级）
3. **环境变量**（回退选项）

## 代理类型

### HTTP/HTTPS 代理

```yaml
proxy:
  enabled: true
  type: "http"
  url: "http://127.0.0.1:7890"
```

### SOCKS5 代理

```yaml
proxy:
  enabled: true
  type: "socks5"
  url: "socks5://127.0.0.1:1080"
```

## 认证配置

### 带认证的 HTTP 代理

```yaml
proxy:
  enabled: true
  type: "http"
  url: "http://proxy.example.com:8080"
  username: "your_username"
  password: "your_password"
```

### 带认证的 SOCKS5 代理

```yaml
proxy:
  enabled: true
  type: "socks5"
  url: "socks5://proxy.example.com:1080"
  username: "your_username"
  password: "your_password"
```

## Bot 级别代理配置

为不同的 bot 配置不同的代理：

```yaml
proxy:
  enabled: true
  type: "http"
  url: "http://127.0.0.1:8080"

bots:
  telegram:
    enabled: true
    token: "your_token"
    proxy:
      enabled: true  # 覆盖全局代理
      type: "socks5"
      url: "socks5://127.0.0.1:1080"

  discord:
    enabled: true
    token: "your_token"
    # 使用全局代理（无 bot 级别覆盖）
```

## WebSocket 支持

所有代理类型都支持 WebSocket 连接：

- **HTTP 代理**：使用 HTTP CONNECT 隧道
- **SOCKS5 代理**：原生 WebSocket 支持

使用 WebSocket 的平台：
- Discord
- 飞书/Lark
- 钉钉

## 故障排查

### 代理连接失败

1. 检查代理服务器是否运行
2. 验证代理 URL 格式
3. 手动测试代理：
   ```bash
   curl -x http://127.0.0.1:8080 https://api.telegram.org
   ```

### 认证失败

1. 验证用户名和密码
2. 检查代理服务器日志
3. 手动测试认证：
   ```bash
   curl -x http://user:pass@127.0.0.1:8080 https://api.telegram.org
   ```

### WebSocket 连接失败

1. 确保代理支持 CONNECT 方法（对于 HTTP 代理）
2. 尝试使用 SOCKS5 代理以获得更好的 WebSocket 支持
3. 检查防火墙规则

## 常用代理服务器

- **Clash**: https://github.com/Dreamacro/clash
- **V2Ray**: https://www.v2fly.org/
- **Shadowsocks**: https://shadowsocks.org/

## 配置示例

### Clash 示例

```yaml
proxy:
  enabled: true
  type: "http"
  url: "http://127.0.0.1:7890"  # Clash 默认 HTTP 端口
```

### V2Ray 示例

```yaml
proxy:
  enabled: true
  type: "socks5"
  url: "socks5://127.0.0.1:1080"  # V2Ray 默认 SOCKS5 端口
```

### 仅使用环境变量

无需修改配置：

```bash
export HTTP_PROXY="http://127.0.0.1:7890"
export HTTPS_PROXY="http://127.0.0.1:7890"
export ALL_PROXY="socks5://127.0.0.1:1080"

clibot serve --config config.yaml
```
