import { getTextFromHTML } from '@shared-ui/utils/string.js'

// A shared suggestion only prefills an empty reply box of an agent who has no reply draft of their own.
export const shouldOfferSuggestion = ({ type, ownDraft, editorHtml }) => {
  if (type !== 'reply') return false
  if (ownDraft && getTextFromHTML(ownDraft.content || '').length > 0) return false
  return getTextFromHTML(editorHtml || '').length === 0
}

// While the agent hasn't changed the prefilled text, it is still the suggestion and not their draft.
// Compares text, not markup: the editor normalizes the HTML it is given.
export const isUntouchedSuggestion = (html, suggestion) =>
  Boolean(suggestion) && getTextFromHTML(html || '') === getTextFromHTML(suggestion.content || '')

export const suggestionAuthorName = (suggestion) =>
  [suggestion?.author?.first_name, suggestion?.author?.last_name].filter(Boolean).join(' ')
