package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// 配置结构
type Config struct {
	Port           string `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// 默认配置
var defaultConfig = Config{
	Port:           ":61055",
	Username:       "username",
	Password:       "password",
	TimeoutSeconds: 30,
}

// 代理服务器结构
type ProxyServer struct {
	config *Config
	logger *log.Logger
}

// 加载配置文件
func loadConfig() *Config {
	configFile := "proxy_config.json"

	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Printf("配置文件 %s 不存在，使用默认配置", configFile)
		return &defaultConfig
	}

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("读取配置文件失败: %v，使用默认配置", err)
		return &defaultConfig
	}

	// 解析JSON配置
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("解析配置文件失败: %v，使用默认配置", err)
		return &defaultConfig
	}

	// 验证配置
	if config.Port == "" {
		config.Port = defaultConfig.Port
	}
	if config.Username == "" {
		config.Username = defaultConfig.Username
	}
	if config.Password == "" {
		config.Password = defaultConfig.Password
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = defaultConfig.TimeoutSeconds
	}

	log.Printf("成功加载配置文件: %s", configFile)
	return &config
}

// 创建新的代理服务器
func NewProxyServer() *ProxyServer {
	config := loadConfig()
	return &ProxyServer{
		config: config,
		logger: log.New(os.Stdout, "[PROXY] ", log.LstdFlags),
	}
}

// 验证代理认证
func (p *ProxyServer) authenticate(r *http.Request) bool {
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		return false
	}

	// 解析 "Basic base64(username:password)"
	if !strings.HasPrefix(auth, "Basic ") {
		return false
	}

	encoded := auth[6:] // 移除 "Basic "
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}

	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return false
	}

	return creds[0] == p.config.Username && creds[1] == p.config.Password
}

// 处理HTTP CONNECT请求（用于HTTPS）
func (p *ProxyServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if !p.authenticate(r) {
		p.logger.Printf("认证失败: %s", r.RemoteAddr)
		w.Header().Set("Proxy-Authenticate", "Basic realm=\"Proxy\"")
		http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
		return
	}

	p.logger.Printf("处理CONNECT请求: %s <- %s", r.Host, r.RemoteAddr)

	// 连接到目标服务器
	timeout := time.Duration(p.config.TimeoutSeconds) * time.Second
	targetConn, err := net.DialTimeout("tcp", r.Host, timeout)
	if err != nil {
		p.logger.Printf("连接目标服务器失败 %s: %v", r.Host, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// 获取客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		p.logger.Printf("不支持连接劫持")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		p.logger.Printf("连接劫持失败: %v", err)
		return
	}
	defer clientConn.Close()

	// 发送200 Connection Established响应
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		p.logger.Printf("发送CONNECT响应失败: %v", err)
		return
	}

	// 开始双向数据转发
	p.forwardData(clientConn, targetConn)
}

// 处理普通HTTP请求
func (p *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.authenticate(r) {
		p.logger.Printf("认证失败: %s", r.RemoteAddr)
		w.Header().Set("Proxy-Authenticate", "Basic realm=\"Proxy\"")
		http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
		return
	}

	p.logger.Printf("处理HTTP请求: %s %s <- %s", r.Method, r.URL.String(), r.RemoteAddr)

	// 创建新的请求
	targetURL := r.URL
	if !targetURL.IsAbs() {
		// 如果是相对URL，构造绝对URL
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		targetURL = &url.URL{
			Scheme:   scheme,
			Host:     r.Host,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
	}

	// 创建HTTP客户端
	timeout := time.Duration(p.config.TimeoutSeconds) * time.Second
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动跟随重定向
		},
	}

	// 创建新请求
	req, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		p.logger.Printf("创建请求失败: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 复制请求头（排除一些代理相关的头）
	for name, values := range r.Header {
		if !p.shouldSkipHeader(name) {
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		p.logger.Printf("请求失败: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		p.logger.Printf("复制响应体失败: %v", err)
	}
}

// 判断是否应该跳过某个请求头
func (p *ProxyServer) shouldSkipHeader(name string) bool {
	skipHeaders := []string{
		"Connection",
		"Proxy-Connection",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Upgrade",
	}

	nameLower := strings.ToLower(name)
	for _, skip := range skipHeaders {
		if strings.ToLower(skip) == nameLower {
			return true
		}
	}
	return false
}

// 双向数据转发
func (p *ProxyServer) forwardData(client, target net.Conn) {
	// 设置连接超时
	timeout := time.Duration(p.config.TimeoutSeconds) * time.Second
	client.SetDeadline(time.Now().Add(timeout))
	target.SetDeadline(time.Now().Add(timeout))

	// 创建两个goroutine进行双向转发
	done := make(chan bool, 2)

	// 客户端到目标服务器
	go func() {
		defer func() { done <- true }()
		io.Copy(target, client)
	}()

	// 目标服务器到客户端
	go func() {
		defer func() { done <- true }()
		io.Copy(client, target)
	}()

	// 等待任一方向的传输完成
	<-done
}

// 启动代理服务器
func (p *ProxyServer) Start() error {
	p.logger.Printf("代理服务器启动，监听端口 %s", p.config.Port)
	p.logger.Printf("用户名: %s", p.config.Username)
	p.logger.Printf("状态: %s", p.getProxyInfo())

	// 创建HTTP服务器
	timeout := time.Duration(p.config.TimeoutSeconds) * time.Second
	server := &http.Server{
		Addr:         p.config.Port,
		Handler:      p,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}

	return server.ListenAndServe()
}

// 获取代理信息
func (p *ProxyServer) getProxyInfo() string {
	return "代理认证已启用"
}

// 实现http.Handler接口
func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func main() {
	// 创建代理服务器
	proxy := NewProxyServer()

	// 启动服务器
	log.Printf("正在启动代理服务器...")
	if err := proxy.Start(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
