// Route guard. It only controls navigation for usability; every permission
// is enforced again by the API.
const PUBLIC_PATHS = ['/login', '/about']

export default defineNuxtRouteMiddleware(async (to) => {
  const auth = useAuthStore()
  await auth.init()

  if (auth.status !== 'authenticated') {
    if (PUBLIC_PATHS.includes(to.path)) return
    return navigateTo({ path: '/login', query: to.fullPath !== '/' ? { redirect: to.fullPath } : {} })
  }
  if (auth.user?.must_change_password && to.path !== '/change-password' && !PUBLIC_PATHS.includes(to.path)) {
    return navigateTo('/change-password')
  }
  if (to.path === '/login') return navigateTo(auth.homePath)
  const requires = to.meta.requires as string | string[] | undefined
  if (requires) {
    const caps = Array.isArray(requires) ? requires : [requires]
    if (!caps.some((c) => auth.can(c))) return navigateTo(auth.homePath)
  }
})
