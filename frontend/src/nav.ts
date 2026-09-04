export type NavItem = { label: string; to: string; icon: string }
export type NavGroup = { label: string; items: NavItem[] }

export const navGroups: NavGroup[] = [
  { label: 'Overview', items: [
    { label: 'Dashboard', to: '/app', icon: 'dashboard' },
  ] },
  { label: 'Connectivity', items: [
    { label: 'Tunnels', to: '/app/tunnels', icon: 'tunnels' },
    { label: 'Agents', to: '/app/agents', icon: 'agents' },
  ] },
  { label: 'Access', items: [
    { label: 'Tokens', to: '/app/tokens', icon: 'tokens' },
  ] },
]
