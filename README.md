# Go代理服务器

一个轻量级、高性能的HTTP/HTTPS代理服务器，支持基本认证和自动部署。

## 项目结构

```
proxy_server/
├── proxy_server.go      # 主程序源码
├── proxy_config.json    # 配置文件
├── proxy_server_amd64   # AMD64架构可执行文件
├── proxy_server_arm64   # ARM64架构可执行文件
├── run.sh              # Linux自动部署脚本
└── README.md           # 说明文档
```

## 功能特性

- ✅ **HTTP/HTTPS代理**: 支持HTTP CONNECT和普通HTTP请求代理
- ✅ **基本认证**: 用户名/密码认证保护
- ✅ **多架构支持**: 提供AMD64和ARM64版本
- ✅ **自动部署**: 一键部署到Linux系统
- ✅ **系统服务**: 支持systemd服务管理
- ✅ **配置灵活**: JSON配置文件，支持热重载
- ✅ **日志记录**: 详细的访问和错误日志

## 快速开始

### 1. 配置文件

编辑 `proxy_config.json` 文件：

```json
{
  "port": ":61055",
  "username": "your_username",
  "password": "your_password",
  "timeout_seconds": 30
}
```

**配置说明：**
- `port`: 代理服务器监听端口（格式：`:端口号`）
- `username`: 代理认证用户名
- `password`: 代理认证密码
- `timeout_seconds`: 连接超时时间（秒）

### 2. 手动运行


#### Linux系统
```bash
# 给予执行权限
chmod +x proxy_server_amd64
# 或 chmod +x proxy_server_arm64

# 运行
./proxy_server_amd64
# 或 ./proxy_server_arm64
```

### 3. Linux自动部署（推荐）

使用提供的部署脚本可以自动安装为系统服务：

```bash
# 给予脚本执行权限
chmod +x run.sh

# 以root权限运行部署脚本
sudo ./run.sh
```

**部署脚本功能：**
- 自动检测系统架构（AMD64/ARM64）
- 选择对应的二进制文件
- 创建工作目录 `/opt/proxy-server`
- 安装为systemd系统服务
- 配置开机自启动
- 清理不需要的文件

## 服务管理

部署完成后，可以使用以下命令管理服务：

```bash
# 查看服务状态
sudo systemctl status proxy-server

# 启动服务
sudo systemctl start proxy-server

# 停止服务
sudo systemctl stop proxy-server

# 重启服务
sudo systemctl restart proxy-server

# 查看实时日志
sudo journalctl -u proxy-server -f

# 禁用开机自启
sudo systemctl disable proxy-server

# 启用开机自启
sudo systemctl enable proxy-server
```

## 客户端配置

### 浏览器代理设置

1. **HTTP代理**: `服务器IP:61055`
2. **HTTPS代理**: `服务器IP:61055`
3. **认证**: 用户名和密码（配置文件中设置的）

### curl命令示例

```bash
# HTTP请求
curl -x http://username:password@服务器IP:61055 http://httpbin.org/ip

# HTTPS请求
curl -x http://username:password@服务器IP:61055 https://httpbin.org/ip
```

### Python requests示例

```python
import requests

proxies = {
    'http': 'http://username:password@服务器IP:61055',
    
}

response = requests.get('https://httpbin.org/ip', proxies=proxies)
print(response.text)
```

## 配置修改

### 修改配置文件

```bash
# 编辑配置文件
sudo nano /opt/proxy-server/proxy_config.json

# 重启服务使配置生效
sudo systemctl restart proxy-server
```

### 修改端口

1. 编辑配置文件中的 `port` 字段
2. 重启服务
3. 更新防火墙规则（如果需要）

```bash
# Ubuntu/Debian
sudo ufw allow 新端口号

# CentOS/RHEL
sudo firewall-cmd --permanent --add-port=新端口号/tcp
sudo firewall-cmd --reload
```

## 故障排除

### 常见问题

1. **服务启动失败**
   ```bash
   # 查看详细错误日志
   sudo journalctl -u proxy-server -n 50
   ```

2. **端口被占用**
   ```bash
   # 检查端口占用
   sudo netstat -tlnp | grep :61055
   # 或使用ss命令
   sudo ss -tlnp | grep :61055
   ```

3. **权限问题**
   ```bash
   # 确保二进制文件有执行权限
   sudo chmod +x /opt/proxy-server/proxy_server
   ```

4. **配置文件格式错误**
   ```bash
   # 验证JSON格式
   python3 -m json.tool /opt/proxy-server/proxy_config.json
   ```

### 日志分析

```bash
# 查看最近的日志
sudo journalctl -u proxy-server -n 100

# 实时监控日志
sudo journalctl -u proxy-server -f

# 查看特定时间段的日志
sudo journalctl -u proxy-server --since "2024-01-01 00:00:00" --until "2024-01-01 23:59:59"
```

## 安全建议

1. **强密码**: 使用复杂的用户名和密码
2. **防火墙**: 限制代理端口的访问来源
3. **定期更新**: 定期更新代理服务器程序
4. **监控日志**: 定期检查访问日志，发现异常访问

### 防火墙配置示例

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow from 信任的IP地址 to any port 61055

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-rich-rule="rule family='ipv4' source address='信任的IP地址' port protocol='tcp' port='61055' accept"
sudo firewall-cmd --reload
```

## 性能优化

1. **调整超时时间**: 根据网络环境调整 `timeout_seconds`
2. **系统限制**: 增加系统文件描述符限制
   ```bash
   # 临时调整
   ulimit -n 65536
   
   # 永久调整（编辑 /etc/security/limits.conf）
   * soft nofile 65536
   * hard nofile 65536
   ```

## 卸载

如需完全卸载代理服务器：

```bash
# 停止并禁用服务
sudo systemctl stop proxy-server
sudo systemctl disable proxy-server

# 删除服务文件
sudo rm /etc/systemd/system/proxy-server.service

# 重新加载systemd
sudo systemctl daemon-reload

# 删除程序文件
sudo rm -rf /opt/proxy-server
```

## 开发说明

### 编译

```bash
# 编译AMD64版本
GOOS=linux GOARCH=amd64 go build -o proxy_server_amd64 proxy_server.go

# 编译ARM64版本
GOOS=linux GOARCH=arm64 go build -o proxy_server_arm64 proxy_server.go

# Windows版本
GOOS=windows GOARCH=amd64 go build -o proxy_server_amd64.exe proxy_server.go
```

### 依赖

本项目仅使用Go标准库，无外部依赖。

## 许可证

本项目采用 [MIT许可证](LICENSE)。

详细许可证条款请查看 [LICENSE](LICENSE) 文件。

## 支持

如有问题或建议，请提交Issue或联系开发者。