<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

type Tunnel = { id: string; subdomain: string; localAddress: string; status: 'open' | 'closed'; agentId: string; openedAt?: string; closedAt?: string }
const auth = useAuthStore()
const tunnels = ref<Tunnel[]>([])
const error = ref('')
const loading = ref(true)
const activeCount = computed(() => tunnels.value.filter((tunnel) => tunnel.status === 'open').length)
async function load() {
  loading.value = true; error.value = ''
  try { tunnels.value = await api<Tunnel[]>('/api/v1/tunnels', { headers: { Authorization: `Bearer ${auth.accessToken}` } }) }
  catch (e: any) { error.value = e.message ?? 'Unable to load tunnels' }
  finally { loading.value = false }
}
onMounted(load)
</script>
<template>
  <section class="tunnels-page" aria-labelledby="tunnels-heading">
    <nav><RouterLink to="/app">Dashboard</RouterLink> · <RouterLink to="/app/agents">Agents</RouterLink> · <RouterLink to="/app/tokens">Tokens</RouterLink></nav>
    <header><p class="eyebrow">Public routes</p><h1 id="tunnels-heading">Tunnels</h1><p>{{ activeCount }} active of {{ tunnels.length }}</p></header>
    <p v-if="loading" aria-live="polite">Loading tunnels…</p><p v-else-if="error" role="alert">{{ error }}</p>
    <div v-else-if="tunnels.length" class="table-wrap"><table><thead><tr><th>Status</th><th>Public subdomain</th><th>Local destination</th><th>Agent</th></tr></thead><tbody><tr v-for="tunnel in tunnels" :key="tunnel.id"><td><span class="badge" :class="tunnel.status">{{ tunnel.status }}</span></td><td><code>{{ tunnel.subdomain }}</code></td><td><code>{{ tunnel.localAddress }}</code></td><td><code>{{ tunnel.agentId }}</code></td></tr></tbody></table></div>
    <div v-else class="empty"><h2>No tunnels yet</h2><p>Connect an agent now. Opening public routes becomes available in the next gateway step.</p><RouterLink to="/app/agents">View agents</RouterLink></div>
  </section>
</template>
