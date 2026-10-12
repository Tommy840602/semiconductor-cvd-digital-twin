<script setup>
import {theme,toggleTheme} from './theme.js';
import {locale} from './i18n.js';
import {computed} from 'vue';
const labels=computed(()=>({'zh-TW':['切換日間主題','切換夜間主題'],en:['Switch to day theme','Switch to night theme'],ja:['昼間テーマに切り替え','夜間テーマに切り替え']}[locale.value]));
</script>
<template><button type="button" class="theme-toggle" :aria-label="labels[theme==='dark'?0:1]" :title="labels[theme==='dark'?0:1]" :aria-pressed="theme==='dark'" @click="toggleTheme"><svg v-if="theme==='light'" width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M20 14A8 8 0 0 1 10 4a8.5 8.5 0 1 0 10 10Z"/></svg><svg v-else width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.4 1.4m11.2 11.2L19 19M5 19l1.4-1.4M17.6 6.4 19 5"/></svg></button></template>
