<script setup lang="ts">
const props = defineProps<{ name: string; size?: number }>()

const paths: Record<string, string> = {
  dashboard: 'M3 3h6v6H3V3Zm8 0h6v6h-6V3ZM3 11h6v6H3v-6Zm8 0h6v6h-6v-6Z',
  tunnels: 'M4 16 16 4M12 4h4v4M8 16H4v-4',
  agents: 'M3 4h14v9H3V4Zm4 12h6M10 13v3',
  tokens: 'M8 12a4 4 0 1 1 3.46-2H18v3h-2v2h-3v-2h-1.54A4 4 0 0 1 8 12Z',
  signout: 'M9 4H4v12h5M13 10h5m0 0-3-3m3 3-3 3',
  collapse: 'M3 4h14v12H3V4Zm5 0v12',
  chevron: 'M5 7.5 10 12.5 15 7.5',
}
</script>
<template>
  <svg :width="props.size ?? 18" :height="props.size ?? 18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
    <path :d="paths[props.name] ?? ''" />
  </svg>
</template>
