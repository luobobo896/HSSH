package transfer

import (
	"testing"

	"github.com/luobobo896/HSSH/pkg/types"
)

// TestBuildTransferChain 测试构建传输链路的各种场景
func TestBuildTransferChain(t *testing.T) {
	tests := []struct {
		name         string
		server       *types.Hop
		viaHops      []string
		wantChain    []string
		description  string
	}{
		{
			name: "场景1: 内网服务器 + 有中转节点",
			server: &types.Hop{
				Name:       "内网 16",
				Host:       "172.27.226.16",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			viaHops:     []string{"HK"},
			wantChain:   []string{"本地", "HK", "网关", "内网 16"},
			description: "本地 → HK → 网关 → 内网 16",
		},
		{
			name: "场景2: 内网服务器 + 无中转节点",
			server: &types.Hop{
				Name:       "内网 16",
				Host:       "172.27.226.16",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			viaHops:     []string{},
			wantChain:   []string{"本地", "网关", "内网 16"},
			description: "本地 → 网关 → 内网 16",
		},
		{
			name: "场景3: 外网服务器 + 有中转节点",
			server: &types.Hop{
				Name:       "外网服务器",
				Host:       "1.2.3.4",
				ServerType: types.ServerExternal,
			},
			viaHops:     []string{"HK"},
			wantChain:   []string{"本地", "HK", "外网服务器"},
			description: "本地 → HK → 外网服务器",
		},
		{
			name: "场景4: 外网服务器 + 无中转节点",
			server: &types.Hop{
				Name:       "外网服务器",
				Host:       "1.2.3.4",
				ServerType: types.ServerExternal,
			},
			viaHops:     []string{},
			wantChain:   []string{"本地", "外网服务器"},
			description: "本地 → 外网服务器（直连）",
		},
		{
			name: "场景5: 内网服务器 + 多个中转节点",
			server: &types.Hop{
				Name:       "内网 16",
				Host:       "172.27.226.16",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			viaHops:     []string{"HK", "日本", "新加坡"},
			wantChain:   []string{"本地", "HK", "日本", "新加坡", "网关", "内网 16"},
			description: "本地 → HK → 日本 → 新加坡 → 网关 → 内网 16",
		},
		{
			name: "场景6: 内网服务器 + viaHops包含网关（去重）",
			server: &types.Hop{
				Name:       "内网 16",
				Host:       "172.27.226.16",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			viaHops:     []string{"HK", "网关"}, // 用户手动选择了网关
			wantChain:   []string{"本地", "HK", "网关", "内网 16"},
			description: "网关应该只出现一次",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 构建链路
			chain := buildTransferChain(tt.server, tt.viaHops)

			// 验证链路长度
			if len(chain) != len(tt.wantChain) {
				t.Errorf("链路长度不匹配: got %d, want %d\ngot: %v\nwant: %v",
					len(chain), len(tt.wantChain), chain, tt.wantChain)
				return
			}

			// 验证每个节点
			for i, want := range tt.wantChain {
				if chain[i] != want {
					t.Errorf("链路节点[%d]不匹配: got %s, want %s", i, chain[i], want)
				}
			}

			t.Logf("✅ %s: %s", tt.description, chain)
		})
	}
}

// buildTransferChain 构建传输链路（与前端逻辑一致）
func buildTransferChain(server *types.Hop, viaHops []string) []string {
	chain := []string{"本地"}

	// 1. 先添加中转节点（排除网关）
	for _, hop := range viaHops {
		if hop != server.Gateway {
			chain = append(chain, hop)
		}
	}

	// 2. 如果是内网服务器，添加网关
	if server.ServerType == types.ServerInternal && server.Gateway != "" {
		chain = append(chain, server.Gateway)
	}

	// 3. 最后添加目标服务器
	chain = append(chain, server.Name)

	return chain
}

// TestServerTypeDetection 测试服务器类型识别
func TestServerTypeDetection(t *testing.T) {
	tests := []struct {
		name       string
		serverType types.ServerType
		isInternal bool
	}{
		{
			name:       "外网服务器 (ServerExternal)",
			serverType: types.ServerExternal,
			isInternal: false,
		},
		{
			name:       "内网服务器 (ServerInternal)",
			serverType: types.ServerInternal,
			isInternal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInternal := tt.serverType == types.ServerInternal
			if isInternal != tt.isInternal {
				t.Errorf("服务器类型判断错误: got %v, want %v", isInternal, tt.isInternal)
			}
		})
	}
}

// TestGatewayRequired 测试内网服务器必须配置网关
func TestGatewayRequired(t *testing.T) {
	tests := []struct {
		name        string
		server      *types.Hop
		wantValid   bool
		description string
	}{
		{
			name: "内网服务器有网关 - 有效",
			server: &types.Hop{
				Name:       "内网 16",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			wantValid:   true,
			description: "内网服务器配置了网关",
		},
		{
			name: "内网服务器无网关 - 无效",
			server: &types.Hop{
				Name:       "内网 16",
				ServerType: types.ServerInternal,
				Gateway:    "",
			},
			wantValid:   false,
			description: "内网服务器必须配置网关",
		},
		{
			name: "外网服务器无网关 - 有效",
			server: &types.Hop{
				Name:       "外网服务器",
				ServerType: types.ServerExternal,
				Gateway:    "",
			},
			wantValid:   true,
			description: "外网服务器不需要网关",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.server.ServerType != types.ServerInternal || tt.server.Gateway != ""
			if valid != tt.wantValid {
				t.Errorf("网关验证失败: got %v, want %v - %s", valid, tt.wantValid, tt.description)
			}
		})
	}
}

// TestSwitchServer 测试切换服务器时的网关清理逻辑
func TestSwitchServer(t *testing.T) {
	tests := []struct {
		name          string
		fromServer    *types.Hop
		toServer      *types.Hop
		initialHops   []string
		wantFinalHops []string
		description   string
	}{
		{
			name: "从内网A切换到内网B - 更换网关",
			fromServer: &types.Hop{
				Name:       "内网A",
				ServerType: types.ServerInternal,
				Gateway:    "网关A",
			},
			toServer: &types.Hop{
				Name:       "内网B",
				ServerType: types.ServerInternal,
				Gateway:    "网关B",
			},
			initialHops:   []string{"HK", "网关A"},
			wantFinalHops: []string{"HK", "网关B"},
			description:   "应该移除网关A，添加网关B",
		},
		{
			name: "从内网切换到外网 - 移除网关",
			fromServer: &types.Hop{
				Name:       "内网",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			toServer: &types.Hop{
				Name:       "外网",
				ServerType: types.ServerExternal,
			},
			initialHops:   []string{"HK", "网关"},
			wantFinalHops: []string{"HK"},
			description:   "应该移除网关，保留中转节点",
		},
		{
			name: "从外网切换到内网 - 添加网关",
			fromServer: &types.Hop{
				Name:       "外网",
				ServerType: types.ServerExternal,
			},
			toServer: &types.Hop{
				Name:       "内网",
				ServerType: types.ServerInternal,
				Gateway:    "网关",
			},
			initialHops:   []string{"HK"},
			wantFinalHops: []string{"HK", "网关"},
			description:   "应该添加网关到末尾",
		},
		{
			name: "切换外网服务器 - 保持viaHops不变",
			fromServer: &types.Hop{
				Name:       "外网A",
				ServerType: types.ServerExternal,
			},
			toServer: &types.Hop{
				Name:       "外网B",
				ServerType: types.ServerExternal,
			},
			initialHops:   []string{"HK", "日本"},
			wantFinalHops: []string{"HK", "日本"},
			description:   "外网服务器之间切换，viaHops应该保持不变",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟切换服务器时的清理逻辑
			servers := []*types.Hop{tt.fromServer, tt.toServer}
			finalHops := simulateSwitchServer(tt.toServer, tt.initialHops, servers)

			// 验证结果
			if len(finalHops) != len(tt.wantFinalHops) {
				t.Errorf("viaHops长度不匹配: got %v, want %v", finalHops, tt.wantFinalHops)
				return
			}
			for i, want := range tt.wantFinalHops {
				if finalHops[i] != want {
					t.Errorf("viaHops[%d]不匹配: got %s, want %s", i, finalHops[i], want)
				}
			}

			t.Logf("✅ %s: %v -> %v", tt.description, tt.initialHops, finalHops)
		})
	}
}

// simulateSwitchServer 模拟切换服务器时的viaHops清理逻辑
func simulateSwitchServer(targetServer *types.Hop, currentHops []string, allServers []*types.Hop) []string {
	// 清理不属于当前服务器的网关
	otherGateways := []string{}
	for _, s := range allServers {
		if s.Name != targetServer.Name && s.ServerType == types.ServerInternal && s.Gateway != "" {
			otherGateways = append(otherGateways, s.Gateway)
		}
	}

	cleanedHops := []string{}
	for _, h := range currentHops {
		isOtherGateway := false
		for _, g := range otherGateways {
			if h == g {
				isOtherGateway = true
				break
			}
		}
		if !isOtherGateway {
			cleanedHops = append(cleanedHops, h)
		}
	}

	// 如果是内网服务器，添加网关
	if targetServer.ServerType == types.ServerInternal && targetServer.Gateway != "" {
		found := false
		for _, h := range cleanedHops {
			if h == targetServer.Gateway {
				found = true
				break
			}
		}
		if !found {
			cleanedHops = append(cleanedHops, targetServer.Gateway)
		}
	}

	return cleanedHops
}
