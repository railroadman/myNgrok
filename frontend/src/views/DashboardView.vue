<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { formatBytes } from '../lib/format'

type Tunnel = { id: string; status: 'open' | 'closed' }
type Agent = { id: string; connected: boolean }
type Token = { id: string; revokedAt?: string }
type Traffic = { requestsTotal: number; requestBytes: number; responseBytes: number }

const auth = useAuthStore()
const tunnels = ref<Tunnel[]>([])
const agents = ref<Agent[]>([])
const tokens = ref<Token[]>([])
const traffic = ref<Traffic>({ requestsTotal: 0, requestBytes: 0, responseBytes: 0 })
const loading = ref(true)

const activeTunnels = computed(() => tunnels.value.filter((t) => t.status === 'open').length)
const onlineAgents = computed(() => agents.value.filter((a) => a.connected).length)
const activeTokens = computed(() => tokens.value.filter((t) => !t.revokedAt).length)
const totalTransferred = computed(() => formatBytes(traffic.value.requestBytes + traffic.value.responseBytes))

async function load() {
  loading.value = true
  const headers = { Authorization: `Bearer ${auth.accessToken}` }
  const [t, a, k, tr] = await Promise.allSettled([
    api<Tunnel[]>('/api/v1/tunnels', { headers }),
    api<Agent[]>('/api/v1/agents', { headers }),
    api<Token[]>('/api/v1/agent-tokens', { headers }),
    api<Traffic>('/api/v1/traffic', { headers }),
  ])
  if (t.status === 'fulfilled') tunnels.value = t.value
  if (a.status === 'fulfilled') agents.value = a.value
  if (k.status === 'fulfilled') tokens.value = k.value
  if (tr.status === 'fulfilled') traffic.value = tr.value
  loading.value = false
}
onMounted(load)
</script>
<template>
  <section class="dashboard-page">
    <header><p class="eyebrow">Control plane</p><h1>Dashboard</h1></header>
    <p v-if="loading" aria-live="polite">Loading…</p>
    <div v-else class="agent-grid">
      <RouterLink class="panel stat-card" to="/app/tunnels">
        <p class="eyebrow">Tunnels</p>
        <h2>{{ activeTunnels }} active</h2>
        <p>{{ tunnels.length }} total</p>
      </RouterLink>
      <RouterLink class="panel stat-card" to="/app/agents">
        <p class="eyebrow">Agents</p>
        <h2>{{ onlineAgents }} online</h2>
        <p>{{ agents.length }} total</p>
      </RouterLink>
      <RouterLink class="panel stat-card" to="/app/tokens">
        <p class="eyebrow">Tokens</p>
        <h2>{{ activeTokens }} active</h2>
        <p>{{ tokens.length }} total</p>
      </RouterLink>
      <div class="panel stat-card">
        <p class="eyebrow">Traffic</p>
        <h2>{{ totalTransferred }} transferred</h2>
        <p>{{ formatBytes(traffic.requestBytes) }} in / {{ formatBytes(traffic.responseBytes) }} out</p>
      </div>
    </div>
  </section>
</template>
<style scoped>
.stat-card h2{margin:6px 0 2px}
</style>
