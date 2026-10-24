// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, ref } from 'vue'

const drafts = vi.hoisted(() => new Map())
const apiMock = vi.hoisted(() => ({
  getDraftSuggestion: vi.fn(),
  deleteDraftSuggestion: vi.fn(),
  saveDraft: vi.fn(),
  deleteDraft: vi.fn()
}))

vi.mock('@main/api', () => ({ default: apiMock }))
vi.mock('@main/stores/conversation', () => ({
  useConversationStore: () => ({
    draftsReady: Promise.resolve(),
    macros: {},
    getMacro: () => null,
    getDraft: (uuid, type) => drafts.get(`${uuid}::${type}`) || null,
    setDraft: (uuid, type, d) => drafts.set(`${uuid}::${type}`, d),
    removeDraft: (uuid, type) => drafts.delete(`${uuid}::${type}`)
  })
}))

import { useDraftManager } from './useDraftManager'

const flush = async () => {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve()
    await nextTick()
  }
}

let app
const mount = () => {
  let manager
  app = createApp({
    setup () {
      manager = useDraftManager(ref('c1'), ref('reply'))
      return () => h('div')
    }
  })
  app.mount(document.createElement('div'))
  return manager
}

describe('useDraftManager shared suggestion', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    drafts.clear()
    Object.values(apiMock).forEach(fn => fn.mockReset().mockResolvedValue({ data: { data: true } }))
  })
  afterEach(() => {
    app?.unmount()
    vi.useRealTimers()
  })

  it('prefills an empty reply box and does not save the untouched suggestion as the agent\'s draft', async () => {
    apiMock.getDraftSuggestion.mockResolvedValue({
      data: { data: { content: '<p>Hello Clara</p>', author: { first_name: 'AI' } } }
    })
    const m = mount()
    await flush()
    expect(m.htmlContent.value).toBe('<p>Hello Clara</p>')
    expect(m.suggestion.value?.author?.first_name).toBe('AI')
    await vi.advanceTimersByTimeAsync(600)
    expect(apiMock.saveDraft).not.toHaveBeenCalled()
  })

  it('keeps the agent\'s own draft and does not ask for a suggestion', async () => {
    drafts.set('c1::reply', { content: '<p>my draft</p>', meta: {} })
    const m = mount()
    await flush()
    expect(m.htmlContent.value).toBe('<p>my draft</p>')
    expect(apiMock.getDraftSuggestion).not.toHaveBeenCalled()
    expect(m.suggestion.value).toBeNull()
  })

  it('discard clears the box and deletes the suggestion', async () => {
    apiMock.getDraftSuggestion.mockResolvedValue({ data: { data: { content: '<p>Hello Clara</p>' } } })
    const m = mount()
    await flush()
    m.discardSuggestion()
    await flush()
    expect(m.htmlContent.value).toBe('')
    expect(m.suggestion.value).toBeNull()
    expect(apiMock.deleteDraftSuggestion).toHaveBeenCalledWith('c1')
  })

  it('an edited suggestion becomes the agent\'s own draft', async () => {
    apiMock.getDraftSuggestion.mockResolvedValue({ data: { data: { content: '<p>Hello Clara</p>' } } })
    const m = mount()
    await flush()
    m.htmlContent.value = '<p>Hello Clara, here you go</p>'
    await flush()
    await vi.advanceTimersByTimeAsync(600)
    expect(m.suggestion.value).toBeNull()
    expect(apiMock.saveDraft).toHaveBeenCalledWith('c1', 'reply', expect.objectContaining({ content: '<p>Hello Clara, here you go</p>' }))
  })
})
