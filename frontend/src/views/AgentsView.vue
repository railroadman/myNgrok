<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

type Agent = { id: string; instanceID: string; hostname: string; os: string; arch: string; version: string; connected: boolean; lastSeenAt?: string }
const auth = useAuthStore()
const agents = ref<Agent[]>([])
const error = ref('')
const loading = ref(true)
const online = computed(() => agents.value.filter((agent) => agent.connected).length)
async function load() {
  loading.value = true; error.value = ''
  try { agents.value = await api<Agent[]>('/api/v1/agents', { headers: { Authorization: `Bearer ${auth.accessToken}` } }) }
  catch (e: any) { error.value = e.message ?? 'Unable to load agents' }
  finally { loading.value = false }
}
onMounted(load)
</script>
<template>
  <section class="agents-page" aria-labelledby="agents-heading">
    <header><p class="eyebrow">Gateway fleet</p><h1 id="agents-heading">Agents</h1><p>{{ online }} online of {{ agents.length }}</p></header>
    <p v-if="loading" aria-live="polite">Loading agents…</p><p v-else-if="error" role="alert">{{ error }}</p>
    <div v-else-if="agents.length" class="agent-grid"><article v-for="agent in agents" :key="agent.id" class="agent-card"><div><span class="status" :class="agent.connected ? 'online' : 'offline'"/> {{ agent.connected ? 'Online' : 'Offline' }}</div><h2>{{ agent.hostname }}</h2><p>{{ agent.os }}/{{ agent.arch }} · {{ agent.version }}</p><code>{{ agent.instanceID }}</code><small v-if="agent.lastSeenAt">Last seen: {{ new Date(agent.lastSeenAt).toLocaleString() }}</small></article></div>
    <div v-else class="empty"><h2>No agents connected</h2><p>Create a token, then start <code>tunnel-agent http 8080</code>.</p><RouterLink to="/app/tokens">Create an agent token</RouterLink></div>
  </section>
</template>
