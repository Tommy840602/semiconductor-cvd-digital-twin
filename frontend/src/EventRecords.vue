<script setup>
import {computed,ref,watch} from 'vue';
import {locale,enumLabel,dateTime,t as tr} from './i18n.js';
const props=defineProps({rows:{type:Array,default:()=>[]},history:Boolean});
const page=ref(0),filter=ref('');
const text=computed(()=>({ 'zh-TW':{all:'全部事件',details:'詳細資料',empty:'沒有符合的紀錄',gap:'採樣延遲',buffer:'傳輸暫存已滿',miss:'缺少採樣',seconds:'秒',unknown:'事件已記錄',actor:'操作人員',system:'自動事件／未記錄人員',success:'指令執行成功',failure:'指令未完成',records:'筆',raw:'原始資料'},en:{all:'All events',details:'Details',empty:'No matching records',gap:'Sampling delayed',buffer:'Delivery buffer full',miss:'Missing samples',seconds:'seconds',unknown:'Event recorded',actor:'Operator',system:'Automatic event / actor not recorded',success:'Command succeeded',failure:'Command not completed',records:'records',raw:'Raw data'},ja:{all:'すべてのイベント',details:'詳細',empty:'該当する記録なし',gap:'サンプリング遅延',buffer:'送信バッファが満杯',miss:'欠落サンプル',seconds:'秒',unknown:'イベント記録済み',actor:'操作者',system:'自動イベント／操作者記録なし',success:'指令成功',failure:'指令未完了',records:'件',raw:'元データ'}}[locale.value]));
const kinds=computed(()=>[...new Set(props.rows.map(r=>r.kind))]);
const filtered=computed(()=>props.rows.filter(r=>!filter.value||r.kind===filter.value));
const pages=computed(()=>Math.max(1,Math.ceil(filtered.value.length/20)));
const visible=computed(()=>filtered.value.slice(page.value*20,page.value*20+20));
watch(filter,()=>page.value=0);watch(pages,n=>page.value=Math.min(page.value,n-1));
function payload(r){if(typeof r.payload==='string'){try{return JSON.parse(r.payload)}catch{return {}}}return r.payload||{}}
function summary(r){const p=payload(r),t=text.value;
 if(r.kind==='data-gap')return p.reason==='delivery buffer full'?t.buffer:`${t.gap}${Number.isFinite(p.missedSeconds)?' · '+t.miss+' '+p.missedSeconds+' '+t.seconds:''}`;
 if(p.from&&p.to)return `${p.robot||''} ${p.from} → ${p.to}`;
 if(r.kind==='command-result')return (p.status==='succeeded'?t.success:t.failure)+(p.reason?' · '+p.reason:'');
 if(r.kind==='lot-start')return `${p.lp||'—'} → ${p.ll||'—'}`;
 return [p.pmId||p.chamberId||p.targetId,p.phase?enumLabel(p.phase):'',p.reason||p.message||''].filter(Boolean).join(' · ')||enumLabel(r.kind);
}
</script>
<template>
<div class="event-records">
<div class="record-toolbar"><select v-model="filter" :aria-label="text.all"><option value="">{{text.all}}</option><option v-for="kind in kinds" :key="kind" :value="kind">{{enumLabel(kind)}}</option></select><span>{{filtered.length}} {{text.records}}</span></div>
<div class="record-table-wrap"><table class="record-table"><thead><tr><th>{{tr('ui126')}}</th><th>{{tr('ui127')}}</th><th v-if="history">{{tr('ui128')}}</th><th v-if="history">{{tr('ui129')}}</th><th>{{tr('ui130')}}</th></tr></thead><tbody>
<tr v-for="(row,index) in visible" :key="row.id||`${row.sourceTime}-${index}`"><td class="record-time">{{dateTime(row.sourceTime)}}</td><td><strong>{{enumLabel(row.kind)}}</strong></td><td v-if="history" class="record-id">{{row.lotId||'—'}}</td><td v-if="history" class="record-id">{{row.waferId||'—'}}</td><td class="record-content">
<div class="record-summary">{{summary(row)}}</div>
<div class="raw-block"><span class="record-label">{{text.raw}}</span><code>{{JSON.stringify(row.payload)}}</code></div>
<div v-if="history" class="actor-block"><span class="record-label">{{text.actor}}</span><span>{{row.actor?.username||text.system}}</span></div>
</td></tr></tbody></table></div><p v-if="!visible.length">{{text.empty}}</p>
<nav v-if="pages>1" class="record-toolbar"><button :disabled="page===0" @click="page--">{{tr('ui134')}}</button><span>{{page+1}} / {{pages}}</span><button :disabled="page+1>=pages" @click="page++">{{tr('ui137')}}</button></nav>
</div>
</template>
<style scoped>
.record-toolbar{display:flex;align-items:center;gap:12px;justify-content:space-between;margin:16px 0;color:var(--muted)}.record-toolbar select{max-width:240px}.record-table-wrap{overflow-x:auto}.record-table{width:100%;border-collapse:collapse;font-size:13px}.record-table th{background:var(--surface-alt);text-align:left;padding:14px 12px;color:var(--muted)}.record-table td{padding:18px 12px;vertical-align:top;border-bottom:1px solid var(--border)}.record-table th+th,.record-table td+td{border-left:1px solid var(--surface-alt)}.record-time{min-width:170px}.record-id{max-width:180px;overflow-wrap:anywhere}.record-content{min-width:280px}.record-summary{color:var(--fg);margin-bottom:10px}.raw-block{padding:10px 12px;background:var(--surface-alt);border-left:3px solid var(--border);border-radius:4px;display:grid;gap:6px}.raw-block code{font-size:12px;font-family:ui-monospace,monospace;overflow-wrap:anywhere;white-space:pre-wrap;color:var(--fg)}.record-label{font-size:11px;font-weight:600;color:var(--muted)}.actor-block{border-top:1px dashed var(--border);margin-top:10px;padding-top:8px;display:flex;gap:10px;color:var(--muted);font-size:12px}
</style>
