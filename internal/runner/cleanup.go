package runner

import (
	"context"
	"fmt"
	"log"

	"github.com/devs-group/skillbox/internal/sandbox"
)

// CleanupOrphans finds and removes any OpenSandbox sandboxes with the
// metadata "managed-by=skillbox" that were left behind by previous server
// instances (e.g. after a crash or ungraceful shutdown). It is designed
// to be called once at server startup.
//
// Each orphaned sandbox is deleted. Errors removing individual sandboxes
// are logged but do not stop the cleanup of remaining sandboxes. A non-nil
// error is returned only if the sandbox listing itself fails.
func CleanupOrphans(ctx context.Context, sb *sandbox.Client) error {
	sandboxes, err := sb.ListSandboxes(ctx, map[string]string{
		"managed-by": "skillbox",
	})
	if err != nil {
		return fmt.Errorf("listing orphaned skillbox sandboxes: %w", err)
	}

	if len(sandboxes) == 0 {
		return nil
	}

	log.Printf("cleanup: found %d orphaned skillbox sandbox(es)", len(sandboxes))

	var lastErr error
	for _, s := range sandboxes {
		log.Printf("cleanup: removing orphaned sandbox %s (state=%s)",
			shortID(s.ID), s.State)

		if err := sb.DeleteSandbox(ctx, s.ID); err != nil {
			log.Printf("cleanup: failed to remove sandbox %s: %v", shortID(s.ID), err)
			lastErr = err
			continue
		}

		log.Printf("cleanup: removed sandbox %s", shortID(s.ID))
	}

	if lastErr != nil {
		return fmt.Errorf("some orphaned sandboxes could not be removed: %w", lastErr)
	}

	return nil
}
