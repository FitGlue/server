package registry

import (
	"context"

	"github.com/fitglue/server/src/go/internal/infra"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	pbsvc "github.com/fitglue/server/src/go/pkg/types/pb/services/registry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RegistryService struct {
	pbsvc.UnimplementedRegistryServiceServer
	store  Store
	logger infra.Logger
}

func NewService(store Store, logger infra.Logger) *RegistryService {
	return &RegistryService{
		store:  store,
		logger: logger,
	}
}

func (s *RegistryService) ListPlugins(ctx context.Context, req *pbsvc.ListPluginsRequest) (*pbsvc.ListPluginsResponse, error) {
	plugins, err := s.store.ListPlugins(ctx, req.Category)
	if err != nil {
		s.logger.Error(ctx, "Failed to list plugins", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list plugins: %v", err)
	}
	return &pbsvc.ListPluginsResponse{Plugins: plugins}, nil
}

func (s *RegistryService) GetPlugin(ctx context.Context, req *pbsvc.GetPluginRequest) (*pbplugin.PluginManifest, error) {
	if req.PluginId == "" {
		return nil, status.Error(codes.InvalidArgument, "plugin_id is required")
	}
	plugin, err := s.store.GetPlugin(ctx, req.PluginId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "plugin not found: %s", req.PluginId)
	}
	return plugin, nil
}

func (s *RegistryService) ListCategories(ctx context.Context, req *pbsvc.ListCategoriesRequest) (*pbsvc.ListCategoriesResponse, error) {
	categories, err := s.store.ListCategories(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list categories", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list categories: %v", err)
	}
	return &pbsvc.ListCategoriesResponse{Categories: categories}, nil
}

func (s *RegistryService) GetPluginIcon(ctx context.Context, req *pbsvc.GetPluginIconRequest) (*pbsvc.GetPluginIconResponse, error) {
	if req.PluginId == "" {
		return nil, status.Error(codes.InvalidArgument, "plugin_id is required")
	}
	iconData, contentType, err := s.store.GetPluginIcon(ctx, req.PluginId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "icon not found for plugin: %s", req.PluginId)
	}
	return &pbsvc.GetPluginIconResponse{
		IconData:    iconData,
		ContentType: contentType,
	}, nil
}

func (s *RegistryService) ListSources(ctx context.Context, req *pbsvc.ListSourcesRequest) (*pbsvc.ListSourcesResponse, error) {
	sources, err := s.store.ListSources(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list sources", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list sources: %v", err)
	}
	return &pbsvc.ListSourcesResponse{Sources: sources}, nil
}

func (s *RegistryService) ListDestinations(ctx context.Context, req *pbsvc.ListDestinationsRequest) (*pbsvc.ListDestinationsResponse, error) {
	destinations, err := s.store.ListDestinations(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list destinations", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list destinations: %v", err)
	}
	return &pbsvc.ListDestinationsResponse{Destinations: destinations}, nil
}

func (s *RegistryService) GetPluginRegistry(ctx context.Context, req *pbsvc.GetPluginRegistryRequest) (*pbplugin.PluginRegistryResponse, error) {
	registry, err := s.store.GetFullRegistry(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to get plugin registry", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get plugin registry: %v", err)
	}
	return registry, nil
}
