package monitor

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigYAML(t *testing.T) {
	// 创建临时 YAML 文件
	tmpFile := filepath.Join(t.TempDir(), "test.yaml")
	content := `enabled: true
targets:
  - type: ip
    value: "192.168.1.1"
  - type: cidr
    value: "10.0.0.0/24"
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	config, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if !config.Enabled {
		t.Error("配置应该启用")
	}
	if len(config.Targets) != 2 {
		t.Errorf("期望 2 个目标，实际 %d", len(config.Targets))
	}
}

func TestLoadConfigJSON(t *testing.T) {
	// 创建临时 JSON 文件
	tmpFile := filepath.Join(t.TempDir(), "test.json")
	content := `{
  "enabled": true,
  "targets": [
    { "type": "ip", "value": "192.168.1.1" },
    { "type": "cidr", "value": "10.0.0.0/24" }
  ]
}`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	config, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if !config.Enabled {
		t.Error("配置应该启用")
	}
	if len(config.Targets) != 2 {
		t.Errorf("期望 2 个目标，实际 %d", len(config.Targets))
	}
}

func TestMatcher_Update(t *testing.T) {
	matcher := NewMatcher()

	config := &Config{
		Enabled: true,
		Targets: []Target{
			{Type: "ip", Value: "192.168.1.1"},
			{Type: "cidr", Value: "10.0.0.0/24"},
		},
	}

	if err := matcher.Update(config); err != nil {
		t.Fatalf("更新匹配器失败: %v", err)
	}

	ipCount, cidrCount := matcher.Count()
	if ipCount != 1 {
		t.Errorf("期望 1 个 IP，实际 %d", ipCount)
	}
	if cidrCount != 1 {
		t.Errorf("期望 1 个 CIDR，实际 %d", cidrCount)
	}
}

func TestMatcher_Match(t *testing.T) {
	matcher := NewMatcher()

	config := &Config{
		Enabled: true,
		Targets: []Target{
			{Type: "ip", Value: "192.168.1.1"},
			{Type: "cidr", Value: "10.0.0.0/24"},
		},
	}

	if err := matcher.Update(config); err != nil {
		t.Fatalf("更新匹配器失败: %v", err)
	}

	// 测试单 IP 匹配
	ip1 := net.ParseIP("192.168.1.1")
	if !matcher.Match(ip1) {
		t.Error("应该匹配 192.168.1.1")
	}

	// 测试 CIDR 匹配
	ip2 := net.ParseIP("10.0.0.100")
	if !matcher.Match(ip2) {
		t.Error("应该匹配 10.0.0.100 (在 10.0.0.0/24 中)")
	}

	// 测试不匹配
	ip3 := net.ParseIP("172.16.0.1")
	if matcher.Match(ip3) {
		t.Error("不应该匹配 172.16.0.1")
	}
}

func TestMonitorSet_HotReload(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "monitor.yaml")

	// 初始配置
	initialConfig := `enabled: true
targets:
  - type: ip
    value: "192.168.1.1"
`
	if err := os.WriteFile(configFile, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("写入初始配置失败: %v", err)
	}

	// 创建 MonitorSet
	ms, err := NewMonitorSet(configFile)
	if err != nil {
		t.Fatalf("创建 MonitorSet 失败: %v", err)
	}

	// 验证初始配置
	ip := net.ParseIP("192.168.1.1")
	if !ms.Match(ip) {
		t.Error("初始配置应该匹配 192.168.1.1")
	}

	// 启动热更新
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ms.Start(ctx); err != nil {
		t.Fatalf("启动热更新失败: %v", err)
	}
	defer ms.Stop()

	// 更新配置文件
	updatedConfig := `enabled: true
targets:
  - type: ip
    value: "10.0.0.1"
  - type: cidr
    value: "172.16.0.0/16"
`
	if err := os.WriteFile(configFile, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("更新配置文件失败: %v", err)
	}

	// 等待热更新生效（需要一些时间让 fsnotify 触发）
	time.Sleep(200 * time.Millisecond)

	// 验证新配置
	ip1 := net.ParseIP("10.0.0.1")
	if !ms.Match(ip1) {
		t.Error("新配置应该匹配 10.0.0.1")
	}

	ip2 := net.ParseIP("172.16.1.1")
	if !ms.Match(ip2) {
		t.Error("新配置应该匹配 172.16.1.1 (在 CIDR 中)")
	}

	// 旧 IP 不应该匹配
	if ms.Match(ip) {
		t.Error("旧 IP 192.168.1.1 不应该再匹配")
	}
}

func TestMatcher_ConcurrentMatch(t *testing.T) {
	matcher := NewMatcher()

	config := &Config{
		Enabled: true,
		Targets: []Target{
			{Type: "ip", Value: "192.168.1.1"},
			{Type: "cidr", Value: "10.0.0.0/24"},
		},
	}

	if err := matcher.Update(config); err != nil {
		t.Fatalf("更新匹配器失败: %v", err)
	}

	// 并发匹配测试
	ip := net.ParseIP("10.0.0.100")
	concurrency := 1000
	results := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			results <- matcher.Match(ip)
		}()
	}

	// 收集结果
	matchCount := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			matchCount++
		}
	}

	if matchCount != concurrency {
		t.Errorf("所有并发匹配都应该成功，实际成功 %d/%d", matchCount, concurrency)
	}
}

