<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Sidebar from '../components/Sidebar.vue'
import Icon from '../components/Icon.vue'

const collapsed = ref(false)
const mobileOpen = ref(false)
const isMobile = ref(false)
const sidebarCollapsed = computed(() => collapsed.value && !isMobile.value)

let mql: MediaQueryList
function syncIsMobile() { isMobile.value = mql.matches }
onMounted(() => {
  mql = window.matchMedia('(max-width: 768px)')
  syncIsMobile()
  mql.addEventListener('change', syncIsMobile)
})
onUnmounted(() => mql?.removeEventListener('change', syncIsMobile))
</script>
<template>
  <div class="app-shell">
    <Sidebar :collapsed="sidebarCollapsed" :mobile-open="mobileOpen" @toggle-collapse="collapsed = !collapsed" @close-mobile="mobileOpen = false" />
    <div v-if="mobileOpen" class="backdrop" @click="mobileOpen = false" />
    <div class="app-content">
      <div class="mobile-topbar">
        <button type="button" class="hamburger" aria-label="Open menu" @click="mobileOpen = true">
          <Icon name="collapse" :size="18" />
        </button>
        <span class="mobile-brand">myNgrok</span>
      </div>
      <main><RouterView /></main>
    </div>
  </div>
</template>
<style scoped>
.app-shell{display:flex;min-height:100vh}
.app-content{flex:1;min-width:0}
main{max-width:1080px;margin:0 auto;padding:40px 32px}
.backdrop{display:none}
.mobile-topbar{display:none}
@media(max-width:768px){
  .mobile-topbar{display:flex;align-items:center;gap:10px;padding:14px 16px;border-bottom:1px solid var(--line);background:var(--panel)}
  .hamburger{background:none;color:var(--text);padding:6px;border-radius:6px}
  .hamburger:hover{background:var(--bg)}
  .mobile-brand{font-weight:800}
  .backdrop{display:block;position:fixed;inset:0;background:rgba(16,24,40,.35);z-index:10}
  main{padding:24px 18px}
}
</style>
