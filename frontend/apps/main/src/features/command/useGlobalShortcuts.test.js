// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, h } from 'vue'

const userMock = vi.hoisted(() => ({ allowed: [] }))
const paletteMock = vi.hoisted(() => ({ openPalette: vi.fn() }))

vi.mock('vue-router', () => ({
  useRoute: () => ({ name: 'inbox-conversation', params: {} }),
  useRouter: () => ({ push: vi.fn() })
}))
vi.mock('@main/stores/conversation', () => ({
  useConversationStore: () => ({
    isConversationOpen: true,
    current: { uuid: 'c1', status: 'Open' },
    conversationsList: []
  })
}))
vi.mock('@main/stores/user', () => ({
  useUserStore: () => ({ can: (permission) => userMock.allowed.includes(permission) })
}))
vi.mock('@main/composables/useEmitter', () => ({ useEmitter: () => ({ emit: vi.fn() }) }))
vi.mock('@main/composables/useBulkActionPermissions', () => ({
  useBulkActionPermissions: () => ({ canBulkAct: { value: false } })
}))
vi.mock('./useCommandPalette', () => ({
  MACROS_COMMAND: 'macros',
  useCommandPalette: () => ({
    macroContext: { value: null },
    openPalette: paletteMock.openPalette,
    closePalette: vi.fn(),
    togglePalette: vi.fn()
  })
}))
vi.mock('./providers/useConversationCommands', () => ({ SNOOZE_COMMAND: 'conv.snooze' }))

import { permissions as perms } from '@main/constants/permissions'
import { useGlobalShortcuts } from './useGlobalShortcuts'

let app

const pressAlt = (code) => {
  const event = new KeyboardEvent('keydown', {
    code,
    key: code.slice(3).toLowerCase(),
    altKey: true,
    cancelable: true
  })
  window.dispatchEvent(event)
  return event
}

describe('useGlobalShortcuts', () => {
  beforeEach(() => {
    paletteMock.openPalette.mockClear()
    app = createApp({ setup: () => (useGlobalShortcuts(), () => h('div')) })
    app.mount(document.createElement('div'))
  })

  afterEach(() => app.unmount())

  it('opens the status picker on Alt+S', () => {
    userMock.allowed = [perms.CONVERSATIONS_UPDATE_STATUS]
    const event = pressAlt('KeyS')
    expect(paletteMock.openPalette).toHaveBeenCalledWith({ parent: 'conv.status' })
    expect(event.defaultPrevented).toBe(true)
  })

  it('leaves Alt+S alone without the status permission', () => {
    userMock.allowed = []
    const event = pressAlt('KeyS')
    expect(paletteMock.openPalette).not.toHaveBeenCalled()
    expect(event.defaultPrevented).toBe(false)
  })
})
