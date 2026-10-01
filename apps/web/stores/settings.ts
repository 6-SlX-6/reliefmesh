import { defineStore } from 'pinia'
import type { Category, ClientSettings, PublicNotice, Settings } from '@reliefmesh/shared-types'
import { cachedSettings, getDB, getMeta, metaKeys, setMeta } from '~/utils/offline-db'
import { categoryFallbackLabel } from '~/utils/labels'

/** Shown if the server has never been reached on this device. */
export const FALLBACK_EMERGENCY_NOTICE =
  'Emergency notice: If there is immediate danger, contact your local emergency services. In the EU, call 112 where available. ReliefMesh is not an emergency dispatch service.'

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    settings: null as Settings | null,
    categories: [] as Category[],
    notice: null as PublicNotice | null,
  }),
  getters: {
    emergencyNotice: (s) => s.settings?.emergency_notice ?? s.notice?.emergency_notice ?? FALLBACK_EMERGENCY_NOTICE,
    instanceName: (s) => s.settings?.instance_name ?? s.notice?.instance_name ?? 'ReliefMesh',
    exerciseMode: (s) => s.settings?.exercise_mode ?? s.notice?.exercise_mode ?? false,
    exerciseLabel: (s) => s.settings?.exercise_label ?? s.notice?.exercise_label ?? 'EXERCISE',
    approxDecimals: (s) => s.settings?.approx_location_decimals ?? 2,
    categoryLabel: (s) => (code: string) =>
      s.categories.find((c) => c.code === code)?.label ?? categoryFallbackLabel[code] ?? code,
  },
  actions: {
    apply(cs: ClientSettings) {
      this.settings = cs.settings
      this.categories = cs.categories
    },
    async loadPublic() {
      const db = getDB()
      try {
        this.notice = await useApi().publicNotice()
        await setMeta(db, metaKeys.publicNotice, this.notice)
      } catch {
        this.notice = (await getMeta<PublicNotice>(db, metaKeys.publicNotice)) ?? null
      }
    },
    async load() {
      const db = getDB()
      try {
        const cs = await useApi().settings()
        this.apply(cs)
        await setMeta(db, metaKeys.settings, cs)
      } catch {
        const cs = await cachedSettings(db)
        if (cs) this.apply(cs)
      }
    },
  },
})
