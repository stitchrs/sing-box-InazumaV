//go:build !with_shadowsocksr

package outbound

import (
	"context"

	"github.com/inazumav/sing-box/adapter"
	"github.com/inazumav/sing-box/log"
	"github.com/inazumav/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewShadowsocksR(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksROutboundOptions) (adapter.Outbound, error) {
	return nil, E.New(`ShadowsocksR is not included in this build, rebuild with -tags with_shadowsocksr`)
}
