/**
 * Typed fetch client for the ReliefMesh REST API.
 *
 * - Same-origin only: cookies are sent with `credentials: 'same-origin'`.
 * - State-changing requests send the CSRF token in `X-CSRF-Token`.
 * - Network failures surface as `NetworkError` so callers can fall back to the
 *   offline queue; HTTP errors surface as `ApiError` with the server's code,
 *   message and per-field validation errors.
 */
import type {
  AdminUser,
  AidRequest,
  ApiErrorDetail,
  Assignment,
  AssignmentCreateInput,
  AssignmentProtected,
  AssignmentStatusInput,
  AssignmentUpdateInput,
  AuditEvent,
  AuditVerifyResult,
  Availability,
  Category,
  ClientSettings,
  Dashboard,
  DemoInfo,
  ListResult,
  Me,
  Note,
  NoteInput,
  Offer,
  OfferCreateInput,
  OfferStatus,
  PublicNotice,
  RequestCreateInput,
  RequestProtected,
  RequestStatusInput,
  RequestUpdateInput,
  RetentionPreview,
  Role,
  SessionResponse,
  Settings,
  SyncOperation,
  SyncPullResult,
  SyncPushResult,
  TimelineEvent,
  Volunteer,
} from '@reliefmesh/shared-types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly details: Record<string, unknown>
  readonly requestId?: string

  constructor(status: number, detail: ApiErrorDetail) {
    super(detail.message)
    this.name = 'ApiError'
    this.status = status
    this.code = detail.code
    this.fields = detail.fields ?? {}
    this.details = detail.details ?? {}
    this.requestId = detail.request_id
  }
}

/** Thrown when the server could not be reached at all. */
export class NetworkError extends Error {
  constructor(message = 'The server could not be reached.') {
    super(message)
    this.name = 'NetworkError'
  }
}

export function isNetworkError(e: unknown): e is NetworkError {
  return e instanceof NetworkError
}

export function isApiError(e: unknown, code?: string): e is ApiError {
  return e instanceof ApiError && (code === undefined || e.code === code)
}

export interface ClientOptions {
  /** Base URL, normally '' (same origin). */
  baseUrl?: string
  /** Returns the current CSRF token. */
  getCsrfToken?: () => string | null | undefined
  /** Called on 401 responses (session ended). */
  onUnauthorized?: (error: ApiError) => void
  /** Injected fetch (tests). */
  fetch?: typeof fetch
  /** Request timeout in milliseconds. */
  timeoutMs?: number
}

type Query = Record<string, string | number | boolean | undefined | null>

function qs(q?: Query): string {
  if (!q) return ''
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(q)) {
    if (v === undefined || v === null || v === '') continue
    params.set(k, String(v))
  }
  const s = params.toString()
  return s ? `?${s}` : ''
}

export interface RequestListQuery extends Query {
  status?: string
  category?: string
  urgency?: string
  q?: string
  open?: boolean
  mine?: boolean
  sort?: 'priority' | 'newest' | 'updated'
  updated_since?: string
  limit?: number
  offset?: number
}

export interface OfferListQuery extends Query {
  status?: string
  category?: string
  allocatable?: boolean
  limit?: number
  offset?: number
}

export interface AssignmentListQuery extends Query {
  request_id?: string
  offer_id?: string
  status?: string
  active?: boolean
  mine?: boolean
  limit?: number
}

export function createApiClient(opts: ClientOptions = {}) {
  const base = opts.baseUrl ?? ''
  const doFetch = opts.fetch ?? ((...args: Parameters<typeof fetch>) => fetch(...args))
  const timeoutMs = opts.timeoutMs ?? 20000

  async function request<T>(method: string, path: string, body?: unknown, raw = false): Promise<T> {
    const headers: Record<string, string> = { Accept: raw ? 'text/csv' : 'application/json' }
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    if (method !== 'GET') {
      const token = opts.getCsrfToken?.()
      if (token) headers['X-CSRF-Token'] = token
    }
    const controller = typeof AbortController !== 'undefined' ? new AbortController() : undefined
    const timer = controller ? setTimeout(() => controller.abort(), timeoutMs) : undefined
    let res: Response
    try {
      res = await doFetch(base + path, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        credentials: 'same-origin',
        cache: 'no-store',
        signal: controller?.signal,
      })
    } catch {
      throw new NetworkError()
    } finally {
      if (timer) clearTimeout(timer)
    }
    if (res.status === 204) return undefined as T
    // Gateways (reverse proxy without upstream) count as "server unreachable".
    if (res.status === 502 || res.status === 503 || res.status === 504) {
      const ct = res.headers.get('Content-Type') ?? ''
      if (!ct.includes('application/json')) throw new NetworkError('The ReliefMesh server is not responding.')
    }
    if (!res.ok) {
      let detail: ApiErrorDetail = { code: 'http_' + res.status, message: `Request failed (${res.status}).` }
      try {
        const data = (await res.json()) as { error?: ApiErrorDetail }
        if (data?.error) detail = data.error
      } catch {
        /* non-JSON error body */
      }
      const err = new ApiError(res.status, detail)
      if (res.status === 401) opts.onUnauthorized?.(err)
      throw err
    }
    if (raw) return (await res.text()) as T
    return (await res.json()) as T
  }

  const get = <T>(path: string) => request<T>('GET', path)
  const post = <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {})
  const patch = <T>(path: string, body: unknown) => request<T>('PATCH', path, body)
  const put = <T>(path: string, body: unknown) => request<T>('PUT', path, body)
  const id = (v: string) => encodeURIComponent(v)

  return {
    health: () => get<{ status: string; version: string }>('/healthz'),

    auth: {
      login: (username: string, password: string) => post<SessionResponse>('/api/v1/auth/login', { username, password }),
      logout: () => post<void>('/api/v1/auth/logout'),
      session: () => get<SessionResponse>('/api/v1/auth/session'),
      changePassword: (current_password: string, new_password: string) =>
        post<SessionResponse>('/api/v1/auth/change-password', { current_password, new_password }),
    },

    me: {
      get: () => get<Me>('/api/v1/me'),
      setAvailability: (availability: Availability, availability_note = '') =>
        patch<void>('/api/v1/me/availability', { availability, availability_note }),
    },

    publicNotice: () => get<PublicNotice>('/api/v1/public/notice'),
    settings: () => get<ClientSettings>('/api/v1/settings'),
    dashboard: () => get<Dashboard>('/api/v1/dashboard'),

    requests: {
      list: (q?: RequestListQuery) => get<ListResult<AidRequest>>('/api/v1/requests' + qs(q)),
      get: (rid: string) => get<AidRequest>(`/api/v1/requests/${id(rid)}`),
      create: (input: RequestCreateInput) => post<AidRequest>('/api/v1/requests', input),
      update: (rid: string, input: RequestUpdateInput) => patch<AidRequest>(`/api/v1/requests/${id(rid)}`, input),
      changeStatus: (rid: string, input: RequestStatusInput) => post<AidRequest>(`/api/v1/requests/${id(rid)}/status`, input),
      reopen: (rid: string, reason: string) => post<AidRequest>(`/api/v1/requests/${id(rid)}/reopen`, { reason }),
      revealProtected: (rid: string) => post<RequestProtected>(`/api/v1/requests/${id(rid)}/protected`),
      events: (rid: string) => get<ListResult<TimelineEvent>>(`/api/v1/requests/${id(rid)}/events`),
      notes: (rid: string) => get<ListResult<Note>>(`/api/v1/requests/${id(rid)}/notes`),
      addNote: (rid: string, input: NoteInput) => post<Note>(`/api/v1/requests/${id(rid)}/notes`, input),
    },

    offers: {
      list: (q?: OfferListQuery) => get<ListResult<Offer>>('/api/v1/offers' + qs(q)),
      get: (oid: string) => get<Offer>(`/api/v1/offers/${id(oid)}`),
      create: (input: OfferCreateInput) => post<Offer>('/api/v1/offers', input),
      update: (oid: string, input: Partial<OfferCreateInput> & { version?: number; verification_level?: string }) =>
        patch<Offer>(`/api/v1/offers/${id(oid)}`, input),
      changeStatus: (oid: string, status: OfferStatus, reason = '') =>
        post<Offer>(`/api/v1/offers/${id(oid)}/status`, { status, reason }),
      revealProtected: (oid: string) => post<RequestProtected>(`/api/v1/offers/${id(oid)}/protected`),
      events: (oid: string) => get<ListResult<AuditEvent>>(`/api/v1/offers/${id(oid)}/events`),
      notes: (oid: string) => get<ListResult<Note>>(`/api/v1/offers/${id(oid)}/notes`),
      addNote: (oid: string, input: NoteInput) => post<Note>(`/api/v1/offers/${id(oid)}/notes`, input),
    },

    assignments: {
      list: (q?: AssignmentListQuery) => get<ListResult<Assignment>>('/api/v1/assignments' + qs(q)),
      get: (aid: string) => get<Assignment>(`/api/v1/assignments/${id(aid)}`),
      create: (input: AssignmentCreateInput) => post<Assignment>('/api/v1/assignments', input),
      update: (aid: string, input: AssignmentUpdateInput) => patch<Assignment>(`/api/v1/assignments/${id(aid)}`, input),
      changeStatus: (aid: string, input: AssignmentStatusInput) =>
        post<Assignment>(`/api/v1/assignments/${id(aid)}/status`, input),
      revealProtected: (aid: string) => post<AssignmentProtected>(`/api/v1/assignments/${id(aid)}/protected`),
    },

    volunteers: {
      list: () => get<ListResult<Volunteer>>('/api/v1/volunteers'),
      setAvailability: (uid: string, availability: Availability, availability_note = '') =>
        patch<void>(`/api/v1/volunteers/${id(uid)}/availability`, { availability, availability_note }),
    },

    exports: {
      requestsCsv: (from?: string, to?: string) => request<string>('GET', '/api/v1/exports/requests.csv' + qs({ from, to }), undefined, true),
      summary: (from?: string, to?: string) => get<Record<string, unknown>>('/api/v1/exports/summary.json' + qs({ from, to })),
    },

    sync: {
      push: (operations: SyncOperation[]) => post<SyncPushResult>('/api/v1/sync/push', { operations }),
      pull: (since?: string) => get<SyncPullResult>('/api/v1/sync/pull' + qs({ since })),
    },

    admin: {
      updateSettings: (input: Partial<Settings>) => put<Settings>('/api/v1/admin/settings', input),
      categories: () => get<ListResult<Category>>('/api/v1/admin/categories'),
      updateCategory: (code: string, input: Partial<Pick<Category, 'label' | 'description' | 'enabled' | 'sort_order'>>) =>
        patch<Category>(`/api/v1/admin/categories/${id(code)}`, input),
      users: () => get<ListResult<AdminUser>>('/api/v1/admin/users'),
      createUser: (input: { username: string; display_name: string; roles: Role[] }) =>
        post<{ user: AdminUser; temporary_password: string }>('/api/v1/admin/users', input),
      updateUser: (uid: string, input: { display_name?: string; roles?: Role[]; is_active?: boolean; unlock?: boolean }) =>
        patch<AdminUser>(`/api/v1/admin/users/${id(uid)}`, input),
      resetPassword: (uid: string) => post<{ temporary_password: string }>(`/api/v1/admin/users/${id(uid)}/reset-password`),
      audit: (q?: { action?: string; entity_type?: string; before_id?: number; limit?: number }) =>
        get<ListResult<AuditEvent>>('/api/v1/admin/audit' + qs(q)),
      verifyAudit: () => get<AuditVerifyResult>('/api/v1/admin/audit/verify'),
      retentionPreview: () => get<RetentionPreview>('/api/v1/admin/retention'),
      applyRetention: () => post<RetentionPreview>('/api/v1/admin/retention/apply', { confirm: true }),
      deleteRecord: (entity_type: 'request' | 'offer', reference: string, reason: string) =>
        post<void>('/api/v1/admin/records/delete', { entity_type, reference, reason }),
      demo: () => get<DemoInfo>('/api/v1/admin/demo'),
      seedDemo: (scenario: string) => post<void>('/api/v1/admin/demo/seed', { scenario }),
    },
  }
}

export type ApiClient = ReturnType<typeof createApiClient>
