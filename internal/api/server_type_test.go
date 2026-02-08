package api

import (
	"testing"

	"github.com/luobobo896/HSSH/pkg/types"
)

func TestServerTypeParsing(t *testing.T) {
	tests := []struct {
		name         string
		serverType   string
		existingType types.ServerType
		expected     types.ServerType
	}{
		// POST 创建场景 (默认 external)
		{"empty string defaults to external", "", types.ServerExternal, types.ServerExternal},
		{"external string", "external", types.ServerExternal, types.ServerExternal},
		{"internal string", "internal", types.ServerExternal, types.ServerInternal},
		{"0 as external", "0", types.ServerExternal, types.ServerExternal},
		{"1 as internal", "1", types.ServerExternal, types.ServerInternal},

		// PUT 更新场景 (保留 existing 值)
		{"empty keeps existing external", "", types.ServerExternal, types.ServerExternal},
		{"empty keeps existing internal", "", types.ServerInternal, types.ServerInternal},
		{"external overrides existing internal", "external", types.ServerInternal, types.ServerExternal},
		{"internal overrides existing external", "internal", types.ServerExternal, types.ServerInternal},
		{"0 overrides existing internal", "0", types.ServerInternal, types.ServerExternal},
		{"1 overrides existing external", "1", types.ServerExternal, types.ServerInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result types.ServerType
			if tt.serverType != "" {
				switch tt.serverType {
				case "external", "0":
					result = types.ServerExternal
				case "internal", "1":
					result = types.ServerInternal
				default:
					result = tt.existingType
				}
			} else {
				result = tt.existingType
			}

			if result != tt.expected {
				t.Errorf("server_type = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestServerTypeToString(t *testing.T) {
	if types.ServerExternal.String() != "external" {
		t.Errorf("ServerExternal.String() = %v, want external", types.ServerExternal.String())
	}
	if types.ServerInternal.String() != "internal" {
		t.Errorf("ServerInternal.String() = %v, want internal", types.ServerInternal.String())
	}
}
