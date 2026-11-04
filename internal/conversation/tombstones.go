package conversation

import (
	"context"
	"errors"
	"time"
)

// RunMessageTombstoneCleaner drops the Message-IDs of deleted messages once they are older than 90 days,
// longer than any IMAP scan window, so the table doesn't grow forever.
func (c *Manager) RunMessageTombstoneCleaner(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.q.DeleteOldMessageTombstones.ExecContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
				c.lo.Error("error deleting old message tombstones", "error", err)
			}
		}
	}
}
