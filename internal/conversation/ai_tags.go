package conversation

import (
	"context"
	"errors"
	"strings"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
)

// maxTagTranscriptMessages bounds the transcript handed to the tag suggester.
const maxTagTranscriptMessages = 50

// SetTagSuggester sets the AI that the ai_tags automation action asks for tags.
func (m *Manager) SetTagSuggester(s TagSuggester) {
	m.tagSuggester = s
}

// AITags returns the tags the suggester picks for the conversation's public transcript, nil when there is
// nothing to read yet. Private notes stay out of the transcript.
func (m *Manager) AITags(ctx context.Context, conversationUUID string) ([]string, error) {
	if m.tagSuggester == nil {
		return nil, errors.New("no tag suggester configured")
	}
	private := false
	msgs, err := m.GetAllConversationMessages(conversationUUID, &private, []string{models.MessageIncoming, models.MessageOutgoing}, maxTagTranscriptMessages)
	if err != nil {
		return nil, err
	}
	transcript := models.Transcript(msgs, maxTagTranscriptMessages)
	if strings.TrimSpace(transcript) == "" {
		return nil, nil
	}
	return m.tagSuggester.SuggestTags(ctx, transcript)
}
