<script setup>
import {t as tr,dateTime,number,fixed} from "./i18n.js";
import {computed,ref,watch} from 'vue';
import {states,duration,flagsText,mergeTimeline,distribution} from './statistics.js';
const props=defineProps({report:Object,view:{type:String,default:'timeline'}});
const selected=ref('all'),detail=ref(null);
const colors=Object.fromEntries(states.map(([id,,color])=>[id,color]));
const labels=computed(()=>Object.fromEntries(states.map(([id,label])=>[id,label])));
const tools=computed(()=>props.report.equipment.map(t=>({...t,timeline:mergeTimeline(t.timeline)})));
const stateValues=computed(()=>selected.value==='all'?props.report.fleet.states:tools.value.find(t=>t.equipmentId===selected.value)?.states||{});
const slices=computed(()=>distribution(stateValues.value));
const coverage=computed(()=>selected.value==='all'?props.report.fleet.coverage:tools.value.find(t=>t.equipmentId===selected.value)?.coverage||0);
const span=computed(()=>Math.max(1,Date.parse(props.report.end)-Date.parse(props.report.start)));
const x=time=>(Date.parse(time)-Date.parse(props.report.start))/span.value*1000;
const maxCount=computed(()=>Math.max(1,...tools.value.flatMap(t=>[t.production.completed,t.production.scrapped])));
const date=t=>dateTime(t);
const time=t=>dateTime(t);
function show(tool,b){detail.value={tool,...b};}
watch(()=>props.report,()=>detail.value=null);
</script>
<template>
<div class="stats-panel" :aria-label="tr('ui501')">
 <section v-show="view==='timeline'" class="stats-gantt">
  <div class="stats-heading"><div><span class="stats-eyebrow">EQUIPMENT TIMELINE</span><h2>{{tr('ui502')}}</h2></div><small>{{tr('ui503')}} {{duration(report.resolutionSeconds)}}</small></div>
  <div class="stats-legend"><span v-for="[id,label,color] in states" :key="id"><i :style="{background:color}"></i>{{label}}</span><span><i class="mixed"></i>{{tr('ui504')}}</span></div>
  <div class="stats-timeline-scroll" tabindex="0" :aria-label="tr('ui505')">
   <div class="stats-timeline">
    <div class="stats-axis"><span>{{tr('ui506')}}</span><div><span>{{time(report.start)}}</span><span>{{time(report.end)}}</span></div></div>
    <div v-for="tool in tools" :key="tool.equipmentId" class="stats-row"><b>{{tool.equipmentId}}</b>
     <svg viewBox="0 0 1000 22" preserveAspectRatio="none" :aria-label="tool.equipmentId+tr('ui507')">
      <defs><pattern :id="'mixed-'+tool.equipmentId" width="8" height="8" patternUnits="userSpaceOnUse"><rect width="8" height="8" fill="#e1e8e5"/><path d="M0 8L8 0" stroke="#819991" stroke-width="2"/></pattern></defs>
      <rect v-for="b in tool.timeline" :key="b.start" :x="x(b.start)" y="2" :width="Math.max(0,x(b.end)-x(b.start))" height="18" rx="2" :fill="b.state==='mixed'?`url(#mixed-${tool.equipmentId})`:colors[b.state]" tabindex="0" role="button" :aria-label="tr('timelineDescription',{id:tool.equipmentId,state:labels[b.state]||tr('ui504'),start:date(b.start),end:date(b.end)})" @mouseenter="show(tool.equipmentId,b)" @focus="show(tool.equipmentId,b)" @click="show(tool.equipmentId,b)" @keydown.enter.stop.prevent="show(tool.equipmentId,b)" @keydown.space.stop.prevent="show(tool.equipmentId,b)"><title>{{labels[b.state]||tr('ui504')}} · {{date(b.start)}} — {{date(b.end)}}</title></rect>
     </svg>
    </div>
   </div>
  </div>
  <div v-if="detail" class="stats-detail" role="status"><b>{{detail.tool}} · {{labels[detail.state]||tr('ui504')}}</b><span>{{date(detail.start)}} — {{date(detail.end)}} · {{duration((Date.parse(detail.end)-Date.parse(detail.start))/1000)}}</span><span>{{states.filter(([id])=>detail.states[id]>0).map(([id,label])=>`${label} ${duration(detail.states[id])}`).join(' ／ ')}}</span><small>{{flagsText(detail.flags)}}</small></div>
  <p v-else class="stats-hint">{{tr('ui508')}}</p>
 </section>
 <div v-show="view==='distribution'||view==='production'" class="stats-bottom">
  <section v-show="view==='distribution'" class="stats-state">
   <div class="stats-heading"><div><span class="stats-eyebrow">TIME DISTRIBUTION</span><h2>{{tr('ui509')}}</h2></div></div>
   <label class="stats-filter">{{tr('ui510')}} <select v-model="selected" :aria-label="tr('ui511')"><option value="all">{{tr('ui512')}}</option><option v-for="t in tools" :key="t.equipmentId" :value="t.equipmentId">{{t.equipmentId}}</option></select></label>
   <div class="stats-donut"><svg viewBox="0 0 200 200" role="img" :aria-label="tr('ui513')+slices.map(s=>`${s.label} ${fixed(s.percent,1)}%`).join('、')"><circle cx="100" cy="100" r="76" fill="none" stroke="#f0f4f2" stroke-width="22"/><circle v-for="s in slices.filter(s=>s.percent>0)" :key="s.key" cx="100" cy="100" r="76" fill="none" :stroke="s.color" stroke-width="22" pathLength="100" :stroke-dasharray="`${s.percent} ${100-s.percent}`" :stroke-dashoffset="-s.offset" transform="rotate(-90 100 100)"><title>{{s.label}} · {{duration(s.seconds)}} · {{fixed(s.percent,1)}}%</title></circle><text x="100" y="96" class="stats-donut-value">{{fixed(coverage,1)}}%</text><text x="100" y="119" class="stats-donut-caption">{{tr('ui514')}}</text></svg></div>
   <div class="stats-distribution"><div v-for="s in slices" :key="s.key"><span><i :style="{background:s.color}"></i>{{s.label}}</span><small>{{duration(s.seconds)}}</small><b>{{fixed(s.percent,1)}}%</b></div></div>
   <p class="stats-hint">{{tr('ui515')}}</p>
  </section>
  <section v-show="view==='production'" class="stats-production">
   <div class="stats-heading"><div><span class="stats-eyebrow">WAFER OUTPUT</span><h2>{{tr('ui516')}}</h2></div><small>{{tr('ui517')}}</small></div>
   <div class="stats-legend"><span><i style="background:#269b83"></i>{{tr('ui518')}}</span><span><i style="background:#de8a65"></i>{{tr('ui519')}}</span></div>
   <article v-for="t in tools" :key="t.equipmentId" class="stats-output-row"><div class="stats-output-title"><b>{{t.equipmentId}}</b><small :class="{'stats-partial':t.production.quality!=='complete'}">{{t.production.quality==='complete'?tr('ui520'):tr('ui521')}}</small></div>
    <div class="stats-bar"><span>{{tr('ui334')}}</span><div><i :style="{width:`${t.production.completed/maxCount*100}%`,background:'#269b83'}"></i></div><b>{{t.production.completed}} <small>{{tr('ui077')}}</small></b></div>
    <div class="stats-bar"><span>{{tr('ui022')}}</span><div><i :style="{width:`${t.production.scrapped/maxCount*100}%`,background:'#de8a65'}"></i></div><b>{{t.production.scrapped}} <small>{{tr('ui077')}}</small></b></div>
    <small class="stats-dummy">{{tr('ui522')}} {{t.production.dummyCompleted}} {{tr('ui523')}} {{t.production.dummyScrapped}} {{tr('ui524')}} {{fixed(t.production.coverage,1)}}%<span v-if="t.production.conflicts"> {{tr('ui525')}}</span></small>
   </article>
   <p class="stats-hint">{{tr('ui526')}}</p>
  </section>
 </div>
</div>
</template>
<style scoped>
.stats-panel{margin-top:18px;padding-top:0;min-width:0}.stats-heading{display:flex;justify-content:space-between;gap:14px;align-items:center;flex-wrap:wrap}.stats-heading>div{display:none}.stats-heading h2{font-size:19px;margin:8px 0 14px}.stats-eyebrow{font-size:10px;letter-spacing:2px;color:var(--muted)}.stats-heading small,.stats-hint{color:var(--muted);font-size:11px;line-height:1.8}.stats-legend{display:flex;gap:14px;flex-wrap:wrap;font-size:11px;margin:8px 0 18px;color:var(--fg)}.stats-legend span,.stats-distribution span{display:flex;align-items:center;gap:6px}i{display:inline-block;width:9px;height:9px;border-radius:2px;flex-shrink:0}.mixed{background:repeating-linear-gradient(135deg,var(--muted) 0 2px,var(--border) 2px 6px)}.stats-timeline-scroll{overflow:auto;max-width:100%;border:1px solid var(--border);border-radius:10px;padding:12px}.stats-timeline{min-width:650px}.stats-axis,.stats-row{display:grid;grid-template-columns:170px minmax(0,1fr);align-items:center;gap:12px}.stats-axis{font-size:10px;color:var(--muted);padding:5px 0 12px}.stats-axis>div{display:flex;justify-content:space-between}.stats-row{min-height:38px;border-top:1px solid var(--surface-alt)}.stats-row b{font-size:11px}.stats-row svg{width:100%;height:22px}.stats-row rect{cursor:pointer}.stats-row rect:focus{outline:2px solid var(--fg);outline-offset:1px}.stats-detail{display:flex;flex-direction:column;gap:6px;border-radius:9px;padding:14px;background:var(--surface-alt);font-size:12px;line-height:1.7;margin-top:12px;overflow-wrap:anywhere}.stats-detail small{color:var(--muted)}.stats-bottom{display:grid;grid-template-columns:minmax(0,1fr);gap:28px;margin-top:0}.stats-state,.stats-production{min-width:0;border:1px solid var(--border);border-radius:12px;padding:20px}.stats-filter{font-size:11px;color:var(--muted);display:flex;align-items:center;gap:10px;flex-wrap:wrap}.stats-filter select{background:var(--surface);border:1px solid var(--border);border-radius:7px;padding:8px;color:var(--fg);max-width:100%;font-size:12px}.stats-donut{max-width:230px;margin:12px auto}.stats-donut svg{display:block;width:100%}.stats-donut text{text-anchor:middle;fill:var(--fg)}.stats-donut-value{font-size:27px;font-weight:600}.stats-donut-caption{font-size:10px;fill:var(--muted)!important}.stats-distribution>div{display:grid;grid-template-columns:1fr auto 52px;gap:7px;padding:10px 0;border-bottom:1px solid var(--surface-alt);align-items:center;font-size:11px}.stats-distribution small{color:var(--muted);font-size:10px}.stats-distribution b{text-align:right}.stats-output-row{padding:12px 0;border-bottom:1px solid var(--surface-alt)}.stats-output-title{display:flex;justify-content:space-between;gap:8px;align-items:center;flex-wrap:wrap;margin-bottom:9px}.stats-output-title b{font-size:11px}.stats-output-title small{font-size:10px;color:var(--muted)}.stats-partial{color:#b2925c!important}.stats-bar{display:grid;grid-template-columns:65px minmax(0,1fr) 55px;gap:8px;align-items:center;margin:7px 0;font-size:10px;color:var(--muted)}.stats-bar>div{height:9px;background:var(--surface-alt);border-radius:3px}.stats-bar i{height:9px;display:block;max-width:100%}.stats-bar b{text-align:right;font-size:12px;color:var(--fg)}.stats-bar small{font-size:9px;color:var(--muted)}.stats-dummy{display:block;font-size:10px;color:var(--muted);line-height:1.7;margin-top:8px}.stats-hint{margin-top:15px}@media(max-width:1000px){.stats-bottom{grid-template-columns:1fr}}@media(max-width:600px){.stats-state,.stats-production{padding:14px}.stats-legend{gap:10px}.stats-heading>div{display:none}.stats-heading h2{font-size:17px}.stats-bottom{gap:16px}.stats-distribution>div{font-size:10px}}
</style>
