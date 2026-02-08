package server

import (
	"fmt"
	"net"

	"github.com/luobobo896/HSSH/pkg/portal"
)

// Authenticator handles token authentication
type Authenticator struct {
	tokens map[string]*portal.TokenConfig // token -> config
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(tokens []portal.TokenConfig) *Authenticator {
	t := make(map[string]*portal.TokenConfig)
	for i := range tokens {
		t[tokens[i].Token] = &tokens[i]
	}
	return &Authenticator{tokens: t}
}

// ValidateToken validates a token and returns its config
func (a *Authenticator) ValidateToken(token string) (*portal.TokenConfig, error) {
	config, ok := a.tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return config, nil
}

// IsRemoteAllowed checks if a remote address is allowed for a token
func (a *Authenticator) IsRemoteAllowed(tokenConfig *portal.TokenConfig, remoteHost string) bool {
	if len(tokenConfig.AllowedRemotes) == 0 {
		return true // No restrictions
	}

	// Parse remote host
	ip := net.ParseIP(remoteHost)
	if ip == nil {
		// Try to resolve as hostname - for now, allow if any CIDR is 0.0.0.0/0
		for _, cidr := range tokenConfig.AllowedRemotes {
			if cidr == "0.0.0.0/0" {
				return true
			}
		}
		return false
	}

	// Check against CIDRs
	for _, cidr := range tokenConfig.AllowedRemotes {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			return true
		}
	}

	return false
}
