// Decides how a message's translation shows in its bubble.
//   show:  a stored translation into the team language (or the agent's original before translating)
//   offer: a "Translate" link, for messages in another language, or customer messages of unknown language
//   none:  nothing, e.g. messages already in the team language or translations turned off
export function translationView(message, teamLanguage) {
  const none = { mode: 'none' }
  if (!teamLanguage || !message || message.type === 'activity') return none
  if (!(message.text_content || '').trim()) return none
  const meta = message.meta || {}
  const language = meta.language || ''
  const translation = meta.translations?.[teamLanguage]
  if (translation?.text) {
    return {
      mode: 'show',
      translation,
      from: language || translation.from || '',
      to: teamLanguage,
      isOriginal: translation.source === 'original'
    }
  }
  if (language === teamLanguage) return none
  if (language) return { mode: 'offer', from: language, to: teamLanguage }
  return message.type === 'incoming' ? { mode: 'offer', from: '', to: teamLanguage } : none
}

// withTranslation returns message meta with a translation stored under language, leaving the rest as is.
export function withTranslation(meta, language, translation) {
  const current = meta || {}
  return { ...current, translations: { ...(current.translations || {}), [language]: translation } }
}

// customerLanguage returns the language of the latest incoming message that has one, '' when none does.
export function customerLanguage(messages) {
  for (let i = (messages || []).length - 1; i >= 0; i--) {
    const m = messages[i]
    if (m?.type === 'incoming' && m.meta?.language) return m.meta.language
  }
  return ''
}

const normalizeText = (s) => (s || '').replace(/\s+/g, ' ').trim()

// translationOriginalFor returns the translation_original payload for a reply, or null. The agent's
// pre-translation text is only sent when the reply still is the unedited translation, so a stored
// "original" never describes text that was changed afterwards.
export function translationOriginalFor(state, conversationUUID, sentText) {
  if (!state || state.conversationUUID !== conversationUUID) return null
  if (
    !normalizeText(state.text) ||
    normalizeText(sentText) !== normalizeText(state.translatedText)
  ) {
    return null
  }
  return { language: state.language, text: state.text, sent_language: state.sentLanguage }
}
