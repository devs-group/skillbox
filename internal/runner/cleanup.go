package runner

import (
	"context"
	"fmt"
	"log"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// CleanupOrphans finds and removes any containers with the label
// "managed-by=skillbox" that were left behind by previous server
// instances (e.g. after a crash or ungraceful shutdown). It is designed
// to be called once at server startup.
//
// Each orphaned container is force-removed. Errors removing individual
// containers are logged but do not stop the cleanup of remaining
// containers. A non-nil error is returned only if the container listing
// itself fails.
func CleanupOrphans(ctx context.Context, docker *client.Client) error {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "managed-by=skillbox")

	containers, err := docker.ContainerList(ctx, containertypes.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("listing orphaned skillbox containers: %w", err)
	}

	if len(containers) == 0 {
		return nil
	}

	log.Printf("cleanup: found %d orphaned skillbox container(s)", len(containers))

	var lastErr error
	for _, c := range containers {
		log.Printf("cleanup: removing orphaned container %s (image=%s, status=%s)",
			c.ID[:12], c.Image, c.Status)

		removeOpts := containertypes.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}
		if err := docker.ContainerRemove(ctx, c.ID, removeOpts); err != nil {
			log.Printf("cleanup: failed to remove container %s: %v", c.ID[:12], err)
			lastErr = err
			continue
		}

		log.Printf("cleanup: removed container %s", c.ID[:12])
	}

	if lastErr != nil {
		return fmt.Errorf("some orphaned containers could not be removed: %w", lastErr)
	}

	return nil
}
