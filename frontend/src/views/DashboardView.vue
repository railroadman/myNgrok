<script setup lang="ts">
import { useRouter } from 'vue-router'; import { useAuthStore } from '../stores/auth'
const auth = useAuthStore(); const router = useRouter()
async function logout() { await auth.logout(); await router.push('/login') }
</script>
<template><section><p class="eyebrow">Control plane / overview</p><h1>Route local<br><em>with confidence.</em></h1><p>One gateway for your agents, tokens and public HTTP routes.</p><div class="agent-grid"><RouterLink class="panel" to="/app/tunnels"><p class="eyebrow">01 / Traffic</p><h2>Inspect tunnels</h2><p>See active public routes and local destinations.</p></RouterLink><RouterLink class="panel" to="/app/agents"><p class="eyebrow">02 / Fleet</p><h2>Monitor agents</h2><p>Check which machines are currently connected.</p></RouterLink><RouterLink class="panel" to="/app/tokens"><p class="eyebrow">03 / Access</p><h2>Create a token</h2><p>Authorize a new Windows agent securely.</p></RouterLink></div><p><button @click="logout">Sign out</button></p></section></template>
