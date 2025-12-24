package infrastructure

import (
	"context"
	"fmt"

	"github.com/arafato/cf-nuke/types"
	"golang.org/x/sync/errgroup"
)

func RemoveCollection(ctx context.Context, resources types.Resources) error {
	purgeGroup, _ := errgroup.WithContext(ctx)

	for _, resource := range resources {
		resource := resource
		if resource.State == types.Filtered || resource.State == types.Hidden {
			continue
		}
		if resource.ProductName != "CachePurge" {
			continue
		}
		purgeGroup.Go(func() error {
			return resource.Remove()
		})
	}

	purgeErr := purgeGroup.Wait()

	g, _ := errgroup.WithContext(ctx)
	for _, resource := range resources {
		resource := resource
		if resource.State == types.Filtered || resource.State == types.Hidden {
			continue
		}
		if resource.ProductName == "CachePurge" {
			continue
		}
		g.Go(func() error {
			return resource.Remove()
		})
	}

	mainErr := g.Wait()
	if purgeErr != nil && mainErr != nil {
		return fmt.Errorf("cache purge error: %v; remove error: %v", purgeErr, mainErr)
	}
	if purgeErr != nil {
		return purgeErr
	}
	return mainErr
}
