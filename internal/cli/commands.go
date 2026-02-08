package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/luobobo896/HSSH/internal/config"
	"github.com/luobobo896/HSSH/internal/profiler"
	"github.com/luobobo896/HSSH/internal/proxy"
	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/internal/transfer"
	"github.com/luobobo896/HSSH/pkg/types"
)

// CLI 命令行接口
type CLI struct {
	config  *types.Config
	manager *config.Manager
	profiler *profiler.NetworkProfiler
}

// NewCLI 创建新的 CLI 实例
func NewCLI() (*CLI, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, err
	}

	cfg, err := mgr.Load()
	if err != nil {
		return nil, err
	}

	return &CLI{
		config:   cfg,
		manager:  mgr,
		profiler: profiler.NewNetworkProfiler(5 * time.Minute),
	}, nil
}

// UploadCommand 上传命令
func (c *CLI) UploadCommand(source, target string, via []string) error {
	// 解析目标路径
	targetParts := strings.SplitN(target, ":", 2)
	if len(targetParts) != 2 {
		return fmt.Errorf("invalid target format, expected host:path")
	}
	targetHost := targetParts[0]
	targetPath := targetParts[1]

	// 构建路径
	var hops []*types.Hop
	for _, hopName := range via {
		hop := c.config.GetHopByName(hopName)
		if hop == nil {
			return fmt.Errorf("hop '%s' not found in config", hopName)
		}
		hops = append(hops, hop)
	}

	// 添加目标主机
	targetHop := c.config.GetHopByName(targetHost)
	if targetHop == nil {
		return fmt.Errorf("target host '%s' not found in config", targetHost)
	}
	hops = append(hops, targetHop)

	// 建立连接链
	chain := ssh.NewChain(hops)
	fmt.Printf("Connecting via: %s -> %s\n", strings.Join(via, " -> "), targetHost)
	if err := chain.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer chain.Disconnect()

	// 创建传输器
	scp := transfer.NewSCPTransfer(chain)

	// 进度通道
	progress := make(chan *types.TransferProgress, 10)
	go func() {
		for p := range progress {
			if p.Status == "completed" {
				fmt.Printf("\r✓ %s uploaded (%.2f MB)\n", p.FileName, float64(p.TotalBytes)/1024/1024)
			} else if p.Status == "running" {
				fmt.Printf("\r%s: %.1f%% (%.2f MB/s)", p.FileName, p.Percentage(), float64(p.Speed)/1024/1024)
			}
		}
	}()

	// 执行上传
	fmt.Printf("Uploading %s to %s:%s\n", source, targetHost, targetPath)
	if err := scp.Upload(source, targetPath, progress); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	close(progress)
	time.Sleep(100 * time.Millisecond) // 等待最后的进度输出

	fmt.Println("Upload completed successfully")
	return nil
}

// ProxyCommand 端口转发命令
func (c *CLI) ProxyCommand(localAddr, remoteHost string, remotePort int, via []string) error {
	// 构建路径
	var hops []*types.Hop
	for _, hopName := range via {
		hop := c.config.GetHopByName(hopName)
		if hop == nil {
			return fmt.Errorf("hop '%s' not found in config", hopName)
		}
		hops = append(hops, hop)
	}

	// 建立连接链
	chain := ssh.NewChain(hops)
	fmt.Printf("Connecting via: %s\n", strings.Join(via, " -> "))
	if err := chain.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// 创建转发器
	forwarder := proxy.NewPortForwarder(chain, localAddr, remoteHost, remotePort)

	fmt.Printf("Starting port forward: %s -> %s:%d\n", localAddr, remoteHost, remotePort)
	fmt.Println("Press Ctrl+C to stop")

	if err := forwarder.Start(); err != nil {
		chain.Disconnect()
		return err
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	// signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nStopping port forward...")
	forwarder.Stop()
	chain.Disconnect()

	return nil
}

// ProbeCommand 探测命令
func (c *CLI) ProbeCommand(target string, via []string) error {
	ctx := context.Background()

	// 构建直连路径
	targetHop := c.config.GetHopByName(target)
	if targetHop == nil {
		return fmt.Errorf("target host '%s' not found in config", target)
	}
	directPath := []*types.Hop{targetHop}

	// 构建经跳板路径
	var viaPath []*types.Hop
	for _, hopName := range via {
		hop := c.config.GetHopByName(hopName)
		if hop == nil {
			return fmt.Errorf("hop '%s' not found in config", hopName)
		}
		viaPath = append(viaPath, hop)
	}
	viaPath = append(viaPath, targetHop)

	// 比较两条路径
	fmt.Println("Probing network paths...")
	fmt.Println()

	// 探测直连
	fmt.Printf("Direct: localhost -> %s\n", target)
	directReport, err := c.profiler.Probe(ctx, directPath)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if directReport.Success {
		fmt.Printf("  Latency: %v\n", directReport.Latency)
	} else {
		fmt.Printf("  Failed: %s\n", directReport.Error)
	}
	fmt.Println()

	// 探测经跳板
	viaStr := strings.Join(via, " -> ")
	fmt.Printf("Via %s: localhost -> %s -> %s\n", viaStr, viaStr, target)
	viaReport, err := c.profiler.Probe(ctx, viaPath)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if viaReport.Success {
		fmt.Printf("  Latency: %v\n", viaReport.Latency)
	} else {
		fmt.Printf("  Failed: %s\n", viaReport.Error)
	}
	fmt.Println()

	// 推荐
	if directReport.Success && viaReport.Success {
		if directReport.Latency < viaReport.Latency {
			diff := viaReport.Latency - directReport.Latency
			fmt.Printf("Recommendation: Direct path is faster by %v\n", diff)
		} else {
			diff := directReport.Latency - viaReport.Latency
			fmt.Printf("Recommendation: Via %s is faster by %v\n", viaStr, diff)
		}
	} else if directReport.Success {
		fmt.Println("Recommendation: Use direct path (via path failed)")
	} else if viaReport.Success {
		fmt.Printf("Recommendation: Use via %s (direct path failed)\n", viaStr)
	} else {
		fmt.Println("Both paths failed")
	}

	return nil
}

// StatusCommand 状态命令
func (c *CLI) StatusCommand() error {
	fmt.Println("=== HSSH Status ===")
	fmt.Println()

	// 显示配置的服务器
	fmt.Printf("Configured servers: %d\n", len(c.config.Hops))
	for _, hop := range c.config.Hops {
		fmt.Printf("  - %s (%s@%s:%d) [%s]\n", hop.Name, hop.User, hop.Host, hop.Port, hop.AuthType)
	}
	fmt.Println()

	// 显示路由配置
	fmt.Printf("Route preferences: %d\n", len(c.config.Routes))
	for _, route := range c.config.Routes {
		via := "direct"
		if route.Via != "" {
			via = route.Via
		}
		fmt.Printf("  - %s -> %s via %s (threshold: %dms)\n", route.From, route.To, via, route.Threshold)
	}
	fmt.Println()

	// 显示预设配置
	fmt.Printf("Profiles: %d\n", len(c.config.Profiles))
	for _, profile := range c.config.Profiles {
		fmt.Printf("  - %s: %s\n", profile.Name, strings.Join(profile.Path, " -> "))
	}

	return nil
}

// ServerAddCommand 添加服务器命令
func (c *CLI) ServerAddCommand(hop *types.Hop) error {
	if err := c.manager.AddHop(hop); err != nil {
		return err
	}
	fmt.Printf("Server '%s' added successfully\n", hop.Name)
	return nil
}

// ServerListCommand 列出服务器命令
func (c *CLI) ServerListCommand() error {
	if len(c.config.Hops) == 0 {
		fmt.Println("No servers configured")
		return nil
	}

	fmt.Printf("%-15s %-20s %-10s %-15s %-10s\n", "NAME", "HOST", "PORT", "USER", "AUTH")
	fmt.Println(strings.Repeat("-", 80))
	for _, hop := range c.config.Hops {
		fmt.Printf("%-15s %-20s %-10d %-15s %-10s\n", hop.Name, hop.Host, hop.Port, hop.User, hop.AuthType)
	}
	return nil
}

// ServerDeleteCommand 删除服务器命令
func (c *CLI) ServerDeleteCommand(name string) error {
	if err := c.manager.DeleteHop(name); err != nil {
		return err
	}
	fmt.Printf("Server '%s' deleted successfully\n", name)
	return nil
}

// ValidatePath 验证路径是否有效
func (c *CLI) ValidatePath(hopNames []string) ([]*types.Hop, error) {
	var hops []*types.Hop
	for _, name := range hopNames {
		hop := c.config.GetHopByName(name)
		if hop == nil {
			return nil, fmt.Errorf("hop '%s' not found in config", name)
		}
		hops = append(hops, hop)
	}
	return hops, nil
}

// GetConfigDir 获取配置目录
func (c *CLI) GetConfigDir() string {
	return c.config.ConfigDir
}
