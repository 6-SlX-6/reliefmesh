import { defineStore } from 'pinia'
import { isApiError, isNetworkError } from '@reliefmesh/api-client'
import type { Me, Role } from '@reliefmesh/shared-types'
import { type CachedSession, clearCaches, getDB, getMeta, metaKeys, setMeta } from '~/utils/offline-db'

type AuthStatus = 'unknown' | 'authenticated' | 'anonymous'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as Me | null,
    csrf: null as string | null,
    status: 'unknown' as AuthStatus,
    /** Signed in from the cached profile while the server is unreachable. */
    offlineSession: false,
    sessionExpired: false,
    initPromise: null as Promise<void> | null,
  }),
  getters: {
    roles: (s): Role[] => s.user?.roles ?? [],
    can: (s) => (capability: string) => s.user?.capabilities.includes(capability) ?? false,
    hasRole: (s) => (role: Role) => s.user?.roles.includes(role) ?? false,
    isCoordinator(): boolean {
      return this.can('request.manage')
    },
    isVolunteer(): boolean {
      return this.hasRole('volunteer')
    },
    isAdmin(): boolean {
      return this.can('user.manage')
    },
    /** Administrators without any operational role. */
    adminOnly(): boolean {
      return this.isAdmin && !this.can('request.create') && !this.can('request.read_all')
    },
    homePath(): string {
      if (!this.user) return '/login'
      if (this.user.must_change_password) return '/change-password'
      if (this.isCoordinator) return '/dashboard'
      if (this.adminOnly) return '/admin'
      if (this.isVolunteer) return '/assignments'
      return '/requests'
    },
  },
  actions: {
    init(): Promise<void> {
      if (!this.initPromise) this.initPromise = this.restore()
      return this.initPromise
    },
    async restore() {
      const api = useApi()
      try {
        const res = await api.auth.session()
        await this.setSession(res.user, res.csrf_token)
      } catch (e) {
        if (isNetworkError(e)) {
          const cached = await getMeta<CachedSession>(getDB(), metaKeys.session)
          if (cached?.user) {
            this.user = cached.user
            this.csrf = null
            this.status = 'authenticated'
            this.offlineSession = true
            return
          }
        }
        this.status = 'anonymous'
      }
    },
    async setSession(user: Me, csrf: string) {
      const db = getDB()
      const cached = await getMeta<CachedSession>(db, metaKeys.session)
      // Never show one person's cached records to another person.
      if (cached?.user && cached.user.id !== user.id) await clearCaches(db)
      this.user = user
      this.csrf = csrf
      this.status = 'authenticated'
      this.offlineSession = false
      this.sessionExpired = false
      await setMeta(db, metaKeys.session, { user, saved_at: new Date().toISOString() } satisfies CachedSession)
    },
    async login(username: string, password: string) {
      const res = await useApi().auth.login(username, password)
      await this.setSession(res.user, res.csrf_token)
      this.initPromise = Promise.resolve()
    },
    /** Re-validates an offline session once the server is reachable again. */
    async revalidate() {
      try {
        const res = await useApi().auth.session()
        await this.setSession(res.user, res.csrf_token)
      } catch (e) {
        if (isApiError(e) && e.status === 401) this.handleUnauthorized()
      }
    },
    handleUnauthorized() {
      if (this.status !== 'authenticated') return
      // Keep the cached profile id so the outbox stays attributable; the
      // user signs in again and pending changes are pushed afterwards.
      this.status = 'anonymous'
      this.csrf = null
      this.sessionExpired = true
      const route = useRoute()
      if (route.path !== '/login') {
        void navigateTo({ path: '/login', query: { expired: '1', redirect: route.fullPath } })
      }
    },
    /** Signs out and removes cached records. The caller must have handled
     * (confirmed or synchronized) pending outbox entries before. */
    async logout() {
      try {
        if (this.csrf) await useApi().auth.logout()
      } catch {
        /* offline: the server session expires on its own */
      }
      await clearCaches(getDB())
      this.user = null
      this.csrf = null
      this.status = 'anonymous'
      this.offlineSession = false
      this.initPromise = Promise.resolve()
    },
  },
})
