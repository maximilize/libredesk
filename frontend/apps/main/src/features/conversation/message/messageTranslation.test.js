import { describe, expect, test } from 'vitest'
import {
  customerLanguage,
  translationOriginalFor,
  translationView,
  withTranslation
} from './messageTranslation'

const msg = (type, meta = {}, text = 'Bonjour') => ({ type, text_content: text, meta })

describe('translationView', () => {
  test('shows a stored translation into the team language', () => {
    const view = translationView(
      msg('incoming', { language: 'fr', translations: { de: { text: 'Hallo', source: 'ai' } } }),
      'de'
    )
    expect(view).toMatchObject({ mode: 'show', from: 'fr', to: 'de', isOriginal: false })
    expect(view.translation.text).toBe('Hallo')
  })

  test("marks the agent's own original", () => {
    const view = translationView(
      msg('outgoing', {
        language: 'fr',
        translations: { de: { text: 'Hallo', source: 'original' } }
      }),
      'de'
    )
    expect(view).toMatchObject({ mode: 'show', isOriginal: true })
  })

  test('offers a translation for other languages and unknown customer messages', () => {
    expect(translationView(msg('incoming', { language: 'it' }), 'de')).toMatchObject({
      mode: 'offer',
      from: 'it'
    })
    expect(translationView(msg('outgoing', { language: 'it' }), 'de')).toMatchObject({
      mode: 'offer'
    })
    expect(translationView(msg('incoming'), 'de')).toMatchObject({ mode: 'offer', from: '' })
  })

  test('stays out of the way in the team language, when off, or without text', () => {
    expect(translationView(msg('incoming', { language: 'de' }), 'de').mode).toBe('none')
    expect(translationView(msg('outgoing'), 'de').mode).toBe('none')
    expect(translationView(msg('incoming', { language: 'fr' }), '').mode).toBe('none')
    expect(translationView(msg('activity', { language: 'fr' }), 'de').mode).toBe('none')
    expect(translationView(msg('incoming', { language: 'fr' }, '   '), 'de').mode).toBe('none')
  })
})

test('withTranslation keeps other languages and meta keys', () => {
  const meta = { echo_id: 'e1', translations: { en: { text: 'Hello' } } }
  expect(withTranslation(meta, 'de', { text: 'Hallo' })).toEqual({
    echo_id: 'e1',
    translations: { en: { text: 'Hello' }, de: { text: 'Hallo' } }
  })
  expect(meta.translations.de).toBeUndefined()
})

test('customerLanguage takes the newest incoming message with a language', () => {
  expect(
    customerLanguage([
      msg('incoming', { language: 'fr' }),
      msg('incoming', { language: 'it' }),
      msg('outgoing', { language: 'es' }),
      msg('incoming')
    ])
  ).toBe('it')
  expect(customerLanguage([msg('outgoing', { language: 'fr' })])).toBe('')
  expect(customerLanguage(undefined)).toBe('')
})

describe('translationOriginalFor', () => {
  const state = {
    conversationUUID: 'c1',
    language: 'de',
    text: 'Hallo Clara, wir prüfen das.',
    sentLanguage: 'fr',
    translatedText: 'Bonjour Clara,  nous vérifions.'
  }

  test('sends the original with an unedited translation', () => {
    expect(translationOriginalFor(state, 'c1', 'Bonjour Clara, nous vérifions.')).toEqual({
      language: 'de',
      text: 'Hallo Clara, wir prüfen das.',
      sent_language: 'fr'
    })
  })

  test('drops it once the translation was edited, or in another conversation', () => {
    expect(translationOriginalFor(state, 'c1', 'Bonjour Clara, nous vérifions demain.')).toBeNull()
    expect(translationOriginalFor(state, 'c2', 'Bonjour Clara, nous vérifions.')).toBeNull()
    expect(translationOriginalFor(null, 'c1', 'x')).toBeNull()
  })
})
