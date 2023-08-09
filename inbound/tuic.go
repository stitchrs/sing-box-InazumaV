//go:build with_quic

package inbound

import (
	"context"
	"net"
	"time"

	"github.com/inazumav/sing-box/adapter"
	"github.com/inazumav/sing-box/common/tls"
	C "github.com/inazumav/sing-box/constant"
	"github.com/inazumav/sing-box/log"
	"github.com/inazumav/sing-box/option"
	"github.com/inazumav/sing-box/transport/tuic"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid/v5"
)

var _ adapter.Inbound = (*TUIC)(nil)

type TUIC struct {
	myInboundAdapter
	server    *tuic.Server
	users     []option.TUICUser
	tlsConfig tls.ServerConfig
}

func NewTUIC(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TUICInboundOptions) (*TUIC, error) {
	options.UDPFragmentDefault = true
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewServer(ctx, router, logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	rawConfig, err := tlsConfig.Config()
	if err != nil {
		return nil, err
	}
	var users []tuic.User
	for index, user := range options.Users {
		if user.UUID == "" {
			return nil, E.New("missing uuid for user ", index)
		}
		userUUID, err := uuid.FromString(user.UUID)
		if err != nil {
			return nil, E.Cause(err, "invalid uuid for user ", index)
		}
		users = append(users, tuic.User{Name: user.Name, UUID: userUUID, Password: user.Password})
	}
	inbound := &TUIC{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeTUIC,
			network:       []string{N.NetworkUDP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		users: options.Users,
	}
	server, err := tuic.NewServer(tuic.ServerOptions{
		Context:           ctx,
		Logger:            logger,
		TLSConfig:         rawConfig,
		Users:             users,
		CongestionControl: options.CongestionControl,
		AuthTimeout:       time.Duration(options.AuthTimeout),
		ZeroRTTHandshake:  options.ZeroRTTHandshake,
		Heartbeat:         time.Duration(options.Heartbeat),
		Handler:           adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, nil),
	})
	if err != nil {
		return nil, err
	}
	inbound.server = server
	return inbound, nil
}

func (h *TUIC) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	metadata = h.createMetadata(conn, metadata)
	metadata.User, _ = auth.UserFromContext[string](ctx)
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *TUIC) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	metadata = h.createPacketMetadata(conn, metadata)
	metadata.User, _ = auth.UserFromContext[string](ctx)
	h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}

func (h *TUIC) AddUsers(users []option.TUICUser) error {
	tmp := make([]option.TUICUser, 0, len(h.users)+len(users))
	tmp = append(tmp, h.users...)
	tmp = append(tmp, users...)
	h.users = tmp
	return nil
}

func (h *TUIC) DelUsers(names []string) error {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	users := make([]option.TUICUser, 0, len(h.users))
	for _, user := range h.users {
		if _, ok := nameSet[user.Name]; !ok {
			users = append(users, user)
		}
	}
	h.users = users
	return nil
}

func (h *TUIC) Start() error {
	packetConn, err := h.myInboundAdapter.ListenUDP()
	if err != nil {
		return err
	}
	return h.server.Start(packetConn)
}

func (h *TUIC) Close() error {
	return common.Close(
		&h.myInboundAdapter,
		common.PtrOrNil(h.server),
	)
}
