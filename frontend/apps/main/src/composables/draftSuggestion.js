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

// The team-language version of an AI suggestion written in the customer's language, if it has one.
export const suggestionOriginal = (suggestion) => {
  const o = suggestion?.meta?.translation_original
  return o?.text && o?.language ? o : null
}

// suggestionTranslationOriginal returns the translation_original payload for sending an unedited
// suggestion, so its team-language version is stored with the reply; null otherwise.
export const suggestionTranslationOriginal = (html, suggestion) => {
  const o = suggestionOriginal(suggestion)
  if (!o || !isUntouchedSuggestion(html, suggestion)) return null
  return { language: o.language, text: o.text, sent_language: suggestion.meta?.language || '' }
}
