package registry

import (
	"context"
	"testing"

	"github.com/fitglue/server/src/go/internal/infra"
	pbsvc "github.com/fitglue/server/src/go/pkg/types/pb/services/registry"
)

func TestRegistryService(t *testing.T) {
	ctx := context.Background()
	logger := infra.NewLogger()

	store, err := NewStaticStore()
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}

	svc := NewService(store, logger)

	// Test ListCategories
	cats, err := svc.ListCategories(ctx, &pbsvc.ListCategoriesRequest{})
	if err != nil {
		t.Fatalf("ListCategories err: %v", err)
	}
	if len(cats.Categories) == 0 {
		t.Errorf("Expected some categories, got 0")
	}

	// Test ListPlugins
	pluginsResp, err := svc.ListPlugins(ctx, &pbsvc.ListPluginsRequest{})
	if err != nil {
		t.Fatalf("ListPlugins err: %v", err)
	}
	if len(pluginsResp.Plugins) < 10 {
		t.Errorf("Expected multiple plugins, got %d", len(pluginsResp.Plugins))
	}

	// Test ListPlugins with category filter
	if len(cats.Categories) > 0 {
		filterCat := cats.Categories[0]
		pluginsFiltered, err := svc.ListPlugins(ctx, &pbsvc.ListPluginsRequest{Category: filterCat})
		if err != nil {
			t.Fatalf("ListPlugins with category err: %v", err)
		}
		for _, p := range pluginsFiltered.Plugins {
			if p.GetCategory() != filterCat {
				t.Errorf("Expected category %s, got %s", filterCat, p.GetCategory())
			}
		}
	}

	// Test GetPlugin
	if len(pluginsResp.Plugins) > 0 {
		targetID := pluginsResp.Plugins[0].Id
		single, err := svc.GetPlugin(ctx, &pbsvc.GetPluginRequest{PluginId: targetID})
		if err != nil {
			t.Fatalf("GetPlugin err: %v", err)
		}
		if single.Id != targetID {
			t.Errorf("Expected %s, got %s", targetID, single.Id)
		}
	}

	// Test ListSources
	sources, err := svc.ListSources(ctx, &pbsvc.ListSourcesRequest{})
	if err != nil {
		t.Fatalf("ListSources err: %v", err)
	}
	if len(sources.Sources) == 0 {
		t.Errorf("Expected sources, got 0")
	}

	// Test ListDestinations
	dests, err := svc.ListDestinations(ctx, &pbsvc.ListDestinationsRequest{})
	if err != nil {
		t.Fatalf("ListDestinations err: %v", err)
	}
	if len(dests.Destinations) == 0 {
		t.Errorf("Expected destinations, got 0")
	}
}
