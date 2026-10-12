<script setup>
import {t as tr,dateTime,number,fixed} from "./i18n.js";
import {chamberStatus,glowStyle} from './statusGlow.js';
import { computed, ref } from 'vue';
import ProcessPanel from './ProcessPanel.vue';
import TransferRobot from './TransferRobot.vue';
const props = defineProps(['data', 'selected', 'stale', 'label']);
const emit = defineEmits(['select', 'wafer']);
const detailDialog = ref(null), detailTarget = ref(''), detailWafer = ref(''), detailOpen = ref(false);
const displayWafer = computed(() => detailTarget.value.startsWith('PM')
 ? props.data.chambers.find(p => p.id === detailTarget.value)?.waferId || '' : detailWafer.value);
const relatedLot = computed(() => (props.data.lots || []).find(l =>
 displayWafer.value ? l.wafers.some(w => w.id === displayWafer.value) :
 detailTarget.value.startsWith('LP') ? l.id === portLot(detailTarget.value)?.id :
 detailTarget.value.startsWith('LL') ? l.id === props.data.locks.find(x => x.id === detailTarget.value)?.lotId :
 l.wafers.some(w => w.id === props.data.chambers.find(x => x.id === detailTarget.value)?.waferId)
));
function showDetail(id, wafer = '') {
 detailTarget.value = id; detailWafer.value = wafer;
 if (id.startsWith('PM')) emit('select', id);
 detailOpen.value = true;
 detailDialog.value.showModal();
}
function openHistory(id) { detailDialog.value.close(); emit('wafer',id); }

// Physical positions are keyed by ID; LL2 is on the right and LL3 in the middle.
const pmLayout = {
 PM1:{center:[185,253],housing:'M95 207 H235 L275 228 V282 L235 302 H95 Z',label:[110,176]},
 PM2:{center:[282,125],housing:'M195 106 L286 48 L345 120 V173 L314 191 L240 159 Z',label:[246,28]},
 PM3:{center:[478,125],housing:'M565 106 L474 48 L415 120 V173 L446 191 L520 159 Z',label:[514,28]},
 PM4:{center:[575,253],housing:'M665 207 H525 L485 228 V282 L525 302 H665 Z',label:[650,176]}
};
const llLayout = {
 LL1:{center:[295,372],housing:'M280 309 L331 338 L327 414 H253 V339 Z',label:[182,352]},
 LL2:{center:[465,372],housing:'M480 309 L429 338 L433 414 H507 V339 Z',label:[578,352]},
 LL3:{center:[380,387],housing:'M343 340 H417 L427 422 H333 Z',label:[578,404]}
};
const portX = [295,380,465];
const endpoints = Object.fromEntries([
 ...Object.entries(pmLayout).map(([id,p])=>[id,p.center]),
 ...Object.entries(llLayout).map(([id,p])=>[id,p.center]),
 ...portX.map((x,i)=>[`LP${i+1}`,[x,581]])
]);
const robots = computed(() => [{...props.data.vac,name:'VAC',center:[380,255]}, {...props.data.atm,name:'ATM',center:[380,480]}]);
function gate(id,r) {
 if(r.name==='ATM')return id?.startsWith('LL')?[llLayout[id].center[0],425]:id?.startsWith('LP')?[endpoints[id][0],536]:null;
 return {PM1:[275,253],PM2:[328,181],PM3:[432,181],PM4:[485,253],LL1:[316,325],LL2:[444,325],LL3:[380,340]}[id];
}
function waypoints(r) {
 const a=endpoints[r.from],b=endpoints[r.to],ga=gate(r.from,r),gb=gate(r.to,r);
 return a && b && ga && gb ? [a,ga,r.center,gb,b] : [];
}
function route(r) {return waypoints(r).map((p,i)=>`${i?'L':'M'} ${p}`).join(' ');}
function portLot(id){const m=props.data.maintenance;if(m?.loaded&&m.selectedLP===id)return props.data.lots?.find(l=>l.id===m.loadedLotId);return props.data.lots?.find(l=>l.lp===id&&!l.parked&&l.purpose!=='dummy'&&l.phase!=='complete')}
function portLabel(id){if(props.data.maintenance?.loaded&&props.data.maintenance.selectedLP===id)return tr('ui197');return portLot(id)?tr('ui198'):tr('ui199')}
function wafer(id) {return (props.data.lots||[]).flatMap(l=>l.wafers).find(w=>w.id===id);}
function waferMark(id) {const w=wafer(id);return w?String(w.slot).padStart(2,'0'):'—';}
function progress(p) {const t=p.processTelemetry;return p.waferId?Math.max(0,Math.min(100,t?.runProgress!=null?t.runProgress*100:100-p.remaining/(t?.durationSeconds||props.data.processSeconds)*100)):0;}
function keyOpen(event,id,waferId='') {if(event.key==='Enter'||event.key===' '){event.preventDefault();showDetail(id,waferId);}}
function slots(lock) { return (props.data.lots || []).flatMap(l=>l.wafers).filter(w=>w.location===lock.id || [props.data.atm,props.data.vac].some(r=>r.waferId===w.id && (r.to===lock.id || r.from===lock.id))); }
const animated = r => r.waferId && route(r) && !props.stale && !props.data.paused;
</script>
<template>
 <div class="cluster-wrap">
 <svg class="cluster" viewBox="0 0 760 660" :aria-label="tr('ui200')">
  <defs><pattern id="cluster-grid" width="16" height="16" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r=".7" fill="var(--grid)"/></pattern></defs>
  <rect width="760" height="660" rx="15" fill="url(#cluster-grid)"/>
  <path class="transfer-body" d="M345 173 H415 L485 213 V290 L435 335 H325 L275 290 V213 Z"/>
  <path class="transfer-body atmosphere-body" d="M245 429 H515 V532 H245 V485 H185 V429 Z"/>
  <circle cx="215" cy="456" r="23" class="alignment-outline"/>
  <path class="alignment-outline" d="M199 438 L204 444 L197 452"/>
  <g class="gate-links"><path d="M316 187 L340 174 M420 174 L444 187 M275 236 V270 M485 236 V270 M303 315 L330 334 M430 334 L457 315 M359 340 H401 M275 425 H315 M360 427 H400 M445 425 H485 M275 536 H315 M360 536 H400 M445 536 H485"/></g>
  <text x="380" y="214" class="zone-title">{{tr('ui201')}}</text>
  <text x="380" y="448" class="zone-title">{{tr('ui202')}}</text>
  <g v-for="p in data.chambers" :key="p.id" class="equipment-target" role="button" tabindex="0" :aria-label="tr('processDetails',{id:p.id})"
    @mouseenter="emit('select',p.id)" @focus="emit('select',p.id)" @click="showDetail(p.id,p.waferId)" @keydown="keyOpen($event,p.id,p.waferId)">
   <path class="pm-housing status-glow status-outline" :data-status="chamberStatus(data,p,stale).key" :style="glowStyle(chamberStatus(data,p,stale))" :class="{selected:selected===p.id}" :d="pmLayout[p.id].housing"/>
   <text :x="pmLayout[p.id].center[0]" :y="pmLayout[p.id].center[1]-30" class="equipment-name">{{p.id}}{{wafer(p.waferId)?.purpose==='dummy'?' · Dummy':''}}</text>
   <circle :cx="pmLayout[p.id].center[0]" :cy="pmLayout[p.id].center[1]" r="23" class="wafer-disc" :class="wafer(p.waferId)?.result||'empty'"/>
   <text :x="pmLayout[p.id].center[0]" :y="pmLayout[p.id].center[1]+5" class="wafer-mark">{{p.waferId?waferMark(p.waferId):'—'}}</text>
   <text :x="pmLayout[p.id].center[0]" :y="pmLayout[p.id].center[1]+34" class="equipment-state">{{stale?tr('ui064'):p.fault?tr('ui041'):data.paused?tr('ui203'):wafer(p.waferId)?.result==='exception'?tr('ui024'):wafer(p.waferId)?.result==='awaiting_result'?tr('ui023'):p.waferId?`${p.remaining}s · ${label(p.step)}`:tr('ui040')}}</text>
   <path v-if="p.waferId" :d="`M${pmLayout[p.id].center[0]-25} ${pmLayout[p.id].center[1]+40}h${progress(p)*.5}`" class="chamber-progress"/>
  </g>
  <g v-for="lock in data.locks" :key="lock.id" class="equipment-target" role="button" tabindex="0" :aria-label="tr('lotDetails',{id:lock.id})" @click="showDetail(lock.id,slots(lock)[0]?.id||'')" @keydown="keyOpen($event,lock.id,slots(lock)[0]?.id||'')">
   <path class="pm-housing lock-housing" :d="llLayout[lock.id].housing"/>
   <text :x="llLayout[lock.id].center[0]" :y="llLayout[lock.id].center[1]-25" class="equipment-name lock-name">{{lock.id}}</text>
   <circle :cx="llLayout[lock.id].center[0]" :cy="llLayout[lock.id].center[1]" r="19" class="wafer-disc" :class="slots(lock)[0]?(slots(lock)[0].location===lock.id?slots(lock)[0].result:'away'):'empty'"/>
   <text :x="llLayout[lock.id].center[0]" :y="llLayout[lock.id].center[1]+4" class="wafer-mark">{{slots(lock)[0]?waferMark(slots(lock)[0].id):'—'}}</text>
   <text :x="llLayout[lock.id].center[0]" :y="llLayout[lock.id].center[1]+31" class="equipment-state">{{slots(lock).filter(w=>w.location===lock.id).length}} {{tr('ui204')}}</text>
  </g>
  <g class="pressure-readouts" aria-hidden="true">
   <g v-for="p in data.chambers" :key="p.id">
   <text :x="pmLayout[p.id].label[0]" :y="pmLayout[p.id].label[1]" class="pressure-name">{{p.id}} · TiN CVD</text>
   <text :x="pmLayout[p.id].label[0]" :y="pmLayout[p.id].label[1]+17" class="pressure-value">{{fixed(p.pressure,2)}} mbar</text>
   </g>
   <g v-for="lock in data.locks" :key="lock.id">
   <text :x="llLayout[lock.id].label[0]" :y="llLayout[lock.id].label[1]" class="pressure-name">{{lock.id}} · {{label(lock.phase)}}</text>
   <text :x="llLayout[lock.id].label[0]" :y="llLayout[lock.id].label[1]+17" class="pressure-value">{{fixed(lock.pressure,2)}} mbar</text>
   </g>
  </g>
  <foreignObject v-for="(id,i) in ['LP1','LP2','LP3']" :key="id" :x="portX[i]-39" y="543" width="78" height="102">
   <button class="port" :aria-label="tr('lotDetails',{id})" @click="showDetail(id)"><b>{{id}}</b><div class="carrier"><i v-for="n in 9" :key="n"></i></div><small>{{portLabel(id)}}</small></button>
  </foreignObject>
  <g v-for="r in robots" :key="r.name" class="robot-overlay">
   <path v-if="r.waferId && route(r)" :d="route(r)" class="active-route"/>
   <TransferRobot :key="`${r.waferId}-${r.from}-${r.to}-${animated(r)}`" :robot="r" :moving="!!animated(r)" :waypoints="waypoints(r)" />
   <text :x="r.center[0]" :y="r.center[1]+39" class="robot-caption">{{r.name}} · {{r.waferId?`${r.from} → ${r.to}`:tr('ui205')}}</text>
   <text :x="r.center[0]" :y="r.center[1]+53" class="robot-wafer">{{r.waferId?tr('ui206'):(r.name==='VAC'?'LL ↔ PM':'LP ↔ LL')}}</text>
  </g>
 </svg>
 <p class="detail-hint">{{tr('ui207')}}</p>
 <dialog @close="detailOpen = false" ref="detailDialog" :class="['lot-dialog', {'process-dialog':!detailTarget.startsWith('LP')}]" aria-labelledby="lot-dialog-title" @click="e => { if(e.target === detailDialog) detailDialog.close() }">
  <div class="panel-title"><h2 id="lot-dialog-title">{{detailTarget}} {{tr('ui208')}}</h2><button :aria-label="tr('ui209')" @click="detailDialog.close()">✕</button></div>
  <ProcessPanel v-if="detailOpen && detailTarget && !detailTarget.startsWith('LP')" :equipment-id="detailTarget" :data="data" />
  <template v-if="relatedLot">
   <dl><dt>Lot</dt><dd>{{relatedLot.id}}</dd><dt>{{tr('ui210')}}</dt><dd>{{relatedLot.paused ? tr('ui211') : label(relatedLot.phase)}}</dd><dt>{{tr('ui212')}}</dt><dd>{{relatedLot.lp}} ↔ {{relatedLot.ll}}</dd><dt>{{tr('ui213')}}</dt><dd>#{{relatedLot.order}}</dd><dt>{{tr('ui214')}}</dt><dd>{{relatedLot.wafers.filter(w=>['success','scrap'].includes(w.result)).length}} / 25</dd><dt>{{tr('ui215')}}</dt><dd>{{relatedLot.wafers.filter(w=>w.result==='success').length}} / {{relatedLot.wafers.filter(w=>w.result==='scrap').length}}</dd></dl>
   <div v-if="detailTarget.startsWith('LP')" class="slots"><button v-for="w in relatedLot.wafers" :key="w.id" :class="w.location===relatedLot.lp?w.result:'away'" @click="detailWafer=w.id">{{w.slot}}</button></div>
   <div v-if="displayWafer" class="wafer-detail"><small>{{tr('ui129')}}</small><p>{{displayWafer}}</p><button class="primary" @click="openHistory(displayWafer)">{{tr('ui216')}}</button></div>
  </template>
  <p v-else class="empty-state">{{tr('ui217')}}</p>
 </dialog>
 </div>
</template>
<style scoped>
.cluster-wrap{width:100%;overflow:auto;margin-top:8px}.cluster{display:block;width:100%;min-width:570px;max-width:760px;margin:auto}.transfer-body{fill:var(--surface-alt);stroke:var(--muted);stroke-width:3}.atmosphere-body{fill:var(--surface-alt)}.gate-links path{fill:none;stroke:var(--muted);stroke-width:7}.zone-title{font-size:12px;fill:var(--muted);text-anchor:middle;letter-spacing:1px}.atmosphere-body~.zone-title{letter-spacing:0}.pm-housing{fill:var(--surface-alt);stroke:var(--border);stroke-width:2}.pm-housing.selected{fill:var(--surface-alt);stroke:var(--fg);stroke-width:3}.pm-housing.fault{fill:#fff0e9;stroke:#df926c}.cluster-pm{width:100%;height:100%;border:0;background:transparent;padding:8px;text-align:left}.cluster-pm b{display:flex;justify-content:space-between;font-size:13px}.cluster-pm b span{font-size:9px;color:var(--muted);font-weight:400}.cluster-pm strong{display:block;font-size:24px;margin:3px 0;color:var(--accent-strong)}.cluster-pm small{display:block;font-size:9px}.wafer-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.cluster-pm .progress{margin-top:6px}.cluster-lock{padding:8px;height:120px}.cluster-lock .slots{display:block;margin:5px 0}.cluster-lock .slots button{width:100%;height:54px;padding:4px;font-size:18px;border-radius:8px}.cluster-lock small{font-size:9px}.robot-base{fill:var(--border);stroke:var(--muted);stroke-width:2}.robot-arm{fill:none;stroke:var(--muted);stroke-width:7;stroke-linecap:round;stroke-linejoin:round}.robot-hub{fill:var(--surface-alt);stroke:var(--fg);stroke-width:3}.active-route{fill:none;stroke:#35ad99;stroke-width:2;stroke-dasharray:5 5;opacity:.65}.moving-wafer{fill:#84dcc2;stroke:var(--accent-strong);stroke-width:2}.robot-caption{text-anchor:middle;font-size:12px;font-weight:600;fill:var(--fg)}.robot-wafer{text-anchor:middle;font-size:9px;fill:var(--muted)}.port{height:99px;width:100%;text-align:center;padding:7px}.port small{font-size:9px}.carrier{height:40px;margin:5px 0}
.lock-detail{width:100%;display:flex;justify-content:space-between;align-items:center;padding:0;border:0;background:transparent}.detail-hint{text-align:center;color:var(--muted);font-size:11px}.lot-dialog{border:1px solid var(--border);border-radius:14px;padding:24px;width:min(440px,90vw);color:var(--fg);box-shadow:0 20px 80px #123c3e33}.lot-dialog::backdrop{background:#123c3e66}.lot-dialog h2{font-size:16px}.lot-dialog dl{display:grid;grid-template-columns:110px 1fr;gap:14px;font-size:13px;margin-top:25px}.lot-dialog dt{color:var(--muted)}.lot-dialog dd{margin:0;overflow-wrap:anywhere}.wafer-detail{border-top:1px solid var(--border);padding-top:16px;overflow-wrap:anywhere}
.lot-dialog{max-height:90vh;overflow:auto}.lot-dialog.process-dialog{width:min(880px,94vw)}
.lot-dialog>.panel-title{position:sticky;top:-24px;background:var(--surface);z-index:2;padding:12px 0;margin-top:-12px}
.equipment-target{cursor:pointer;outline:none}.equipment-target:focus-visible .pm-housing{stroke:var(--fg);stroke-width:4}.equipment-name{text-anchor:middle;font-size:13px;font-weight:700;fill:var(--accent-strong)}.equipment-state{text-anchor:middle;font-size:10px;fill:var(--fg)}.pressure-name{text-anchor:middle;font-size:11px;fill:var(--muted)}.pressure-value{text-anchor:middle;font-size:13px;font-weight:600;fill:var(--accent-strong)}.wafer-disc{fill:var(--border);stroke:var(--muted);stroke-width:1.5}.wafer-disc.empty{fill:var(--surface-alt);stroke:var(--border);stroke-dasharray:3 3}.wafer-disc.success{fill:#9fd9c2;stroke:var(--fg)}.wafer-disc.awaiting_result,.wafer-disc.exception,.wafer-disc.scrap{fill:#f8dba6;stroke:#dda751}.wafer-disc.away{fill:var(--border);stroke:var(--border)}.wafer-mark{text-anchor:middle;font-size:15px;font-weight:600;fill:var(--accent-strong);pointer-events:none}.chamber-progress{stroke:var(--fg);stroke-width:3}.lock-name{font-size:11px}.alignment-outline{fill:none;stroke:var(--muted);stroke-width:2}.robot-overlay,.pressure-readouts{pointer-events:none}.robot-caption{font-size:10px}.robot-wafer{font-size:8px}
</style>
