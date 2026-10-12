<script setup>
import {t as tr,enumLabel,dateTime,taipeiInputISO,number,systemMessage} from "./i18n.js";
import {ref,computed,watch,onUnmounted} from 'vue';
const props=defineProps({equipmentId:String,data:Object,waferId:{type:String,default:''},historical:{type:Boolean,default:false}});
const metric=ref('temperature'),historyMode=ref(props.historical),from=ref(''),to=ref(''),series=ref({points:[],resolutionSeconds:1,unit:''}),events=ref([]),error=ref(''),loading=ref(false),page=ref(0),hasMore=ref(false);
let timer,serial=0,controller,eventQuery;
const labels={get baseline(){return tr('ui319')},get delta(){return tr('ui320')},get temperature(){return tr('ui321')},get power(){return tr('ui322')},get deviation(){return tr('ui323')},get precursor(){return tr('ui324')},get reactant(){return tr('ui325')},get carrier(){return tr('ui326')},get purge(){return tr('ui327')},get pressure(){return tr('ui089')},get valve(){return tr('ui328')},get thickness(){return tr('ui247')},get rate(){return tr('ui329')},get uniformity(){return tr('ui330')},get flow(){return tr('ui331')}};
const phases={get exception(){return tr('ui024')},get preheat(){return tr('ui036')},get stabilize(){return tr('ui332')},get deposition(){return tr('ui333')},get purge(){return tr('ui039')},get complete(){return tr('ui334')},get idle(){return tr('ui040')},get fault(){return tr('ui041')},get pumping(){return tr('ui335')},get venting(){return tr('ui336')},get holding(){return tr('ui337')}};
const isLock=computed(()=>props.equipmentId?.startsWith('LL'));
const choices=computed(()=>isLock.value?['pressure','valve']:Object.keys(labels));
const equipment=computed(()=>[...(props.data?.chambers||[]),...(props.data?.locks||[])].find(p=>p.id===props.equipmentId));
const process=computed(()=>!props.waferId||equipment.value?.waferId===props.waferId?equipment.value?.processTelemetry:null);
const results=computed(()=>events.value.filter(e=>e.kind==='process-result').slice().reverse());
const clock=ref(Date.now());
const clockTimer=setInterval(()=>clock.value=Date.now(),1000);
onUnmounted(()=>clearInterval(clockTimer));
const lastTime=computed(()=>props.data?.sourceTime);
const stale=computed(()=>!lastTime.value||clock.value-Date.parse(lastTime.value)>5000);
const format=v=>v==null?'—':number(v,{minimumFractionDigits:2,maximumFractionDigits:2});
function query(){const q=new URLSearchParams();if(isLock.value)q.set('llId',props.equipmentId);else q.set('pmId',props.equipmentId);if(props.waferId)q.set('waferId',props.waferId);if(historyMode.value){if(from.value)q.set('from',taipeiInputISO(from.value));if(to.value)q.set('to',taipeiInputISO(to.value));}return q;}
async function json(url,signal){const r=await fetch(url,{signal});if(!r.ok){let body;try{body=await r.json()}catch{};throw Error(systemMessage(body)|| (r.status===422?tr('ui338'):tr('ui339')));}return r.json();}
async function load(more=false,onlySeries=false){const id=++serial;controller?.abort();controller=new AbortController();loading.value=true;try{const q=query();q.set('metric',metric.value);const h=more&&eventQuery?new URLSearchParams(eventQuery):new URLSearchParams(q);h.delete('metric');h.delete('llId');if(!h.has('to'))h.set('to',new Date().toISOString());if(!h.has('from'))h.set('from',new Date(Date.parse(h.get('to'))-300000).toISOString());if(!more&&!onlySeries)eventQuery=h.toString();h.set('page',more?page.value+1:0);
 const [s,ev]=await Promise.all([json('/api/equipment/'+props.data.equipmentId+'/telemetry?'+q,controller.signal),(isLock.value||onlySeries)?Promise.resolve(null):json('/api/equipment/'+props.data.equipmentId+'/process-history?'+h,controller.signal)]);if(id!==serial)return;series.value=s;if(ev){events.value=more?[...events.value,...ev]:ev;page.value=more?page.value+1:0;hasMore.value=ev.length===100;}error.value='';
}catch(e){if(e.name!=='AbortError'&&id===serial)error.value=e.message;}finally{if(id===serial)loading.value=false;}}
watch(()=>[props.equipmentId,props.waferId,metric.value,historyMode.value],()=>{if(!choices.value.includes(metric.value))metric.value=choices.value[0];clearInterval(timer);series.value={points:[],resolutionSeconds:1,unit:''};events.value=[];load();if(!historyMode.value)timer=setInterval(()=>load(false,page.value>0),5000);},{immediate:true});
onUnmounted(()=>{clearInterval(timer);controller?.abort();serial++;});
const chart=computed(()=>{const points=series.value.points||[];const times=points.map(p=>Date.parse(p.time));const values=points.flatMap(p=>[p.actual,p.target]).filter(v=>v!=null&&Number.isFinite(Number(v))).map(Number);const min=Math.min(...values,0),max=Math.max(...values,1),start=Math.min(...times),end=Math.max(...times);const x=t=>30+(t-start)/Math.max(1000,end-start)*640,y=v=>150-(v-min)/(max-min)*130;
 function path(key){let d='',prev;for(const p of points){const t=Date.parse(p.time),v=p[key];if(v==null){prev=null;continue;}const gap=!prev||t-Date.parse(prev.time)>series.value.resolutionSeconds*1500||p.processRunId!==prev.processRunId;d+=`${gap?'M':'L'}${x(t)},${y(Number(v))} `;prev=p;}return d;}
 return{actual:path('actual'),target:path('target'),max,markers:events.value.filter(e=>e.kind==='process-phase-changed'&&Date.parse(e.sourceTime)>=start&&Date.parse(e.sourceTime)<=end).map(e=>({x:x(Date.parse(e.sourceTime)),phase:e.payload.phase,time:e.sourceTime})),start:points[0]?.time,end:points.at(-1)?.time};});
</script>
<template>
<section class="process-panel">
 <div class="process-heading"><h3>{{equipmentId}} {{tr('ui340')}}</h3><small>{{tr('ui341')}}</small></div>
 <p class="process-note" v-if="historical">{{tr('ui342')}}</p>
 <p class="notice" v-if="stale">{{tr('ui343')}}</p>
 <p class="process-status">{{phases[equipment?.step||equipment?.pumpState]||tr('ui344')}} <span v-if="process">{{tr('ui345')}} {{Math.round(process.phaseProgress*100)}}{{tr('ui346')}} {{Math.round(process.runProgress*100)}}%</span></p>
 <div class="process-readings" v-if="!isLock">
  <div><small>{{tr('ui347')}}</small><b>{{format(equipment?.temperature)}} / {{format(equipment?.targetTemperature)}}</b></div>
  <div><small>{{tr('ui348')}}</small><b>{{format(process?.heaterPowerPct)}} % / {{format(process?.temperatureDeviationC)}} °C</b></div>
  <div><small>{{tr('ui349')}}</small><b>{{format(process?.thicknessNm)}} nm / {{format(process?.depositionRateNmPerSecond)}} nm/s</b></div>
  <div><small>{{tr('ui350')}}</small><b>{{format(process?.uniformityPct)}} %</b></div>
 </div>
 <p v-if="!isLock&&!process" class="process-note">{{tr('ui351')}}</p>
 <div class="process-readings" v-if="equipment?.gasChannels"><div v-for="(g,id) in equipment.gasChannels" :key="id"><small>{{labels[id]}} {{tr('ui352')}}</small><b>{{format(g.actual)}} / {{format(g.target)}}</b></div></div>
 <p class="process-note">{{tr('ui089')}} {{format(equipment?.pressure)}} {{tr('ui353')}} {{format(equipment?.valveOpeningPct)}} {{tr('ui354')}} {{lastTime?dateTime(lastTime):'—'}}</p>
 <div class="process-tools"><label>{{tr('ui355')}} <select v-model="metric"><option v-for="id in choices" :value="id">{{labels[id]}}</option></select></label><label>{{tr('ui356')}} <select v-model="historyMode"><option :value="false">{{tr('ui357')}}</option><option :value="true">{{tr('ui358')}}</option></select></label></div>
 <div class="process-tools" v-if="historyMode"><label>{{tr('ui359')}}<input type="datetime-local" v-model="from"/></label><label>{{tr('ui360')}}<input type="datetime-local" v-model="to"/></label><button @click="load()">{{tr('ui361')}}</button></div>
 <p v-if="error" class="notice">{{error}}</p>
 <small>{{labels[metric]}} · {{series.unit}} · {{series.resolutionSeconds===60?tr('ui362'):tr('ui363')}} {{tr('ui364')}} {{loading?tr('ui365'):''}}</small>
 <svg viewBox="0 0 700 185" class="process-chart" :aria-label="labels[metric]+tr('ui366')">
  <path d="M30 20V150H675" class="axis"/><text x="30" y="14">{{format(chart.max)}} {{series.unit}}</text>
  <g v-for="m in chart.markers" :key="m.time+m.phase"><line :x1="m.x" :x2="m.x" y1="22" y2="150" class="phase-line"><title>{{phases[m.phase]}} · {{dateTime(m.time)}}</title></line></g>
  <path :d="chart.target" class="target-line"/><path :d="chart.actual" class="actual-line"/>
  <text x="30" y="175">{{chart.start?dateTime(chart.start):''}}</text><text x="670" y="175" text-anchor="end">{{chart.end?dateTime(chart.end):''}}</text>
 </svg>
 <p v-if="!series.points?.some(p=>p.actual!=null)" class="process-note">{{tr('ui367')}}</p>
 <small>{{tr('ui368')}}</small>
 <details v-if="series.resolutionSeconds===60"><summary>{{tr('ui369')}}</summary><div class="process-table"><table><thead><tr><th>{{tr('ui126')}}</th><th>{{tr('ui370')}}</th><th>{{tr('ui371')}}</th><th>{{tr('ui372')}}</th></tr></thead><tbody><tr v-for="p in series.points" :key="p.time+p.processRunId"><td>{{dateTime(p.time)}}</td><td>{{format(p.actual)}}</td><td>{{format(p.min)}} / {{format(p.max)}}</td><td>{{p.sampleCount}}</td></tr></tbody></table></div></details>
 <details v-if="!isLock"><summary>{{tr('ui373')}}</summary><div class="process-table"><p v-for="r in results" :key="r.eventId">{{r.waferId}} · {{r.payload.complete?tr('ui334'):tr('ui261')}} · {{format(r.payload.result.thicknessNm)}} nm · {{enumLabel(r.payload.result.quality)}}<br/><small>{{r.payload.processRunId}} {{tr('ui374')}}{{r.payload.result.thicknessTolerancePct}}{{tr('ui375')}} {{r.payload.result.uniformityLimitPct}}%</small></p><p v-for="e in events.filter(e=>e.kind==='process-phase-changed')" :key="e.eventId"><small>{{dateTime(e.sourceTime)}} · {{phases[e.payload.phase]}} · {{e.waferId}}</small></p><p v-if="!events.length">{{tr('ui376')}}</p><button v-if="hasMore" @click="load(true)">{{tr('ui377')}}</button></div></details>
</section>
</template>
<style scoped>
.process-panel{padding:18px 0;border-top:1px solid var(--border);margin-top:18px}.process-heading{display:flex;justify-content:space-between;align-items:center;gap:10px}.process-heading h3{margin:0;font-size:16px}.process-status{font-size:13px;color:var(--accent-strong)}.process-readings{display:grid;grid-template-columns:1fr 1fr;gap:12px}.process-readings>div{background:var(--surface-alt);border-radius:8px;padding:12px}.process-readings b{display:block;font-size:13px;margin-top:6px}.process-note{font-size:11px;color:var(--muted)}.process-tools{display:flex;flex-wrap:wrap;gap:10px;align-items:end;margin:16px 0}.process-tools label{flex:1;min-width:140px;font-size:11px}.process-tools select,.process-tools input{margin-top:5px}.process-chart{width:100%;height:auto;margin:15px 0}.process-chart path{fill:none}.axis{stroke:var(--border)}.actual-line{stroke:var(--fg);stroke-width:2}.target-line{stroke:#b19a60;stroke-width:1.5;stroke-dasharray:5 4}.phase-line{stroke:var(--border);stroke-dasharray:2 4}.process-chart text{font-size:10px;fill:var(--muted)}.process-table{max-height:250px;overflow:auto;font-size:12px;overflow-wrap:anywhere}@media(max-width:520px){.process-readings{grid-template-columns:1fr}.process-heading{align-items:start;flex-direction:column}}
</style>
