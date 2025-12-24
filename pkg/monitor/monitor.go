package monitor

import (
	"context"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// MonitorSet 管控 IP 监控集合（支持热更新）
type MonitorSet struct {
	configPath string
	matcher    *Matcher
	current    atomic.Value // 存储 *Matcher，无锁读取
	watcher    *fsnotify.Watcher
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *log.Logger
}

// NewMonitorSet 创建监控集合
func NewMonitorSet(configPath string) (*MonitorSet, error) {
	matcher := NewMatcher()
	ms := &MonitorSet{
		configPath: configPath,
		matcher:    matcher,
		logger:     log.Default(),
	}
	ms.current.Store(matcher)

	// 初始化加载配置
	if err := ms.reload(); err != nil {
		return nil, fmt.Errorf("初始加载配置失败: %w", err)
	}

	return ms, nil
}

// Start 启动热更新监听
func (ms *MonitorSet) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监听器失败: %w", err)
	}

	ms.watcher = watcher
	ms.ctx, ms.cancel = context.WithCancel(ctx)

	// 监听配置文件所在目录（而不是文件本身，因为某些系统需要）
	if err := watcher.Add(ms.configPath); err != nil {
		// 如果直接监听文件失败，尝试监听目录
		dir := filepath.Dir(ms.configPath)
		if err := watcher.Add(dir); err != nil {
			watcher.Close()
			return fmt.Errorf("添加文件监听失败: %w", err)
		}
	}

	// 启动监听 goroutine
	go ms.watchLoop()

	return nil
}

// watchLoop 文件监听循环
func (ms *MonitorSet) watchLoop() {
	defer ms.watcher.Close()

	// 防抖：避免短时间内多次触发
	var lastReload time.Time
	debounceDelay := 100 * time.Millisecond

	for {
		select {
		case <-ms.ctx.Done():
			return

		case event, ok := <-ms.watcher.Events:
			if !ok {
				return
			}

			// 只处理写入和重命名事件
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Rename == fsnotify.Rename {
				// 检查是否是目标文件
				if event.Name != ms.configPath {
					continue
				}

				// 防抖：避免频繁重载
				now := time.Now()
				if now.Sub(lastReload) < debounceDelay {
					continue
				}
				lastReload = now

				// 延迟一小段时间，确保文件写入完成
				time.Sleep(50 * time.Millisecond)

				// 重新加载配置
				if err := ms.reload(); err != nil {
					ms.logger.Printf("[MonitorSet] 配置热更新失败: %v", err)
				}
			}

		case err, ok := <-ms.watcher.Errors:
			if !ok {
				return
			}
			ms.logger.Printf("[MonitorSet] 文件监听错误: %v", err)
		}
	}
}

// reload 重新加载配置
func (ms *MonitorSet) reload() error {
	config, err := LoadConfig(ms.configPath)
	if err != nil {
		return err
	}

	// 记录变更前的状态
	oldIPCount, oldCIDRCount := ms.matcher.Count()

	// 创建新的匹配器
	newMatcher := NewMatcher()
	if err := newMatcher.Update(config); err != nil {
		return fmt.Errorf("更新匹配器失败: %w", err)
	}

	// 记录变更后的状态
	newIPCount, newCIDRCount := newMatcher.Count()

	// 原子性切换（无锁）
	oldMatcher := ms.current.Swap(newMatcher).(*Matcher)
	ms.matcher = newMatcher

	// 记录变更日志
	ms.logger.Printf("[MonitorSet] 配置已热更新: IP数量 %d->%d, CIDR数量 %d->%d, 启用状态: %v",
		oldIPCount, newIPCount, oldCIDRCount, newCIDRCount, config.Enabled)

	// 清理旧匹配器（实际上不需要，但可以显式释放）
	_ = oldMatcher

	return nil
}

// Match 匹配 IP（无锁读取）
func (ms *MonitorSet) Match(ip net.IP) bool {
	matcher := ms.current.Load().(*Matcher)
	return matcher.Match(ip)
}

// Stop 停止监听
func (ms *MonitorSet) Stop() error {
	if ms.cancel != nil {
		ms.cancel()
	}
	if ms.watcher != nil {
		return ms.watcher.Close()
	}
	return nil
}

// GetStats 获取统计信息
func (ms *MonitorSet) GetStats() (ipCount, cidrCount int) {
	matcher := ms.current.Load().(*Matcher)
	return matcher.Count()
}

