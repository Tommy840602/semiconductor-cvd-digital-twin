<script setup>
import {t as tr,dateTime,number,fixed} from "./i18n.js";
import {ref,watch,onMounted,onUnmounted} from 'vue';
import StatisticsPanel from './StatisticsPanel.vue';
import {maintenanceLabel} from './maintenance.js';
import {utilizationText} from './utilization.js';
const props=defineProps({api:Function,view:{type:String,default:'utilization'},fleet:{type:Array,default:()=>[]}});
const emit=defineEmits(['report','select']);
const period=ref('today'),statistics=ref(null),report=ref(null),loading=ref(false),error=ref('');
let generation=0,timer,controller;
async function refresh(){
 const ticket=++generation;controller?.abort();controller=new AbortController();
 loading.value=true;error.value='';
 if(statistics.value?.period!==period.value){report.value=null;statistics.value=null;emit('report',null);}
 try{const value=await props.api('/equipment/statistics?period='+period.value,{signal:controller.signal});if(ticket===generation){statistics.value=value;report.value=value.utilization;emit('report',value.utilization);}}
 catch(e){if(ticket===generation&&e.name!=='AbortError'){error.value=e.message;report.value=null;statistics.value=null;emit('report',null);}}
 finally{if(ticket===generation)loading.value=false;}
}
const date=value=>dateTime(value);
watch(period,refresh);
onMounted(()=>{refresh();timer=setInterval(refresh,30000)});
onUnmounted(()=>{generation++;controller?.abort();clearInterval(timer)});
</script>
<template>
<section class="util-panel" :aria-label="tr('ui528')" :aria-busy="loading">
 <div class="util-heading"><div><span class="util-eyebrow">EQUIPMENT UTILIZATION</span><h2>{{{utilization:tr('utilHeading'),timeline:tr('ui502'),distribution:tr('ui509'),production:tr('outputHeading')}[view]||tr('utilHeading')}}</h2></div><label>{{tr('ui529')}} <select v-model="period"><option value="today">{{tr('ui530')}}</option><option value="24h">{{tr('ui531')}}</option><option value="7d">{{tr('ui532')}}</option><option value="30d">{{tr('ui533')}}</option></select></label></div>
 <p v-if="loading&&!report" role="status">{{tr('ui534')}}</p>
 <p v-else-if="error" role="alert">{{error}} <button @click="refresh">{{tr('ui535')}}</button></p>
 <template v-else-if="report">
 <div v-show="view==='utilization'" class="util-summary"><article><span>{{tr('ui536')}}</span><strong>{{utilizationText(report.fleet,'utilization')}}</strong><small>{{tr('ui537')}}</small></article><article><span>{{tr('ui538')}}</span><strong>{{utilizationText(report.fleet,'pmUtilization')}}</strong><small>{{tr('ui539')}}</small></article><article><span>{{tr('ui540')}}</span><strong>{{fixed(report.fleet.coverage,1)}}<em>%</em></strong><small>{{tr('ui541')}}</small></article></div>
 <p class="util-time">{{date(report.start)}} — {{date(report.end)}} {{tr('ui542')}}<br>{{tr('ui543')}} {{date(report.computedAt)}} {{tr('ui544')}}</p>
 <div v-show="view==='utilization'" class="util-tools"><button v-for="m in report.equipment" :key="m.equipmentId" @click="emit('select',m.equipmentId)"><b>{{m.equipmentId}}</b><span v-if="maintenanceLabel(fleet.find(x=>x.equipmentId===m.equipmentId)?.maintenance)" class="maintenance-badge">{{maintenanceLabel(fleet.find(x=>x.equipmentId===m.equipmentId)?.maintenance)}}</span><span>{{tr('ui186')}} <strong>{{utilizationText(m,'utilization')}}</strong></span><span>{{tr('ui187')}} <strong>{{utilizationText(m,'pmUtilization')}}</strong></span><small>{{tr('ui545')}} {{fixed(m.coverage,1)}}{{tr('ui546')}}</small></button></div>
 <p v-if="view==='utilization'&&report.fleet.quality!=='complete'" class="util-note">{{tr('ui547')}}</p>
 <StatisticsPanel v-if="statistics" :report="statistics" :view="view" v-show="view!=='utilization'"/>
 </template>
</section>
</template>
<style scoped>
.util-panel{background:var(--surface);border:1px solid var(--border);border-radius:14px;padding:24px;margin-bottom:24px;color:var(--fg);min-width:0}.util-heading{display:flex;justify-content:space-between;gap:16px;align-items:center;flex-wrap:wrap}.util-heading h2{font-size:20px;margin:8px 0 16px}.util-eyebrow{font-size:10px;letter-spacing:2px;color:var(--muted)}.util-heading select{padding:9px;border:1px solid var(--border);border-radius:8px;background:var(--surface);color:inherit}.util-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px;margin-top:12px}.util-summary article{background:var(--surface-alt);border-radius:10px;padding:18px;display:flex;flex-direction:column;gap:10px}.util-summary strong{font-size:27px;color:var(--fg)}.util-summary em{font-size:15px;font-style:normal}.util-panel small,.util-time{color:var(--muted);font-size:11px;line-height:1.7}.util-time{margin:16px 0}.util-tools{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.util-tools button{padding:14px 10px;min-width:0;display:flex;flex-direction:column;gap:9px;text-align:left;border:1px solid var(--border);border-radius:9px;background:var(--surface);color:inherit;cursor:pointer}.util-tools button:hover{border-color:#21977f;background:var(--surface-alt)}.util-tools b{font-size:11px;overflow-wrap:anywhere}.util-tools span{display:flex;flex-wrap:wrap;gap:5px;font-size:11px;justify-content:space-between}.util-note{font-size:12px;line-height:1.7;color:#876b40;background:#faf7ef;padding:12px;border-radius:8px;margin-bottom:0}@media(max-width:1100px){.util-tools{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:600px){.util-panel{padding:16px}.util-summary{grid-template-columns:1fr;gap:8px}.util-summary article{padding:13px}.util-summary strong{font-size:22px}.util-tools{grid-template-columns:1fr 1fr}.util-heading label{font-size:12px}}
</style>
