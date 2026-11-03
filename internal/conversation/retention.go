package conversation

import (
	"context"
	"errors"
	"time"

	"github.com/lib/pq"
)

// retentionBatchSize bounds one delete so a large backlog doesn't hold locks for long.
const retentionBatchSize = 500

// RetentionPolicy says how long resolved conversations are kept. Days is the general period, 0 keeps
// everything. Conversations carrying one of Tags are kept for TaggedDays instead; 0 keeps them.
// With DeleteContacts, a contact goes along with their last conversation.
type RetentionPolicy struct {
	Days           int
	DeleteContacts bool
	Tags           []string
	TaggedDays     int
}

// RunRetention deletes resolved conversations older than the retention period once an hour.
// The policy is read on every run as days, the contact switch, tags and tagged days, so a changed
// setting applies without a restart.
func (c *Manager) RunRetention(ctx context.Context, policy func() (days int, deleteContacts bool, tags []string, taggedDays int)) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			days, deleteContacts, tags, taggedDays := policy()
			p := RetentionPolicy{Days: days, DeleteContacts: deleteContacts, Tags: tags, TaggedDays: taggedDays}
			if p.Days <= 0 {
				continue
			}
			conversations, contacts, err := c.DeleteExpiredConversations(ctx, p)
			if err != nil && !errors.Is(err, context.Canceled) {
				c.lo.Error("error deleting expired conversations", "error", err)
			}
			if conversations > 0 {
				c.lo.Info("deleted expired conversations", "conversations", conversations, "contacts", contacts, "retention_days", p.Days)
			}
		}
	}
}

// DeleteExpiredConversations deletes resolved conversations that have had no message and no status change
// for longer than the policy allows, in batches. With DeleteContacts, contacts and visitors left without any
// conversation are deleted too; their avatars go with the unlinked-media sweep. Each batch is one
// transaction, so a failure leaves nothing half done and the next run picks the batch up again.
// Returns how many conversations and contacts went.
func (c *Manager) DeleteExpiredConversations(ctx context.Context, p RetentionPolicy) (conversations, contacts int, err error) {
	for {
		candidates, n, m, err := c.deleteExpiredBatch(ctx, p)
		conversations += n
		contacts += m
		if err != nil || candidates < retentionBatchSize || n == 0 {
			return conversations, contacts, err
		}
	}
}

// deleteExpiredBatch deletes one batch of expired conversations and, if asked, the contacts they leave
// without any conversation. Candidates are locked before the delete, so a message being inserted into one
// of them commits first and the delete sees it; the contacts likewise, so a conversation created for one
// of them meanwhile waits on its foreign key and keeps the contact.
func (c *Manager) deleteExpiredBatch(ctx context.Context, p RetentionPolicy) (candidates, conversations, contacts int, err error) {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback()

	var ids, locked []int64
	if err := tx.Stmtx(c.q.GetExpiredConversations).SelectContext(ctx, &ids, p.Days, retentionBatchSize, pq.Array(tags), p.TaggedDays); err != nil {
		return 0, 0, 0, err
	}
	if len(ids) == 0 {
		return 0, 0, 0, nil
	}
	if err := tx.Stmtx(c.q.LockConversations).SelectContext(ctx, &locked, pq.Array(ids)); err != nil {
		return 0, 0, 0, err
	}
	var contactIDs []int64
	if err := tx.Stmtx(c.q.DeleteExpiredConversations).SelectContext(ctx, &contactIDs, pq.Array(ids), p.Days); err != nil {
		return 0, 0, 0, err
	}
	if p.DeleteContacts && len(contactIDs) > 0 {
		var deleted []int64
		if err := tx.Stmtx(c.q.LockContacts).SelectContext(ctx, &locked, pq.Array(contactIDs)); err != nil {
			return 0, 0, 0, err
		}
		if err := tx.Stmtx(c.q.DeleteContactsWithoutConversations).SelectContext(ctx, &deleted, pq.Array(contactIDs)); err != nil {
			return 0, 0, 0, err
		}
		contacts = len(deleted)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, 0, err
	}
	return len(ids), len(contactIDs), contacts, nil
}
