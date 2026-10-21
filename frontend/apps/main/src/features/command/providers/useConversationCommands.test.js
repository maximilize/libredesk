import { describe, it, expect } from 'vitest'
import {
  formatSnoozeDuration,
  moveInboxTargets
} from '@main/features/command/providers/useConversationCommands'

describe('formatSnoozeDuration', () => {
  it('keeps minutes below an hour', () => {
    expect(formatSnoozeDuration(30)).toBe('30m')
  })

  it('uses hours only for exact hours', () => {
    expect(formatSnoozeDuration(60)).toBe('1h')
    expect(formatSnoozeDuration(180)).toBe('3h')
  })

  it('keeps minutes when the duration is not a whole hour', () => {
    expect(formatSnoozeDuration(90)).toBe('90m')
    expect(formatSnoozeDuration(1445)).toBe('1445m')
  })
})

describe('moveInboxTargets', () => {
  const inboxes = [
    { id: 1, channel: 'email', enabled: true },
    { id: 2, channel: 'email', enabled: true },
    { id: 3, channel: 'email', enabled: false },
    { id: 4, channel: 'livechat', enabled: true }
  ]

  it('offers the other enabled email inboxes', () => {
    const targets = moveInboxTargets(inboxes, { inbox_id: 1, inbox_channel: 'email' })
    expect(targets.map((inbox) => inbox.id)).toEqual([2])
  })

  it('offers nothing for conversations outside email', () => {
    expect(moveInboxTargets(inboxes, { inbox_id: 4, inbox_channel: 'livechat' })).toEqual([])
    expect(moveInboxTargets(inboxes, null)).toEqual([])
  })
})
