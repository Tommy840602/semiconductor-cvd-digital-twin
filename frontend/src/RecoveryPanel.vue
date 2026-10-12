<script setup>
import {t as tr,enumLabel,dateTime,number,fixed,systemMessage,evidenceReason} from "./i18n.js";
import {ref,computed,onMounted,onUnmounted} from 'vue';
import ProcessPanel from './ProcessPanel.vue';
const props=defineProps({user:Object,data:Object,api:Function,initialPm:String,initialWafer:String,initialCase:String});
const rows=ref([]),detail=ref(null),error=ref(''),busy=ref(false),editing=ref(false),pm=ref(props.initialPm||'PM1'),steps=ref([0,0,1,0]),target=ref(10),trouble=ref(''),reason=ref('');
const phaseFilter=ref('all');
const filteredRows=computed(()=>rows.value.filter(r=>phaseFilter.value==='all'||({ee:['awaiting_ee','draft','returned','withdrawn','failed','invalidated'],pe:['submitted'],execution:['approved_waiting','executing','awaiting_result','returning','cancelling'],release:['resolved']}[phaseFilter.value]||[]).includes(r.status)));
const dispositionType=ref('supplement'),cause=ref(''),correctiveAction=ref(''),verificationMethod=ref(''),verificationResult=ref(''),release=ref(null);
const incident=computed(()=>props.data.incidents?.[detail.value?.incident_id]||detail.value?.incident);
const labels=computed(()=>[tr('ui036'),tr('ui037'),tr('ui333'),tr('ui039')]);
const names={get awaiting_ee(){return tr('ui378')},get awaiting_result(){return tr('ui379')},get returning(){return tr('ui380')},get resolved(){return tr('ui381')},get exception(){return tr('ui382')},get 'result-confirm'(){return tr('ui383')},get 'result-reject'(){return tr('ui384')},get migration(){return tr('ui385')},get draft(){return tr('ui386')},get submitted(){return tr('ui280')},get returned(){return tr('ui387')},get approved_waiting(){return tr('ui388')},get cancelling(){return tr('ui389')},get executing(){return tr('ui390')},get completed(){return tr('ui391')},get failed(){return tr('ui392')},get withdrawn(){return tr('ui393')},get invalidated(){return tr('ui394')},get superseded(){return tr('ui395')},get approved(){return tr('ui396')},get published(){return tr('ui397')}, get 'cancel-requested'(){return tr('ui398')}};
const ee=computed(()=>props.user.department==='EE'&&['Engineer','Leader'].includes(props.user.role));
const pe=computed(()=>props.user.department==='PE'&&['Engineer','Leader'].includes(props.user.role));
const owner=computed(()=>detail.value?.owner_id===props.user.id);
const current=computed(()=>detail.value?.versions.at(-1));
const historyPm=computed(()=>[...(detail.value?.versions||[])].reverse().map(v=>v.prescription.pmId).find(id=>/^PM[1-4]$/.test(id||'')));
const previous=computed(()=>detail.value?.versions.at(-2));
const p=computed(()=>props.data.chambers?.find(p=>p.id===pm.value));
const baseline=computed(()=>p.value?.processTelemetry?.thicknessNm);
const rootRate=computed(()=>p.value?.processTelemetry?.rootDepositionRateNmPerSecond||0);
const total=computed(()=>steps.value.reduce((a,b)=>a+Number(b||0),0));
const valid=computed(()=>[cause.value,correctiveAction.value,verificationMethod.value,verificationResult.value].every(v=>v.trim().length>=3)&&(dispositionType.value!=='supplement'||(steps.value.every(n=>Number.isInteger(n)&&n>=0&&n<=3600)&&total.value>=1&&total.value<=3600&&target.value>0)));
const releaseResult=r=>{try{return typeof r.device_result==='string'?JSON.parse(r.device_result):r.device_result||{}}catch{return {}}};
const statusName=r=>!r.incident_id?tr('ui399')+(r.status==='completed'?tr('ui400'):names[r.status]||r.status):names[r.status]||r.status;
const num=v=>v==null?'—':fixed(v,3);
let timer,active=true;
async function load(){try{const r=await props.api('/recoveries'+(props.initialWafer?'?waferId='+encodeURIComponent(props.initialWafer):''));if(!active)return;rows.value=r;try{release.value=await props.api('/release');}catch(e){release.value={eligible:false,blocks:[e.message],items:[],releases:[]};}if(detail.value){const d=await props.api('/recoveries/'+detail.value.id);if(active)detail.value=d;}}catch(e){if(active)error.value=e.message;}}
async function select(id){try{detail.value=await props.api('/recoveries/'+id);editing.value=false;reason.value='';error.value='';}catch(e){error.value=e.message;}}
function edit(){const n=current.value.prescription;pm.value=n.pmId;dispositionType.value=incident.value?.legacy&&incident.value?.returned?'legacy_return':n.dispositionType||'supplement';if(dispositionType.value==='legacy_return')pm.value=incident.value.pmId||'';cause.value=n.cause||'';correctiveAction.value=n.correctiveAction||'';verificationMethod.value=n.verificationMethod||'';verificationResult.value=n.verificationResult||'';steps.value=[...(n.stepSeconds||[0,0,1,0])];target.value=n.targetThicknessNm||10;trouble.value=n.troubleshooting;editing.value=true;}
async function action(name,extra={}){busy.value=true;error.value='';try{detail.value=await props.api('/recoveries/'+detail.value.id+'/'+name,{method:'POST',body:JSON.stringify({revision:detail.value.revision,...extra})});editing.value=false;await load();}catch(e){error.value=e.message;await load();}finally{busy.value=false;}}
async function save(){const body={incidentId:detail.value?.incident_id,dispositionType:dispositionType.value,cause:cause.value,correctiveAction:correctiveAction.value,verificationMethod:verificationMethod.value,verificationResult:verificationResult.value,pmId:pm.value,stepSeconds:steps.value,targetThicknessNm:target.value,troubleshooting:trouble.value};if(detail.value?.owner_id)return action(detail.value.status==='draft'?'save':'revise',body);busy.value=true;error.value='';try{detail.value=await props.api('/recoveries',{method:'POST',body:JSON.stringify(body)});editing.value=false;await load();}catch(e){error.value=e.message;}finally{busy.value=false;}}
async function confirmResult(decision){busy.value=true;error.value='';try{detail.value=await props.api('/dispositions/'+detail.value.id+'/'+decision,{method:'POST',body:JSON.stringify({revision:detail.value.revision,executionId:incident.value.executionId,resultVersion:incident.value.resultVersion,reason:reason.value})});await load();}catch(e){error.value=e.message;}finally{busy.value=false;}}
async function releaseTool(){busy.value=true;error.value='';try{release.value=await props.api('/release',{method:'POST',body:JSON.stringify({generation:release.value.generation})});}catch(e){error.value=e.message;}finally{busy.value=false;}}
onMounted(async()=>{await load();if(props.initialCase)await select(props.initialCase);timer=setInterval(load,2000)});onUnmounted(()=>{active=false;clearInterval(timer)});
function waferPurpose(id){return props.data.lots?.flatMap(l=>l.wafers).find(w=>w.id===id)?.purpose==='dummy'?tr('ui401'):tr('ui402')}
</script>
<template>
<section class="panel recovery">
 <div class="panel-title"><h2>{{tr('ui403')}}</h2></div>
 <p>{{tr('ui404')}}</p>
 <p v-if="!user.department">{{tr('ui405')}}</p>
 <div v-if="release" class="approval">
 <h3>{{data.dispositionHold?tr('ui406'):tr('ui407')}} {{tr('ui408')}} {{release.generation}}</h3>
 <p v-for="block in (release.blockMessages||release.blocks)" :key="typeof block==='string'?block:block.message">{{systemMessage(block)}}</p>
 <div v-for="item in release.items" :key="item.incident.id">{{item.incident.pmId}} · {{item.incident.waferId||tr('ui409')}} · {{names[item.incident.status]||item.incident.status}} · {{item.incident.returned?tr('ui410'):tr('ui411')}}</div>
 <button v-if="user.role==='Leader'" class="primary" :disabled="busy||!release.eligible" @click="releaseTool">{{tr('ui412')}}</button>
 <details v-if="release.releases?.length"><summary>{{tr('ui413')}}{{release.releases.length}}）</summary><p v-for="r in release.releases.slice(0,3)" :key="r.id">{{dateTime(r.created_at)}} · {{r.status==='pending'?tr('ui414'):r.status==='released'?tr('ui415'):tr('ui416')}}<br/><small>{{systemMessage(releaseResult(r))}}</small></p></details>
 </div>
 <p role="alert" class="danger">{{error}}</p>
 <label class="phase-filter">{{tr('ui417')}}<select v-model="phaseFilter"><option value="all">{{tr('ui418')}}</option><option value="ee">{{tr('ui378')}}</option><option value="pe">{{tr('ui280')}}</option><option value="execution">{{tr('ui419')}}</option><option value="release">{{tr('ui420')}}</option></select></label>
 <div class="recovery-layout">
 <div class="recovery-list"><button v-for="r in filteredRows" :key="r.id" :class="{chosen:detail?.id===r.id}" @click="select(r.id)"><b>{{r.prescription.pmId||tr('ui421')}} · {{r.wafer_id||tr('ui409')}} <small v-if="r.wafer_id">{{waferPurpose(r.wafer_id)}}</small></b><span>v{{r.revision}} · {{statusName(r)}}</span><small>{{evidenceReason(r)}}</small></button><p v-if="!filteredRows.length">{{tr('ui422')}}</p></div>
 <div>
 <form v-if="editing&&ee" @submit.prevent="save" class="recovery-form">
  <h3>{{detail?tr('ui423'):tr('ui424')}}</h3>
  <label>{{tr('ui425')}}<select v-model="pm" :disabled="!!detail"><option v-if="dispositionType==='legacy_return'" value="">{{tr('ui426')}}</option><option v-for="n in [1,2,3,4]">PM{{n}}</option></select></label>
  <p v-if="dispositionType==='legacy_return'">{{tr('ui427')}} {{detail.wafer_id}} {{tr('ui428')}}</p><p v-else>{{tr('ui129')}} {{p?.waferId||'—'}} {{tr('ui429')}} <b>{{num(baseline)}} nm</b></p>
  <label>{{tr('ui430')}}<select v-model="dispositionType"><option v-if="incident?.legacy&&incident?.returned" value="legacy_return">{{tr('ui431')}}</option><template v-else-if="detail?.wafer_id"><option value="supplement">{{tr('ui432')}}</option><option value="scrap">{{tr('ui433')}}</option></template><option v-else value="equipment">{{tr('ui434')}}</option></select></label>
  <label>{{tr('ui435')}}<textarea v-model="cause" required minlength="3" maxlength="4000" /></label>
  <label>{{tr('ui436')}}<textarea v-model="correctiveAction" required minlength="3" maxlength="4000" /></label>
  <label>{{tr('ui437')}}<textarea v-model="verificationMethod" required minlength="3" maxlength="4000" /></label>
  <label>{{tr('ui438')}}<textarea v-model="verificationResult" required minlength="3" maxlength="4000" /></label>
  <template v-if="dispositionType==='supplement'">
  <label>{{tr('ui439')}}<input v-model.number="target" type="number" min="0.001" max="100000" step="0.001" required /></label>
  <div v-for="(label,i) in labels" :key="label" class="step-row"><label><input type="checkbox" :checked="steps[i]>0" @change="steps[i]=$event.target.checked?1:0" />{{label}}</label><label>{{label}}{{tr('ui440')}}<input v-model.number="steps[i]" :disabled="steps[i]===0" type="number" min="1" max="3600" step="1" /></label></div>
  <p>{{tr('ui441')}} {{total}} {{tr('ui442')}} {{num(rootRate*steps[2])}} nm</p>
  <p v-if="target<baseline" class="danger">{{tr('ui443')}}</p>
  <small>{{tr('ui444')}}</small>
  </template><button class="primary" :disabled="busy||!valid">{{tr('ui445')}}</button>
 </form>
 <template v-else-if="detail&&current">
  <h3>{{current.prescription.pmId||tr('ui421')}} · {{detail.wafer_id||tr('ui409')}} · v{{detail.revision}}</h3>
  <p v-if="!detail.incident_id">{{tr('ui446')}}</p><p><b>{{statusName(detail)}}</b> · {{evidenceReason(detail)}}</p>
  <p>{{tr('ui447')}} {{current.author?.username||tr('ui448')}}（{{current.author?.department}} {{current.author?.role}}）</p>
  <p>{{tr('ui449')}}{{current.prescription.dispositionType==='legacy_return'?tr('ui450'):current.prescription.dispositionType==='scrap'?tr('ui451'):current.prescription.dispositionType==='equipment'?tr('ui452'):tr('ui453')}}</p>
  <p>{{tr('ui454')}}{{current.prescription.cause||current.prescription.troubleshooting||tr('ui455')}}</p><p>{{tr('ui456')}}{{current.prescription.correctiveAction}}</p><p>{{tr('ui457')}}{{current.prescription.verificationMethod}}</p><p>{{tr('ui458')}}{{current.prescription.verificationResult}}</p>
  <button v-if="ee&&!detail.owner_id" @click="edit">{{tr('ui459')}}</button>
  <template v-if="current.prescription.dispositionType==='supplement'"><div class="reading"><span>{{tr('ui460')}} <b>{{num(current.prescription.baselineThicknessNm)}} nm</b></span><span>{{tr('ui249')}} <b>{{num(current.prescription.targetThicknessNm)}} nm</b></span><span>{{tr('ui461')}} <b>{{num(current.prescription.estimatedDeltaNm)}} nm</b></span></div>
  <table><thead><tr><th>{{tr('ui462')}}</th><th v-if="previous">{{tr('ui463')}}</th><th>{{tr('ui464')}}</th></tr></thead><tbody><tr v-for="(label,i) in labels"><td>{{label}}</td><td v-if="previous">{{previous.prescription.stepSeconds?.[i]||tr('ui465')}}</td><td>{{current.prescription.stepSeconds?.[i]||tr('ui465')}}</td></tr></tbody></table>
  <p>{{tr('ui466')}} {{current.prescription.totalSeconds}} {{tr('ui103')}}</p>
  <p v-if="previous">{{tr('ui467')}} {{num(previous.prescription.targetThicknessNm)}} {{tr('ui468')}}{{previous.prescription.troubleshooting}}</p>
  </template><details v-if="previous"><summary>{{tr('ui469')}}</summary><table><thead><tr><th>{{tr('ui470')}}</th><th>{{tr('ui471')}}</th><th>{{tr('ui472')}}</th></tr></thead><tbody><tr v-for="(title,key) in {cause:tr('ui473'),correctiveAction:tr('ui436'),verificationMethod:tr('ui437'),verificationResult:tr('ui438'),dispositionType:tr('ui474')}" :key="key"><th>{{title}}</th><td>{{previous.prescription[key]||tr('ui475')}}</td><td>{{current.prescription[key]||tr('ui475')}}</td></tr></tbody></table></details><div class="buttons" v-if="ee&&owner&&detail.incident_id">
   <button v-if="['draft','returned','withdrawn','failed','invalidated'].includes(detail.status)" @click="edit">{{tr('ui476')}}</button>
   <button v-if="detail.status==='draft'" :disabled="busy" @click="action('submit')">{{tr('ui477')}}</button>
   <button v-if="['draft','submitted','returned','approved_waiting'].includes(detail.status)" :disabled="busy" @click="action('withdraw')">{{tr('ui478')}}</button>
  </div>
  <div v-if="pe&&!owner&&detail.incident_id&&detail.status==='submitted'" class="approval">
   <h3>{{tr('ui479')}}</h3><p>{{tr('ui480')}}</p>
   <button class="primary" :disabled="busy" @click="action('approve')">{{tr('ui481')}}{{detail.revision}}</button>
   <label>{{tr('ui482')}}<textarea v-model="reason" maxlength="4000" /></label><button :disabled="busy||!reason.trim()" @click="action('return',{reason})">{{tr('ui483')}}</button>
  </div>
  <div v-if="incident?.status==='awaiting_result'" class="approval">
   <h3>{{tr('ui484')}} {{incident.resultVersion}}</h3><p>{{incident.returned?tr('ui485'):tr('ui486')}}</p>
   <template v-if="pe&&!owner"><label>{{tr('ui487')}}<textarea v-model="reason" maxlength="4000" /></label><div class="buttons"><button :disabled="busy" @click="confirmResult('confirm')">{{tr('ui488')}}{{incident.returned?'':tr('ui489')}}</button><button :disabled="busy||reason.trim().length<3" @click="confirmResult('reject')">{{tr('ui490')}}</button></div></template>
  </div>
  <p v-if="detail.status==='cancelling'">{{tr('ui491')}}</p>
  <div v-for="intent in detail.intents" :key="intent.execution_id" class="result">
   <p>{{tr('ui492')}} {{intent.approved_by.username}} · v{{intent.revision}}</p>
   <small>executionId：{{intent.execution_id}}</small>
   <p v-if="intent.result?.result">{{tr('ui493')}} {{num(intent.result.result.baselineThicknessNm)}} {{tr('ui494')}} {{num(intent.result.result.deltaThicknessNm)}} {{tr('ui495')}} {{num(intent.result.result.thicknessNm)}} nm · {{enumLabel(intent.result.result.quality)}}</p>
  </div>
  <ProcessPanel v-if="detail.wafer_id&&historyPm" :historical="!!incident?.legacy" :equipment-id="historyPm" :wafer-id="detail.wafer_id" :data="data" />
  <p v-if="incident?.returnedAt">{{tr('ui496')}}{{dateTime(incident.returnedAt)}}</p><details open><summary>{{tr('ui497')}}</summary><div v-for="(e,i) in detail.events" :key="i" class="timeline"><b>v{{e.revision}} · {{names[e.action]||e.action}}</b><span>{{e.actor?.username||tr('ui498')}} {{e.actor?.department}} · {{dateTime(e.time)}}</span><small>{{e.details.reason||''}}</small></div></details>
  <details><summary>{{tr('ui499')}}</summary><pre>{{detail.versions}}</pre></details>
 </template><p v-else>{{tr('ui500')}}</p>
 </div></div>
</section>
</template>
<style scoped>
.phase-filter{white-space:nowrap;display:flex;align-items:center;gap:12px;margin:18px 0}.recovery-layout{display:grid;grid-template-columns:minmax(180px,280px) minmax(0,1fr);gap:26px}.recovery-list{display:flex;flex-direction:column;gap:10px}.recovery-list button{text-align:left;display:grid;gap:8px}.recovery-form{display:grid;gap:15px}.recovery-form label,.approval label{display:grid;gap:8px}textarea{font:inherit;border:1px solid var(--border);border-radius:7px;padding:10px;width:100%;box-sizing:border-box;color:inherit}.step-row{display:flex;gap:20px;align-items:center}.step-row label:first-child{display:flex;align-items:center;min-width:100px}.step-row input[type=checkbox]{width:18px}.reading{display:flex;gap:22px;flex-wrap:wrap}.reading b{display:block;margin-top:8px}.approval,.result{padding:16px;background:var(--surface-alt);border-radius:10px;margin:20px 0}.approval label{margin:18px 0}.timeline{display:grid;gap:6px;border-left:3px solid #42a990;padding:10px 16px;margin:10px 0}small,pre{overflow-wrap:anywhere}pre{white-space:pre-wrap;font-size:12px}table{width:100%;margin-top:18px}td,th{text-align:left;padding:10px}@media(max-width:850px){.recovery-layout{grid-template-columns:1fr}.recovery-list{max-height:220px;overflow:auto}}
</style>
