package config

import (
	"log"

	"github.com/luobobo896/HSSH/pkg/types"
	"github.com/google/uuid"
)

// MigrateConfig 执行配置版本迁移
// 版本 1 -> 版本 2：添加 ID 字段，将 name 关联改为 ID 关联
func MigrateConfig(cfg *types.Config) error {
	// 如果版本号为 0，说明是旧配置，设置为版本 1
	if cfg.Version == 0 {
		cfg.Version = types.ConfigVersion1
	}

	// 版本 1 -> 版本 2 迁移
	if cfg.Version < types.ConfigVersion2 {
		log.Printf("[Config] Migrating from version %d to version %d", cfg.Version, types.ConfigVersion2)

		if err := migrateV1ToV2(cfg); err != nil {
			return err
		}

		cfg.Version = types.ConfigVersion2
		log.Printf("[Config] Migration completed, now at version %d", cfg.Version)
	}

	return nil
}

// migrateV1ToV2 执行从版本 1 到版本 2 的迁移
// 主要变更：
// 1. 为所有 Hop 生成 UUID
// 2. 将 Gateway (name) 转换为 GatewayID (uuid)
// 3. 将 RoutePreference 的 From/To/Via 转换为 FromID/ToID/ViaID
// 4. 将 Profile 的 Path (names) 转换为 PathIDs (uuids)
func migrateV1ToV2(cfg *types.Config) error {
	// 第一步：建立 name -> ID 的映射，并为没有 ID 的 Hop 生成 ID
	nameToID := make(map[string]string)

	for _, hop := range cfg.Hops {
		if hop.ID == "" {
			// 生成新的 UUID
			hop.ID = uuid.New().String()
			log.Printf("[Config] Generated ID for hop '%s': %s", hop.Name, hop.ID)
		}
		nameToID[hop.Name] = hop.ID
	}

	// 第二步：转换 Hop 的 Gateway 为 GatewayID
	for _, hop := range cfg.Hops {
		if hop.Gateway != "" && hop.GatewayID == "" {
			if gatewayID, ok := nameToID[hop.Gateway]; ok {
				hop.GatewayID = gatewayID
				log.Printf("[Config] Migrated gateway for '%s': %s -> %s", hop.Name, hop.Gateway, gatewayID)
			} else {
				log.Printf("[Config] Warning: Gateway '%s' not found for hop '%s'", hop.Gateway, hop.Name)
			}
		}
	}

	// 第三步：转换 RoutePreference
	for _, route := range cfg.Routes {
		// 转换 From
		if route.From != "" && route.FromID == "" {
			if fromID, ok := nameToID[route.From]; ok {
				route.FromID = fromID
				route.FromName = route.From // 保留原名称用于显示
				log.Printf("[Config] Migrated route From: %s -> %s", route.From, fromID)
			}
		}

		// 转换 To
		if route.To != "" && route.ToID == "" {
			if toID, ok := nameToID[route.To]; ok {
				route.ToID = toID
				route.ToName = route.To
				log.Printf("[Config] Migrated route To: %s -> %s", route.To, toID)
			}
		}

		// 转换 Via
		if route.Via != "" && route.ViaID == "" {
			if viaID, ok := nameToID[route.Via]; ok {
				route.ViaID = viaID
				route.ViaName = route.Via
				log.Printf("[Config] Migrated route Via: %s -> %s", route.Via, viaID)
			}
		}
	}

	// 第四步：转换 Profile
	for _, profile := range cfg.Profiles {
		// 为 Profile 生成 ID
		if profile.ID == "" {
			profile.ID = uuid.New().String()
			log.Printf("[Config] Generated ID for profile '%s': %s", profile.Name, profile.ID)
		}

		// 转换 Path (names) 为 PathIDs
		if len(profile.Path) > 0 && len(profile.PathIDs) == 0 {
			for _, hopName := range profile.Path {
				if hopID, ok := nameToID[hopName]; ok {
					profile.PathIDs = append(profile.PathIDs, hopID)
					profile.PathNames = append(profile.PathNames, hopName)
				} else {
					log.Printf("[Config] Warning: Hop '%s' in profile '%s' not found", hopName, profile.Name)
				}
			}
			log.Printf("[Config] Migrated profile '%s' path: %v -> %v", profile.Name, profile.Path, profile.PathIDs)
		}
	}

	return nil
}

// NeedsMigration 检查配置是否需要迁移
func NeedsMigration(cfg *types.Config) bool {
	return cfg.Version < types.ConfigVersion2
}
