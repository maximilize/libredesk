// @vitest-environment jsdom
import { describe, expect, it, test } from 'vitest'
import {
  isUntouchedSuggestion,
  shouldOfferSuggestion,
  suggestionAuthorName,
  suggestionOriginal,
  suggestionTranslationOriginal
} from './draftSuggestion'

describe('shouldOfferSuggestion', () => {
  it('offers a suggestion for an empty reply box without a draft of its own', () => {
    expect(shouldOfferSuggestion({ type: 'reply', ownDraft: null, editorHtml: '' })).toBe(true)
    expect(
      shouldOfferSuggestion({
        type: 'reply',
        ownDraft: { content: '<p></p>' },
        editorHtml: '<p></p>'
      })
    ).toBe(true)
  })

  it("never replaces the agent's own draft or typed text", () => {
    expect(
      shouldOfferSuggestion({ type: 'reply', ownDraft: { content: '<p>mine</p>' }, editorHtml: '' })
    ).toBe(false)
    expect(
      shouldOfferSuggestion({ type: 'reply', ownDraft: null, editorHtml: '<p>typing</p>' })
    ).toBe(false)
  })

  it('only applies to replies, not private notes', () => {
    expect(shouldOfferSuggestion({ type: 'private_note', ownDraft: null, editorHtml: '' })).toBe(
      false
    )
  })
})

describe('isUntouchedSuggestion', () => {
  const suggestion = { content: '<p>Hello Clara</p>' }

  it('treats editor-normalized markup with the same text as untouched', () => {
    expect(isUntouchedSuggestion('<p>Hello Clara</p>', suggestion)).toBe(true)
    expect(isUntouchedSuggestion('<p class="x">Hello Clara</p>', suggestion)).toBe(true)
  })

  it("treats edited text as the agent's own draft", () => {
    expect(isUntouchedSuggestion('<p>Hello Clara, thanks</p>', suggestion)).toBe(false)
    expect(isUntouchedSuggestion('<p>Hello Clara</p>', null)).toBe(false)
  })
})

describe('suggestionAuthorName', () => {
  it('joins the author name and tolerates a missing one', () => {
    expect(suggestionAuthorName({ author: { first_name: 'AI', last_name: 'Prep' } })).toBe(
      'AI Prep'
    )
    expect(suggestionAuthorName({})).toBe('')
  })
})

describe('suggestion in the customer language', () => {
  const suggestion = {
    content: '<p>Bonjour Clara, nous vérifions le paiement.</p>',
    meta: {
      suggestion: true,
      language: 'fr',
      translation_original: { language: 'de', text: 'Hallo Clara, wir prüfen die Zahlung.' }
    }
  }

  test('exposes the team-language version', () => {
    expect(suggestionOriginal(suggestion)).toEqual({
      language: 'de',
      text: 'Hallo Clara, wir prüfen die Zahlung.'
    })
    expect(suggestionOriginal({ content: '<p>x</p>', meta: { suggestion: true } })).toBeNull()
  })

  test('sends it along only while the suggestion is unedited', () => {
    expect(
      suggestionTranslationOriginal('<p>Bonjour Clara, nous vérifions le paiement.</p>', suggestion)
    ).toEqual({
      language: 'de',
      text: 'Hallo Clara, wir prüfen die Zahlung.',
      sent_language: 'fr'
    })
    expect(
      suggestionTranslationOriginal('<p>Bonjour Clara, nous vérifions demain.</p>', suggestion)
    ).toBeNull()
    expect(suggestionTranslationOriginal('<p>x</p>', null)).toBeNull()
  })
})
