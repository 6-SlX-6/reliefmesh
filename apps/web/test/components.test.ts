import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import StatusBadge from '../components/common/StatusBadge.vue'
import UrgencyBadge from '../components/common/UrgencyBadge.vue'
import EmergencyNotice from '../components/common/EmergencyNotice.vue'
import ConnectionStatus from '../components/common/ConnectionStatus.vue'
import CriticalNoticeDialog from '../components/requests/CriticalNoticeDialog.vue'
import { FALLBACK_EMERGENCY_NOTICE, useSettingsStore } from '../stores/settings'
import { useSyncStore } from '../stores/sync'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('StatusBadge', () => {
  it('conveys status with text, not only color', () => {
    const w = mount(StatusBadge, { props: { kind: 'request', status: 'under_review' } })
    expect(w.text()).toContain('Status:')
    expect(w.text()).toContain('Under review')
    expect(w.find('svg').exists()).toBe(true)
  })
  it('marks records waiting for synchronization', () => {
    const w = mount(StatusBadge, { props: { kind: 'offer', status: 'available', pending: true } })
    expect(w.text()).toContain('Waiting to sync')
  })
})

describe('UrgencyBadge', () => {
  it('labels urgency for screen readers', () => {
    const w = mount(UrgencyBadge, { props: { urgency: 'critical' } })
    expect(w.text()).toContain('Urgency:')
    expect(w.text()).toContain('Critical')
  })
})

describe('EmergencyNotice', () => {
  it('shows the EU default until settings are loaded', () => {
    const w = mount(EmergencyNotice)
    expect(w.text()).toContain('112')
    expect(w.text()).toContain(FALLBACK_EMERGENCY_NOTICE.slice(0, 40))
  })
  it('shows the configured deployment notice', () => {
    const settings = useSettingsStore()
    settings.notice = { instance_name: 'X', emergency_notice: 'Call 911 in an emergency. ReliefMesh is not a dispatch service.', exercise_mode: false, exercise_label: '' }
    const w = mount(EmergencyNotice)
    expect(w.text()).toContain('Call 911')
    expect(w.attributes('aria-label')).toBe('Emergency notice')
  })
})

describe('ConnectionStatus', () => {
  it('distinguishes offline, unreachable and pending changes', async () => {
    const sync = useSyncStore()
    sync.browserOnline = false
    sync.pending = 2
    const w = mount(ConnectionStatus)
    expect(w.get('[data-testid=connection-label]').text()).toBe('Offline')
    expect(w.text()).toContain('2 waiting')
    sync.browserOnline = true
    sync.serverReachable = false
    await w.vm.$nextTick()
    expect(w.get('[data-testid=connection-label]').text()).toBe('Server unreachable')
  })
})

describe('CriticalNoticeDialog', () => {
  it('states that emergency services are not notified', () => {
    const settings = useSettingsStore()
    settings.settings = { critical_urgency_notice: 'Call emergency services now.' } as never
    const w = mount(CriticalNoticeDialog, { props: { open: true }, attachTo: document.body })
    expect(w.text()).toContain('Call emergency services now.')
    expect(w.text()).toContain('does not notify emergency services')
    w.unmount()
  })
})
