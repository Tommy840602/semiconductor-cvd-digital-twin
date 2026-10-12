<script setup>
import {ref,onMounted,onUnmounted} from 'vue';
defineProps({icon:String});
const playing=ref(false);let timer,media;
function finish(){clearTimeout(timer);playing.value=false;}
function play(){if(playing.value||media?.matches)return;playing.value=true;timer=setTimeout(finish,850);}
function preference(){if(media.matches)finish();}
onMounted(()=>{media=matchMedia('(prefers-reduced-motion: reduce)');media.addEventListener('change',preference);});
onUnmounted(()=>{clearTimeout(timer);media?.removeEventListener('change',preference);});
</script>
<template>
<button class="sidebar-button" @pointerenter="e=>{if(e.pointerType!=='touch')play()}" @focus="e=>{if(e.target.matches(':focus-visible'))play()}" @click="play">
<svg class="sidebar-icon" :class="{playing}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false" @animationend="finish">
<template v-if="icon==='equipment'"><rect x="3" y="3" width="18" height="18" rx="3"/><path d="M3 15h18M7 21v-6M17 21v-6"/><circle class="lamp lamp-a" cx="7" cy="8" r="1"/><circle class="lamp lamp-b" cx="12" cy="8" r="1"/><circle class="lamp lamp-c" cx="17" cy="8" r="1"/></template>
<template v-else-if="icon==='history'"><path d="M11 21a9 9 0 1 1 10-10M5 6l9 9M5 12l6 6M11 4l7 7"/><circle cx="17" cy="17" r="5"/><path class="clock-hand" d="M17 14v3l2 1"/></template>
<template v-else-if="icon==='alarm'"><g class="bell"><path d="M5 17h14l-2-3V9a5 5 0 0 0-10 0v5zM10 20a2 2 0 0 0 4 0M12 2v2"/></g></template>
<template v-else-if="icon==='disposition'"><path d="M13 3a5 5 0 0 0-6 6L3 17a2 2 0 0 0 4 2l4-8a5 5 0 0 0 5-6l-4 3-2-2z"/><path class="check" pathLength="1" d="m14 17 3 3 5-6"/></template>
<template v-else-if="icon==='accounts'"><circle cx="9" cy="7" r="4"/><path d="M2 21v-3a7 7 0 0 1 11-5"/><g class="gear"><circle cx="17" cy="17" r="3"/><path d="M17 12v2m0 6v2m-5-5h2m6 0h2m-8.5-3.5 1.4 1.4m4.2 4.2 1.4 1.4m-7 0 1.4-1.4m4.2-4.2 1.4-1.4"/></g></template>
<template v-else-if="icon==='report'"><path d="M5 2h9l5 5v15H5zM14 2v6h5"/><path class="report-bars" d="M8 17v-3m4 3v-6m4 6v-4"/></template>
<template v-else-if="icon==='logout'"><path d="M10 3H4v18h6"/><g class="exit-arrow"><path d="M9 12h12m-4-4 4 4-4 4"/></g></template>
</svg><span class="sidebar-label"><slot/></span>
</button>
</template>
<style scoped>
.sidebar-button{display:flex!important;align-items:center;gap:14px!important;text-align:left}.sidebar-icon{width:24px;height:24px;flex:0 0 24px;overflow:visible;pointer-events:none}.sidebar-label{margin:0!important;min-width:0}.sidebar-button:focus-visible{outline:2px solid #8cdec8;outline-offset:3px}.playing .lamp{animation:nav-lamp .7s ease both}.playing .lamp-b{animation-name:nav-lamp-b}.playing .lamp-c{animation-name:nav-lamp-c}.playing .clock-hand{transform-origin:17px 17px;animation:nav-clock .7s ease}.playing .bell{transform-origin:12px 3px;animation:nav-bell .7s ease}.playing .check{stroke-dasharray:1;animation:nav-check .7s ease}.playing .gear{transform-origin:17px 17px;animation:nav-gear .7s ease}.playing .exit-arrow{animation:nav-exit .7s ease}
.playing .report-bars{transform-origin:12px 17px;animation:nav-report .7s ease}
@keyframes nav-report{0%,100%{transform:scaleY(1)}40%{transform:scaleY(.45)}}
@keyframes nav-lamp{0%,100%{fill:transparent}25%{fill:currentColor}}@keyframes nav-lamp-b{0%,100%{fill:transparent}50%{fill:currentColor}}@keyframes nav-lamp-c{0%,100%{fill:transparent}75%{fill:currentColor}}
@keyframes nav-clock{to{transform:rotate(360deg)}}@keyframes nav-bell{20%{transform:rotate(14deg)}40%{transform:rotate(-12deg)}60%{transform:rotate(8deg)}80%{transform:rotate(-4deg)}}@keyframes nav-check{from{stroke-dashoffset:1}to{stroke-dashoffset:0}}@keyframes nav-gear{50%{transform:rotate(60deg)}}@keyframes nav-exit{50%{transform:translateX(3px)}}
@media(prefers-reduced-motion:reduce){.sidebar-icon *{animation:none!important}}
</style>
