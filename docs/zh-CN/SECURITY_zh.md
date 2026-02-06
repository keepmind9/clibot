# 安全策略

## 支持的版本

目前仅最新版本的 clibot 会收到安全更新。

| 版本   | 支持状态            |
| ------ | ------------------- |
| 最新版 | :white_check_mark: |

## 报告漏洞

**如果您发现安全漏洞，请勿公开提 Issue。**

请发送邮件至：**security@keepmind9.com**

请包含：
- 漏洞描述
- 复现步骤
- 潜在影响评估
- 建议的缓解方案（如有）

### 后续流程

1. **确认**：我们会在 48 小时内确认收到您的报告
2. **调查**：我们将调查漏洞并确定其严重程度
3. **修复**：我们将修复漏洞并与您协调披露时间
4. **披露**：我们将公告安全修复并发布补丁版本

我们的目标是在确认后 7 天内解决关键安全问题。

## 安全最佳实践

### 部署安全

clibot 是一个可远程执行代码的强大工具。**请遵循以下安全准则：**

#### 1. 启用白名单（必需）
```yaml
security:
  whitelist_enabled: true  # 必须为 true
  allowed_users:
    discord:
      - "your-user-id-1"
      - "your-user-id-2"
```

**切勿在生产环境中使用 `whitelist_enabled: false`。**

#### 2. 使用环境变量存储密钥
```yaml
bots:
  discord:
    token: "${DISCORD_TOKEN}"  # 使用环境变量
```

通过环境变量设置密钥：
```bash
export DISCORD_TOKEN="your-secret-token"
clibot serve
```

#### 3. 限制 Bot 权限
- 为 Bot 使用最小必需权限
- 除非绝对必要，不要授予管理员权限
- 定期在 IM 平台审计 Bot 权限

#### 4. 限制动态会话数量
```yaml
session:
  max_dynamic_sessions: 50  # 合理的限制
```

#### 5. 以非 Root 用户运行
```bash
# 创建专用用户
useradd -r -s /bin/false clibot

# 以 clibot 用户运行
sudo -u clibot clibot serve
```

#### 6. 使用防火墙规则
仅暴露必要的端口：
```bash
# 仅允许本地连接（如果使用反向代理）
iptables -A INPUT -p tcp --dport 8080 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
```

#### 7. 启用日志和监控
```yaml
logging:
  level: info
  file: /var/log/clibot/clibot.log
  enable_stdout: true
```

定期审查日志以发现可疑活动。

### 用户安全

#### 1. 了解风险
clibot 可执行远程代码。任何能访问您 Bot 的人都可以：
- 执行任意命令
- 读写工作目录中的文件
- 访问系统资源
- 修改数据

**仅将可信用户加入白名单。**

#### 2. 使用专用工作目录
```yaml
sessions:
  - name: "my-session"
    work_dir: "/home/user/sandbox"  # 隔离目录
```

避免使用：
- `/` （根目录）
- `/home` （所有用户数据）
- 系统目录（`/etc`、`/var` 等）

#### 3. 定期审查白名单
定期审计您的 `allowed_users` 列表，移除不再需要访问权限的用户。

#### 4. 监控使用情况
定期检查日志中的：
- 异常命令模式
- 来自意外用户的访问
- 授权失败尝试

### 开发安全

#### 1. 保持依赖更新
```bash
go get -u ./...
go mod tidy
```

#### 2. 使用 Go 的安全特性
- 始终验证用户输入
- 使用 `context.Context` 设置超时
- 避免 `eval` 类操作
- 清理文件路径

#### 3. 遵循安全最佳实践
- 永不记录敏感数据（token、密码）
- 使用安全的随机数生成
- 为 API 端点实现速率限制
- 验证所有外部输入

## 安全功能

### 内置保护

clibot 包含多项安全功能：

- **白名单强制执行**：仅授权用户可交互
- **管理员分离**：敏感操作需要管理员权限
- **会话隔离**：每个会话在独立的 tmux 会话中运行
- **输入验证**：所有用户输入在执行前都会验证

### 审计追踪

所有操作都会记录：
- 时间戳
- 用户身份（平台、用户 ID）
- 执行的命令
- 结果

启用详细日志以进行安全审计：
```yaml
logging:
  level: debug
```

## 负责任的披露

我们感谢负责任的披露，并将：

- 全程让您了解进度
- 在安全公告中致谢您（如您愿意）
- 与您协作确定披露时间表

## 安全更新

安全更新将：
1. 在发布说明中公告
2. 在变更日志中标记 `security` 标签
3. 作为补丁版本发布（例如 `v1.0.1` → `v1.0.2`）

订阅 [Releases](https://github.com/keepmind9/clibot/releases) 以接收通知。

## 联系方式

一般安全相关问题或疑问：
- 邮箱：security@keepmind9.com
- GitHub Security：[https://github.com/keepmind9/clibot/security](https://github.com/keepmind9/clibot/security)

## 相关文档

- [配置指南](../README_zh.md#配置)
- [部署指南](../deployment.md)
- [贡献指南](../CONTRIBUTING_zh.md)
