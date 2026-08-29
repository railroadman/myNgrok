<script setup lang="ts">
import { ref } from 'vue'; import { useRouter } from 'vue-router'; import { useAuthStore } from '../stores/auth'
const email = ref(''); const password = ref(''); const error = ref(''); const auth = useAuthStore(); const router = useRouter()
async function submit() { error.value = ''; try { await auth.login(email.value, password.value); await router.push('/app') } catch (e: any) { error.value = e.message ?? 'Login failed' } }
</script>
<template><section class="auth-page"><p class="eyebrow">Secure tunnel control plane</p><h1>Welcome back.</h1><p>Manage live routes to your local services.</p><form @submit.prevent="submit"><label>Email<input v-model="email" type="email" autocomplete="email" required /></label><label>Password<input v-model="password" type="password" autocomplete="current-password" minlength="10" required /></label><p v-if="error" class="alert" role="alert">{{ error }}</p><button>Enter dashboard</button></form><p><RouterLink to="/register">Create an account →</RouterLink></p></section></template>
