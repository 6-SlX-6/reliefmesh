export interface NavItem {
  to: string
  label: string
  icon: string
  primary?: boolean
}

/** Navigation entries derived from the user's capabilities. */
export function useNavigation() {
  const auth = useAuthStore()
  return computed<NavItem[]>(() => {
    const items: NavItem[] = []
    if (!auth.user) return items
    if (auth.isCoordinator) items.push({ to: '/dashboard', label: 'Dashboard', icon: 'activity', primary: true })
    if (auth.adminOnly) items.push({ to: '/admin', label: 'Administration', icon: 'shield', primary: true })
    if (auth.can('assignment.work_own') && !auth.isCoordinator) items.push({ to: '/assignments', label: 'My tasks', icon: 'truck', primary: true })
    if (auth.can('request.read_all')) items.push({ to: '/requests', label: 'Requests', icon: 'inbox', primary: true })
    else if (auth.can('request.create')) items.push({ to: '/requests', label: 'My requests', icon: 'inbox', primary: true })
    if (auth.can('offer.read_all')) items.push({ to: '/offers', label: 'Offers', icon: 'package', primary: true })
    else if (auth.can('offer.create')) items.push({ to: '/offers', label: 'My offers', icon: 'package' })
    if (auth.can('assignment.manage')) items.push({ to: '/assignments', label: 'Assignments', icon: 'truck' })
    if (auth.can('volunteer.list')) items.push({ to: '/volunteers', label: 'Volunteers', icon: 'users' })
    if (auth.can('export.reports')) items.push({ to: '/reports', label: 'Reports', icon: 'download' })
    if (auth.isAdmin && !auth.adminOnly) items.push({ to: '/admin', label: 'Administration', icon: 'shield' })
    items.push({ to: '/sync', label: 'Pending changes', icon: 'refresh' })
    items.push({ to: '/account', label: 'My account', icon: 'user' })
    items.push({ to: '/about', label: 'About & emergency info', icon: 'info' })
    return items
  })
}
