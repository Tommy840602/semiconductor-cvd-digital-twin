<script setup>
import {computed,ref,watch,onMounted,onUnmounted} from 'vue';import {t} from './i18n.js';import InspectionPanel from './InspectionPanel.vue';
const props=defineProps({api:Function,equipmentId:String,user:Object});const rows=ref([]),measurement=ref(''),runId=ref(''),error=ref(false);const page=ref(0);const pages=computed(()=>Math.max(1,Math.ceil(rows.value.length/10)));const visible=computed(()=>rows.value.slice(page.value*10,page.value*10+10));watch(pages,n=>page.value=Math.min(page.value,n-1));function shortId(id){const slot=id.match(/-W\d+$/)?.[0]||'';const lot=id.split('-LOT-')[1]?.split('-')[0];return lot?`${props.equipmentId} · ${lot}…${slot}`:id}let active=true,timer,epoch=0;
async function load(){const e=++epoch;try{const a=await props.api('/vision/alarms?equipmentId='+encodeURIComponent(props.equipmentId));if(active&&e===epoch){rows.value=a;error.value=false;}}catch{if(active&&e===epoch)error.value=true;}}
watch(()=>props.equipmentId,()=>{rows.value=[];page.value=0;measurement.value='';runId.value='';load();});onMounted(()=>{load();timer=setInterval(load,5000);});onUnmounted(()=>{active=false;epoch++;clearInterval(timer);});
</script>
<template>
<section class="vision-alerts">
<header class="vision-alert-heading"><h3>{{t('vision_alarms')}}</h3><span>{{rows.length}} {{t('ui152')}}</span></header>
<p v-if="error" role="alert">{{t('vision_error')}}</p><p v-else-if="!rows.length">{{t('vision_noAlarms')}}</p>
<div v-else class="vision-alert-list">
<button v-for="r in visible" :key="r.id" class="vision-alert-row" :class="{selected:runId===r.id}" :title="r.wafer_id" @click="measurement=r.measurement_id;runId=r.id">
<span class="vision-wafer"><small>{{t('ui129')}}</small><strong>{{shortId(r.wafer_id)}}</strong></span>
<span><small>{{t('vision_chamber')}}</small><b>{{r.source.pm}}</b></span><span><small>{{t('vision_revision')}}</small><b>{{r.revision}}</b></span><span class="vision-alert-action">{{t('vision_details')}} →</span>
</button>
</div>
<nav v-if="pages>1" class="vision-alert-pages"><button :disabled="page===0" @click="page--">{{t('ui134')}}</button><span>{{page+1}} / {{pages}}</span><button :disabled="page+1>=pages" @click="page++">{{t('ui137')}}</button></nav>
<p v-if="runId" class="vision-alert-full-id">{{t('ui129')}}：{{rows.find(r=>r.id===runId)?.wafer_id}}</p>
<InspectionPanel v-if="measurement" :api="api" :measurement-id="measurement" :initial-run-id="runId" :user="user"/>
</section>
</template>
<style scoped>
.vision-alerts{padding:20px 0 28px;margin-bottom:24px;border-bottom:1px solid var(--border)}.vision-alert-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:20px}.vision-alert-heading h3{margin:0}.vision-alert-heading>span{color:var(--muted);font-size:12px}.vision-alert-list{display:grid;gap:12px}.vision-alert-row{width:100%;display:grid;grid-template-columns:minmax(0,1fr) 100px 110px 130px;align-items:center;text-align:left;gap:24px;padding:18px 20px;background:var(--surface);border:1px solid var(--border);border-radius:10px}.vision-alert-row:hover,.vision-alert-row.selected{border-color:#21977f;background:var(--surface-alt)}.vision-alert-row>span{display:grid;gap:7px}.vision-alert-row small{font-size:11px;color:var(--muted)}.vision-alert-row strong{font-weight:500;overflow-wrap:anywhere}.vision-alert-row b{font-weight:500}.vision-alert-action{color:#21977f;font-size:12px}.vision-alert-pages{display:flex;justify-content:flex-end;align-items:center;gap:16px;margin-top:20px;font-size:12px;color:var(--muted)}.vision-alert-full-id{padding:16px;background:var(--surface-alt);overflow-wrap:anywhere;font-size:12px;margin-top:24px}@media(max-width:700px){.vision-alert-row{grid-template-columns:1fr 1fr;gap:16px;padding:16px}.vision-wafer{grid-column:1/-1}.vision-alert-action{grid-column:1/-1}}
</style>
