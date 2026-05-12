package cloudwego

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/gopkg/cloud/metainfo"
	hertzdiscovery "github.com/cloudwego/hertz/pkg/app/client/discovery"
	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	hertzregistry "github.com/cloudwego/hertz/pkg/app/server/registry"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	kitexserver "github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
	hertzetcd "github.com/hertz-contrib/registry/etcd"
	kitexetcd "github.com/kitex-contrib/registry-etcd"

	mw "github.com/castlexu/micro-service/pkg/middleware"
	mwkitex "github.com/castlexu/micro-service/pkg/middleware/kitex"
)

const (
	defaultRegistryType = "etcd"
	defaultPrefix       = "micro-service"
	defaultWeight       = 10
)

func KitexRegistryOptions(cfg RegistryConfig) ([]kitexserver.Option, error) {
	if err := validateRegistryConfig(cfg, false); err != nil {
		return nil, err
	}
	r, err := kitexetcd.NewEtcdRegistry(
		cfg.Endpoints,
		kitexetcd.WithEtcdServicePrefix(kitexPrefix(cfg.Prefix)),
	)
	if err != nil {
		return nil, err
	}
	return []kitexserver.Option{
		kitexserver.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: cfg.ServiceName}),
		kitexserver.WithRegistry(r),
		kitexserver.WithMetaHandler(transmeta.MetainfoServerHandler),
	}, nil
}

func KitexClientOptions(cfg DiscoveryConfig) ([]client.Option, error) {
	if err := validateDiscoveryConfig(cfg); err != nil {
		return nil, err
	}
	r, err := kitexetcd.NewEtcdResolver(
		cfg.Endpoints,
		kitexetcd.WithEtcdServicePrefix(kitexPrefix(cfg.Prefix)),
	)
	if err != nil {
		return nil, err
	}
	return []client.Option{
		client.WithResolver(r),
		client.WithTransportProtocol(transport.TTHeaderFramed),
		client.WithMiddleware(mwkitex.ClientTrace()),
		client.WithMiddleware(kitexClientMetaForward()),
		client.WithMetaHandler(transmeta.MetainfoClientHandler),
	}, nil
}

func kitexClientMetaForward() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			if traceID := mw.TraceIDFromContext(ctx); traceID != "" {
				ctx = mw.WithMeta(ctx, traceID, "", "", "")
			}
			ctx = metainfo.TransferForward(ctx)
			return next(ctx, req, resp)
		}
	}
}

func HertzServerOptions(cfg RegistryConfig, listenAddr string) ([]hertzconfig.Option, error) {
	if err := validateRegistryConfig(cfg, true); err != nil {
		return nil, err
	}
	addr := firstNonEmpty(cfg.Addr, listenAddr)
	if addr == "" {
		return nil, errors.New("registry addr is required")
	}
	weight := cfg.Weight
	if weight == 0 {
		weight = defaultWeight
	}
	r, err := hertzetcd.NewEtcdRegistry(cfg.Endpoints)
	if err != nil {
		return nil, err
	}
	return []hertzconfig.Option{
		hertzserver.WithRegistry(r, &hertzregistry.Info{
			ServiceName: cfg.ServiceName,
			Addr:        utils.NewNetAddr("tcp", addr),
			Weight:      weight,
			Tags:        cfg.Tags,
		}),
	}, nil
}

type HertzServiceResolver struct {
	serviceName string
	resolver    hertzdiscovery.Resolver
}

func NewHertzServiceResolver(cfg DiscoveryConfig, serviceName string) (*HertzServiceResolver, error) {
	if err := validateDiscoveryConfig(cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("service name is required")
	}
	r, err := hertzetcd.NewEtcdResolver(cfg.Endpoints)
	if err != nil {
		return nil, err
	}
	return &HertzServiceResolver{
		serviceName: serviceName,
		resolver:    r,
	}, nil
}

func (r *HertzServiceResolver) BaseURL(ctx context.Context) (string, error) {
	if r == nil || r.resolver == nil {
		return "", errors.New("hertz service resolver is not initialized")
	}
	result, err := r.resolver.Resolve(ctx, r.serviceName)
	if err != nil {
		return "", err
	}
	if len(result.Instances) == 0 {
		return "", fmt.Errorf("no instances found for service %q", r.serviceName)
	}
	addr := result.Instances[0].Address()
	if addr == nil || strings.TrimSpace(addr.String()) == "" {
		return "", fmt.Errorf("empty address found for service %q", r.serviceName)
	}
	return "http://" + addr.String(), nil
}

func validateRegistryConfig(cfg RegistryConfig, allowListenAddr bool) error {
	if !isEtcdType(cfg.Type) {
		return errors.New("registry type must be etcd")
	}
	if len(cfg.Endpoints) == 0 {
		return errors.New("registry endpoints are required")
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return errors.New("registry service_name is required")
	}
	if !allowListenAddr && strings.TrimSpace(cfg.Addr) == "" {
		return errors.New("registry addr is required")
	}
	return nil
}

func validateDiscoveryConfig(cfg DiscoveryConfig) error {
	if !isEtcdType(cfg.Type) {
		return errors.New("discovery type must be etcd")
	}
	if len(cfg.Endpoints) == 0 {
		return errors.New("discovery endpoints are required")
	}
	return nil
}

func isEtcdType(value string) bool {
	return value == "" || value == defaultRegistryType
}

func kitexPrefix(prefix string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		prefix = defaultPrefix
	}
	return prefix + "/kitex"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
