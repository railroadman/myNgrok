<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { useAuthStore } from '../stores/auth'

type Token = { id: string; name: string; prefix: string; createdAt: string; revokedAt?: string }
const auth = useAuthStore(); const tokens = ref<Token[]>([]); const name = ref(''); const revealed = ref(''); const error = ref('')
function headers() { return { Authorization: `Bearer ${auth.accessToken}` } }
async function load() { try { tokens.value = await api<Token[]>('/api/v1/agent-tokens', { headers: headers() }) } catch (e: any) { error.value = e.message ?? 'Unable to load tokens' } }
async function create() { error.value = ''; revealed.value = ''; try { const result = await api<Token & { token: string }>('/api/v1/agent-tokens', { method: 'POST', headers: headers(), body: JSON.stringify({ name: name.value }) }); revealed.value = result.token; name.value = ''; await load() } catch (e: any) { error.value = e.message ?? 'Unable to create token' } }
async function revoke(id: string) { if (!confirm('Revoke this token?')) return; await api(`/api/v1/agent-tokens/${id}`, { method: 'DELETE', headers: headers() }); await load() }
onMounted(load)
</script>
<template><section><p class="eyebrow">Access / agent credentials</p><h1>Agent tokens</h1><p>Create a scoped credential for every machine. A token is shown once only.</p><div class="panel"><form @submit.prevent="create"><label>Token label<input v-model="name" required maxlength="128" placeholder="Windows home PC" /></label><button>Create token</button></form></div><p v-if="revealed" class="alert" role="status"><strong>Copy now — it cannot be shown again.</strong><br><code>{{ revealed }}</code></p><p v-if="error" class="alert" role="alert">{{ error }}</p><div v-if="tokens.length" class="table-wrap"><table><thead><tr><th>Credential</th><th>Label</th><th>Status</th><th></th></tr></thead><tbody><tr v-for="token in tokens" :key="token.id"><td><code>{{ token.prefix }}…</code></td><td>{{ token.name }}</td><td><span class="badge" :class="token.revokedAt ? 'closed' : 'open'">{{ token.revokedAt ? 'Revoked' : 'Active' }}</span></td><td><button v-if="!token.revokedAt" @click="revoke(token.id)">Revoke</button></td></tr></tbody></table></div><div v-else class="empty"><h2>No agent tokens yet</h2><p>Create one to connect your Windows client.</p></div></section></template>
