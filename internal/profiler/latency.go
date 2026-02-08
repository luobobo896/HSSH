package profiler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
)

// NetworkProfiler 网络性能分析器
type NetworkProfiler struct {
	cache      map[string]*types.LatencyReport
	cacheTTL   time.Duration
	mu         sync.RWMutex
}

// NewNetworkProfiler 创建新的网络分析器
func NewNetworkProfiler(cacheTTL time.Duration) *NetworkProfiler {
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}
	return &NetworkProfiler{
		cache:    make(map[string]*types.LatencyReport),
		cacheTTL: cacheTTL,
	}
}

// Probe 探测指定路径的延迟
func (np *NetworkProfiler) Probe(ctx context.Context, hops []*types.Hop) (*types.LatencyReport, error) {
	path := types.Path{
		From: "localhost",
		To:   hops[len(hops)-1].Name,
		Via:  make([]string, 0, len(hops)-1),
	}
	for i := 0; i < len(hops)-1; i++ {
		path.Via = append(path.Via, hops[i].Name)
	}

	// 检查缓存
	if report := np.getCached(path); report != nil {
		return report, nil
	}

	// 执行探测
	report, err := np.doProbe(ctx, hops, path)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	np.setCache(path, report)

	return report, nil
}

// doProbe 执行实际的延迟探测
func (np *NetworkProfiler) doProbe(ctx context.Context, hops []*types.Hop, path types.Path) (*types.LatencyReport, error) {
	chain := ssh.NewChain(hops)

	start := time.Now()
	if err := chain.Connect(); err != nil {
		return &types.LatencyReport{
			Path:      path,
			Latency:   0,
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
		}, nil
	}
	defer chain.Disconnect()

	// 执行一个简单命令来测试延迟
	_, _, err := chain.Execute("echo ping")
	latency := time.Since(start)

	if err != nil {
		return &types.LatencyReport{
			Path:      path,
			Latency:   latency,
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	return &types.LatencyReport{
		Path:      path,
		Latency:   latency,
		Timestamp: time.Now(),
		Success:   true,
	}, nil
}

// getCached 获取缓存的报告
func (np *NetworkProfiler) getCached(path types.Path) *types.LatencyReport {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if report, exists := np.cache[path.Key()]; exists {
		if time.Since(report.Timestamp) < np.cacheTTL {
			return report
		}
	}
	return nil
}

// setCache 设置缓存
func (np *NetworkProfiler) setCache(path types.Path, report *types.LatencyReport) {
	np.mu.Lock()
	defer np.mu.Unlock()
	np.cache[path.Key()] = report
}

// ClearCache 清除缓存
func (np *NetworkProfiler) ClearCache() {
	np.mu.Lock()
	defer np.mu.Unlock()
	np.cache = make(map[string]*types.LatencyReport)
}

// ComparePaths 比较两条路径的性能
func (np *NetworkProfiler) ComparePaths(ctx context.Context, pathA, pathB []*types.Hop) (*types.LatencyReport, *types.LatencyReport, error) {
	reportA, err := np.Probe(ctx, pathA)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to probe path A: %w", err)
	}

	reportB, err := np.Probe(ctx, pathB)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to probe path B: %w", err)
	}

	return reportA, reportB, nil
}

// GetBestPath 从多条路径中选择最优的一条
func (np *NetworkProfiler) GetBestPath(ctx context.Context, paths [][]*types.Hop) ([]*types.Hop, *types.LatencyReport, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no paths provided")
	}

	var bestPath []*types.Hop
	var bestReport *types.LatencyReport

	for _, path := range paths {
		report, err := np.Probe(ctx, path)
		if err != nil {
			continue
		}

		if !report.Success {
			continue
		}

		if bestReport == nil || report.Latency < bestReport.Latency {
			bestReport = report
			bestPath = path
		}
	}

	if bestPath == nil {
		return nil, nil, fmt.Errorf("no viable path found")
	}

	return bestPath, bestReport, nil
}

// MeasureBandwidth 测量带宽（简化版，基于文件传输时间估算）
func (np *NetworkProfiler) MeasureBandwidth(ctx context.Context, chain *ssh.Chain, testSize int64) (int64, error) {
	if !chain.IsConnected() {
		return 0, fmt.Errorf("chain not connected")
	}

	// 生成测试数据
	testData := make([]byte, testSize)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// 使用 dd 命令测试写入速度
	start := time.Now()
	_, stderr, err := chain.Execute(fmt.Sprintf("dd bs=1M count=%d 2>&1 | tail -1", testSize/(1024*1024)))
	if err != nil {
		return 0, fmt.Errorf("bandwidth test failed: %w, stderr: %s", err, stderr)
	}

	elapsed := time.Since(start)
	bandwidth := int64(float64(testSize) / elapsed.Seconds())

	return bandwidth, nil
}
