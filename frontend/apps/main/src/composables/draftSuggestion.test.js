// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { isUntouchedSuggestion, shouldOfferSuggestion, suggestionAuthorName } from './draftSuggestion'

describe('shouldOfferSuggestion', () => {
  it('offers a suggestion for an empty reply box without a draft of its own', () => {
    expect(shouldOfferSuggestion({ type: 'reply', ownDraft: null, editorHtml: '' })).toBe(true)
    expect(shouldOfferSuggestion({ type: 'reply', ownDraft: { content: '<p></p>' }, editorHtml: '<p></p>' })).toBe(true)
  })

  it('never replaces the agent\'s own draft or typed text', () => {
    expect(shouldOfferSuggestion({ type: 'reply', ownDraft: { content: '<p>mine</p>' }, editorHtml: '' })).toBe(false)
    expect(shouldOfferSuggestion({ type: 'reply', ownDraft: null, editorHtml: '<p>typing</p>' })).toBe(false)
  })

  it('only applies to replies, not private notes', () => {
    expect(shouldOfferSuggestion({ type: 'private_note', ownDraft: null, editorHtml: '' })).toBe(false)
  })
})

describe('isUntouchedSuggestion', () => {
  const suggestion = { content: '<p>Hello Clara</p>' }

  it('treats editor-normalized markup with the same text as untouched', () => {
    expect(isUntouchedSuggestion('<p>Hello Clara</p>', suggestion)).toBe(true)
    expect(isUntouchedSuggestion('<p class="x">Hello Clara</p>', suggestion)).toBe(true)
  })

  it('treats edited text as the agent\'s own draft', () => {
    expect(isUntouchedSuggestion('<p>Hello Clara, thanks</p>', suggestion)).toBe(false)
    expect(isUntouchedSuggestion('<p>Hello Clara</p>', null)).toBe(false)
  })
})

describe('suggestionAuthorName', () => {
  it('joins the author name and tolerates a missing one', () => {
    expect(suggestionAuthorName({ author: { first_name: 'AI', last_name: 'Prep' } })).toBe('AI Prep')
    expect(suggestionAuthorName({})).toBe('')
  })
})
