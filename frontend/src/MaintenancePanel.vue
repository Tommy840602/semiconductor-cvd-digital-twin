<script setup>
import {t as tr,enumLabel,uiMessage,dateTime,number,fixed,systemMessage} from "./i18n.js";
import {computed,ref,watch,onMounted,onUnmounted} from 'vue';
import {maintenanceLabels} from './maintenance.js';
const props=defineProps({data:Object,user:Object,api:Function,stale:Boolean});
const emit=defineEmits(['refresh','wafer']);
const qualification=ref(null),opinion=ref('');let qualificationTimer,qualificationSerial=0;
const pe=computed(()=>allowed.value&&props.user?.department==='PE');
async function loadQualification(){if(!held.value||!m.value?.run||document.hidden)return;const ticket=++qualificationSerial;try{const value=await props.api('/maintenance/qualification');if(ticket===qualificationSerial)qualification.value=value}catch(e){if(ticket===qualificationSerial)message.value=e.message}}
async function decide(decision){busy.value=true;try{qualification.value=await props.api('/maintenance/qualification',{method:'POST',body:JSON.stringify({requestId:crypto.randomUUID(),digest:qualification.value.digest,decision,reason:opinion.value})});emit('refresh')}catch(e){message.value=e.message;await loadQualification()}finally{busy.value=false}}
watch(()=>`${props.data.equipmentId}/${m.value?.run?.id}/${m.value?.generation}`,()=>{qualificationSerial++;qualification.value=null;loadQualification()});
onMounted(()=>{loadQualification();qualificationTimer=setInterval(loadQualification,5000)});onUnmounted(()=>{qualificationSerial++;clearInterval(qualificationTimer)});
const selectedPumps=ref([]);
const lp=ref('LP1'),busy=ref(false),message=ref(''),history=ref([]),pending=ref(''),pendingRequest=ref(null);
const m=computed(()=>props.data.maintenance),held=computed(()=>m.value&&m.value.phase!=='released');
const allowed=computed(()=>['Engineer','Leader'].includes(props.user?.role));
const supported=computed(()=>props.data.maintenanceModelVersion===1);
const canServiceAgain=computed(()=>held.value&&!m.value.loaded&&['waiting','failed'].includes(m.value.phase)&&m.value.pumpTest?.status!=='running');
watch(()=>m.value?.id,()=>{selectedPumps.value=[...(m.value?.selectedPumpIds||[])]});
const phase=computed(()=>m.value?.phase||'none');
const disabled=computed(()=>busy.value||!!pending.value||props.stale||!supported.value);
async function action(type){
 busy.value=true;message.value='';
 const parameters=m.value?{maintenanceId:m.value.id,expectedGeneration:m.value.generation,lp:lp.value}:{lp:lp.value};
 if(type==='maintenance-request'&&selectedPumps.value.length)parameters.selectedPumpIds=[...selectedPumps.value];
 try{
  if(!pending.value){pending.value=crypto.randomUUID();pendingRequest.value={commandId:pending.value,type,parameters};}
  if(pendingRequest.value)await props.api('/commands',{method:'POST',body:JSON.stringify(pendingRequest.value)});
  let result;for(let n=0;n<40;n++){result=await props.api('/commands/'+pending.value);if(['succeeded','rejected'].includes(result.status))break;await new Promise(r=>setTimeout(r,500));}
  if(result.status==='succeeded'){message.value=uiMessage('ui218');pending.value='';emit('refresh');await loadHistory()}
  else if(result.status==='rejected'){message.value=systemMessage(typeof result.result==='string'?JSON.parse(result.result):result.result)||tr('ui219');pending.value=''}
  else message.value=uiMessage('ui220');
 }catch(e){message.value=e.message;if(e.status>=400&&e.status<500){pending.value='';pendingRequest.value=null}}finally{busy.value=false}
}
async function loadHistory(){try{history.value=(await props.api('/maintenance')).history}catch(e){message.value=e.message}}
watch(()=>props.data.equipmentId,()=>{pending.value='';pendingRequest.value=null;history.value=[]});
</script>
<template>
<section class="panel maintenance-panel" :aria-label="tr('ui221')">
 <div class="maintenance-title"><h2>{{tr('ui221')}}</h2><span>{{maintenanceLabels[phase]||tr('ui222')}}</span></div>
 <p>{{tr('ui223')}}</p>
 <p v-if="!supported">{{tr('ui224')}}</p>
 <div v-if="held" class="maintenance-info"><span>{{tr('ui225')}} {{m.id}}</span><span>{{tr('ui226')}} {{m.generation}}</span><span v-if="m.selectedLP">{{tr('ui227')}} {{m.selectedLP}} · {{m.loaded?tr('ui228'):tr('ui229')}}</span><span v-if="m.parkedLotId">{{tr('ui230')}} {{m.parkedLotId}} · {{m.loaded?tr('ui231'):tr('ui232')}}</span></div>
 <ul v-if="held&&m.blockers?.length" class="maintenance-blockers"><li v-for="b in m.blockers" :key="b">{{systemMessage(b)}}</li></ul>
 <p v-if="held&&data.paused" class="notice">{{tr('ui233')}}</p>
 <fieldset v-if="allowed&&(!held||canServiceAgain)"><legend>{{tr('phmSelect')}}</legend><label v-for="n in 3" :key="n" style="display:inline-flex;margin:10px"><input type="checkbox" v-model="selectedPumps" :value="'LL'+n" :disabled="disabled" /> LL{{n}}</label></fieldset>
 <p v-if="m?.selectedPumpIds?.length">{{m.selectedPumpIds.join(' / ')}} · {{m.pumpTest ? tr(m.pumpTest.status==='passed'?'phmPassed':m.pumpTest.status==='failed'?'phmFailed':m.pumpTest.status==='invalidated'?'phmInvalidated':m.pumpTest.status==='interrupted'?'phmInterrupted':'phmRunning'):tr('phmTestRequired')}}</p>
 <div v-if="allowed" class="maintenance-actions">
 <button v-if="m?.selectedPumpIds?.length&&['waiting','failed'].includes(phase)" :disabled="disabled||data.paused||m.pumpTest?.status==='running'" @click="action('vacuum-test')">{{tr('phmTest')}}</button>
  <button v-if="!held||canServiceAgain" :disabled="disabled" @click="action('maintenance-request')">{{tr(canServiceAgain?'phmServiceAgain':'ui234')}}</button>
  <button v-if="phase==='maintenance'" :disabled="disabled" @click="action('maintenance-end')">{{tr('ui235')}}</button>
  <button v-if="held&&data.paused" :disabled="disabled" @click="action('recovery-clock')">{{tr('ui236')}}</button>
  <template v-if="held&&!m.loaded&&(['maintenance','waiting','failed'].includes(phase)||qualification?.status==='rejected')"><label>{{tr('ui237')}} <select v-model="lp"><option>LP1</option><option>LP2</option><option>LP3</option></select></label><button :disabled="disabled" @click="action('maintenance-carrier-load')">{{tr('ui238')}}</button></template>
  <button v-if="held&&m.loaded&&(['waiting','failed'].includes(phase)||qualification?.status==='rejected')" :disabled="disabled||data.paused||(m.selectedPumpIds?.length&&m.pumpTest?.status!=='passed')" @click="action('qualification-start')">{{m.run?tr('ui239'):tr('ui240')}}</button>
  <button v-if="held&&m.loaded&&phase!=='qualifying'" :disabled="disabled" @click="action('maintenance-carrier-restore')">{{tr('ui241')}}</button>
  <button v-if="phase==='ready'" class="primary" :disabled="disabled||!qualification?.confirmed" @click="action('resume')">{{tr('ui242')}}</button>
  <button v-if="pending" :disabled="busy" @click="action('')">{{tr('ui243')}}</button>
 </div>
 <section v-if="held&&m?.run" class="dummy-approval"><h3>{{tr('dummyTitle')}}</h3><p>{{tr(qualification?.status==='approved'?'dummyApproved':qualification?.status==='rejected'?'dummyRejected':qualification?.status==='awaiting-pe'?'dummyPE':'dummyWaiting')}}</p><div class="maintenance-results"><article v-for="w in qualification?.evidence?.wafers||[]" :key="w.pm"><b>{{w.pm}}</b><span>{{w.state?enumLabel(w.state):tr('ui246')}} · {{w.passed?tr('ui244'):tr('ui245')}}</span><button v-if="w.waferId" @click="emit('wafer',w.waferId)">{{tr('dummyMap')}}</button></article></div><p v-if="qualification?.confirmation?.actor">{{qualification.confirmation.actor.username}} · {{dateTime(qualification.confirmation.created_at)}} · {{qualification.confirmation.reason}}</p><div class="maintenance-actions"><button :disabled="busy" @click="loadQualification">{{tr('dummyRefresh')}}</button><template v-if="pe"><input v-model="opinion" :placeholder="tr('dummyReason')" :aria-label="tr('dummyReason')"/><button :disabled="disabled||!qualification?.canConfirm" @click="decide('confirm')">{{tr('dummyConfirm')}}</button><button :disabled="disabled||!qualification||qualification.status==='rejected'||!opinion.trim()" @click="decide('reject')">{{tr('dummyReject')}}</button></template></div></section>
 <p v-if="message" role="status">{{message}}</p>
 <div v-if="m?.run" class="maintenance-results"><article v-for="n in 4" :key="n"><b>PM{{n}}</b><strong>{{m.run.results?.['PM'+n] ? (m.run.results['PM'+n].complete&&m.run.results['PM'+n].result.quality==='pass'?tr('ui244'):tr('ui245')):tr('ui246')}}</strong><small v-if="m.run.results?.['PM'+n]">{{tr('ui247')}} {{fixed(m.run.results['PM'+n].result.thicknessNm,2)}} {{tr('ui248')}} {{fixed(m.run.results['PM'+n].result.uniformityPct,2)??'—'}}%<br>{{tr('ui249')}} {{m.run.results['PM'+n].result.targetThicknessNm}} {{tr('ui250')}} {{m.run.results['PM'+n].result.thicknessTolerancePct}}{{tr('ui251')}} {{m.run.results['PM'+n].result.uniformityLimitPct}}%</small></article></div>
 <p v-if="m?.run" class="maintenance-run">{{tr('ui252')}} {{m.run.id}} {{tr('ui253')}} {{m.run.recipeVersion}} · {{m.run.stepSeconds.join(' / ')}} {{tr('ui254')}} {{m.run.status==='invalidated'?tr('ui255'):m.run.status==='passed'?tr('ui256'):m.run.status==='failed'?tr('ui257'):tr('ui258')}}<br>{{tr('ui259')}}</p>
 <details @toggle="e=>e.target.open&&loadHistory()"><summary>{{tr('ui260')}}</summary><article v-for="cycle in history" :key="cycle.id" class="maintenance-history"><b>{{cycle.id}} · {{maintenanceLabels[cycle.phase]}}</b><div v-for="run in cycle.runs" :key="run.id">{{run.id}} · {{run.recipeVersion}} · {{enumLabel(run.status)}}<span v-for="(r,id) in run.results" :key="id"> ／ {{id}} {{r.complete?enumLabel(r.result.quality):tr('ui261')}}（{{fixed(r.result.thicknessNm,2)}} nm）</span></div></article></details>
</section>
</template>
<style scoped>
.maintenance-panel{margin:24px 0;min-width:0}.maintenance-title{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}.maintenance-title h2{font-size:18px}.maintenance-title>span{background:var(--accent-soft);color:var(--accent);border-radius:8px;padding:8px 12px}.maintenance-panel p,.maintenance-info,.maintenance-history{font-size:12px;line-height:1.8;overflow-wrap:anywhere}.maintenance-info{display:flex;gap:10px;flex-wrap:wrap;color:var(--muted)}.maintenance-actions{display:flex;gap:12px;flex-wrap:wrap;align-items:center;margin:18px 0}.maintenance-actions button{padding:12px}.maintenance-actions label{display:flex;align-items:center;gap:8px}.maintenance-actions select{width:100px}.maintenance-blockers{font-size:12px;color:#94734c;line-height:1.8}.maintenance-results{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}.maintenance-results article{border:1px solid var(--border);border-radius:10px;padding:14px;display:flex;flex-direction:column;gap:10px}.maintenance-results small{font-size:11px}.maintenance-run{color:var(--muted)}.maintenance-history{border-top:1px solid var(--border);padding:14px 0}summary{cursor:pointer}@media(max-width:700px){.maintenance-results{grid-template-columns:repeat(2,minmax(0,1fr))}.maintenance-actions{align-items:stretch}.maintenance-actions button{width:100%}}
</style>
