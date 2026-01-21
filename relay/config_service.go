package relay

import (
	"context"

	"connectrpc.com/connect"
	configv1 "github.com/unblink/unblink/relay/gen/unblink/config/v1"
)

// ConfigService implements the ConfigServiceHandler interface
type ConfigService struct {
	relay *Relay
	cfg   *Config
}

func NewConfigService(relay *Relay, cfg *Config) *ConfigService {
	return &ConfigService{
		relay: relay,
		cfg:   cfg,
	}
}

// GetFlags returns feature flags
func (s *ConfigService) GetFlags(
	ctx context.Context,
	req *connect.Request[configv1.GetFlagsRequest],
) (*connect.Response[configv1.GetFlagsResponse], error) {
	return connect.NewResponse(&configv1.GetFlagsResponse{
		DevImpersonateEmail: s.cfg.DevImpersonate,
	}), nil
}
