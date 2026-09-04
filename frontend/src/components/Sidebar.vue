<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
import { navGroups } from '../nav'
import Icon from './Icon.vue'

defineProps<{ collapsed: boolean; mobileOpen: boolean }>()
const emit = defineEmits<{ (e: 'toggle-collapse'): void; (e: 'close-mobile'): void }>()
const auth = useAuthStore()
</script>
<template>
  <aside class="sidebar" :class="{ collapsed, 'mobile-open': mobileOpen }">
    <div class="org-switch">
      <span class="org-icon">↗</span>
      <span v-if="!collapsed" class="org-name">myNgrok</span>
      <Icon v-if="!collapsed" name="chevron" :size="14" class="org-chevron" />
      <button class="collapse-btn" type="button" @click="emit('toggle-collapse')" aria-label="Toggle sidebar">
        <Icon name="collapse" :size="16" />
      </button>
    </div>
    <nav>
      <div v-for="group in navGroups" :key="group.label" class="sidebar-group">
        <p v-if="!collapsed" class="sidebar-group-label">{{ group.label }}</p>
        <RouterLink
          v-for="item in group.items"
          :key="item.to"
          :to="item.to"
          class="sidebar-item"
          :class="{ 'exact-root': item.to === '/app' }"
          active-class="active"
          :exact-active-class="item.to === '/app' ? 'active' : ''"
          @click="emit('close-mobile')"
        >
          <Icon :name="item.icon" :size="17" />
          <span v-if="!collapsed">{{ item.label }}</span>
        </RouterLink>
      </div>
    </nav>
    <button class="signout-btn" type="button" @click="auth.logout()">
      <Icon name="signout" :size="17" />
      <span v-if="!collapsed">Sign out</span>
    </button>
  </aside>
</template>
<style scoped>
.sidebar{display:flex;flex-direction:column;width:232px;flex-shrink:0;background:var(--panel);border-right:1px solid var(--line);height:100vh;position:sticky;top:0;transition:width .15s ease}
.sidebar.collapsed{width:64px}
.org-switch{display:flex;align-items:center;gap:8px;padding:16px 14px;border-bottom:1px solid var(--line)}
.org-icon{display:inline-grid;place-items:center;width:26px;height:26px;border-radius:8px;background:var(--accent);color:#fff;font-weight:800;font-size:.85rem;flex-shrink:0}
.org-name{font-weight:700;font-size:.9rem;margin-right:auto}
.org-chevron{color:var(--muted);margin-right:6px}
.collapse-btn{margin-left:auto;background:none;color:var(--muted);padding:4px;border-radius:6px}
.collapse-btn:hover{background:var(--bg)}
.sidebar.collapsed .org-switch{justify-content:center}
.sidebar.collapsed .collapse-btn{margin-left:0}
nav{flex:1;overflow-y:auto;padding:12px 10px}
.sidebar-group{margin-bottom:16px}
.sidebar-group-label{font:11px 'DM Mono',monospace;color:var(--muted);text-transform:uppercase;letter-spacing:.06em;padding:0 8px;margin:0 0 6px}
.sidebar-item{display:flex;align-items:center;gap:10px;padding:8px 10px;border-radius:8px;color:var(--text);font-size:.86rem;font-weight:600;margin-bottom:2px}
.sidebar-item:hover{background:var(--bg)}
.sidebar-item.active{background:var(--accent-bg);color:var(--accent)}
.sidebar.collapsed .sidebar-item{justify-content:center}
.signout-btn{display:flex;align-items:center;gap:10px;margin:10px;padding:9px 10px;background:none;color:var(--muted);font-weight:600;font-size:.86rem;border-radius:8px;border-top:1px solid var(--line);border-radius:0;border-top:1px solid var(--line)}
.signout-btn:hover{background:var(--bg);color:var(--text)}
.sidebar.collapsed .signout-btn{justify-content:center}
@media(max-width:768px){
  .sidebar{position:fixed;left:0;top:0;z-index:20;width:232px;transform:translateX(-100%);box-shadow:0 0 0 rgba(0,0,0,0)}
  .sidebar.mobile-open{transform:translateX(0);box-shadow:2px 0 16px rgba(0,0,0,.12)}
}
</style>
