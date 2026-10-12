<script setup>
import {t as tr,uiMessage,dateTime,number,systemMessage} from "./i18n.js";
import {ref,onMounted,onUnmounted} from 'vue';
const props=defineProps({api:Function});
const data=ref(null),error=ref(''),message=ref(''),busy=ref(false),base=ref(''),enabled=ref(false),dirty=ref(false),retryId=ref('');
const states={get disconnected(){return tr('ui262')},get connected(){return tr('ui263')},get needs_reauth(){return tr('ui264')},get pending(){return tr('ui265')},get expanded(){return tr('ui266')},get skipped(){return tr('ui267')},get queued(){return tr('ui268')},get retry(){return tr('ui269')},get sending(){return tr('ui270')},get accepted(){return tr('ui271')},get unknown(){return tr('ui272')},get failed(){return tr('ui273')},get cancelled(){return tr('ui274')},get expired(){return tr('ui275')}};
const types={get fault(){return tr('ui276')},get 'fault-cleared'(){return tr('ui277')},get offline(){return tr('ui278')},get online(){return tr('ui279')},get review(){return tr('ui280')},get result(){return tr('ui281')},get release(){return tr('ui282')}};
let timer,active=true;
async function load(){try{const result=await props.api('/notifications');if(!active)return;data.value=result;if(!dirty.value){base.value=result.connection.public_base_url;enabled.value=result.connection.enabled;}}catch(e){if(active)error.value=e.message;}}
async function action(path,body){busy.value=true;error.value='';message.value='';try{const result=await props.api('/notifications/'+path,{method:'POST',body:JSON.stringify(body||{})});if(path==='connect'){window.location.assign(result.url);return;}dirty.value=false;retryId.value='';message.value=uiMessage('ui283');await load();}catch(e){error.value=e.message;}finally{busy.value=false;}}
onMounted(()=>{load();timer=setInterval(load,10000)});onUnmounted(()=>{active=false;clearInterval(timer)});
</script>
<template>
<section class="mail-panel">
<h2>{{tr('ui284')}}</h2>
<p>{{tr('ui285')}}</p>
<p v-if="error" role="alert">{{error}}</p><p role="status">{{message}}</p>
<template v-if="data">
<div class="mail-status"><b>{{states[data.connection.status]||data.connection.status}}</b><span>{{data.connection.sender||tr('ui286')}}</span><span>{{data.allowed?(data.connection.enabled?tr('ui287'):tr('ui288')):tr('ui289')}}</span></div>
<p v-if="!data.configured">{{tr('ui290')}}</p>
<div class="mail-actions"><button :disabled="busy||!data.configured" @click="action('connect')">{{tr('ui291')}}</button><button :disabled="busy||data.connection.status==='disconnected'" @click="action('disconnect')">{{tr('ui292')}}</button></div>
<form class="mail-settings" @submit.prevent="action('settings',{enabled,publicBaseUrl:base})">
<label>{{tr('ui293')}}<input v-model="base" type="url" :placeholder="tr('ui294')" required @input="dirty=true" /></label>
<label class="mail-toggle"><input v-model="enabled" type="checkbox" :disabled="!data.allowed" @change="dirty=true" />{{tr('ui295')}}</label>
<button :disabled="busy">{{tr('ui296')}}</button>
</form>
<p class="mail-note">{{tr('ui297')}}{{data.redirectUri||tr('ui298')}}{{tr('ui299')}} {{data.offlineSeconds}} {{tr('ui300')}} {{data.recoverySeconds}} {{tr('ui301')}}</p>
<details open><summary>{{tr('ui302')}}</summary>
<p v-if="!data.deliveries.length">{{tr('ui303')}}</p>
<div class="mail-scroll" v-if="data.deliveries.length"><table><thead><tr><th>{{tr('ui304')}}</th><th>{{tr('ui305')}}</th><th>{{tr('ui210')}}</th><th>{{tr('ui306')}}</th><th>{{tr('ui307')}}</th></tr></thead><tbody><tr v-for="d in data.deliveries" :key="d.id"><td>{{d.equipment_id}}<br>{{types[d.type]}} {{d.subject_id}}</td><td>{{d.address}}</td><td>{{states[d.status]}}<br><small v-if="d.delivery_locale">{{tr('deliveryLanguage')}}: {{d.delivery_locale}}<span v-if="d.locale_reason==='shared-address-conflict'"> · {{tr('sharedAddressConflict')}}</span></small><br>{{tr('ui308')}} {{d.attempts}} {{tr('ui309')}}</td><td>{{dateTime(d.updated_at)}} {{tr('ui310')}}<br>{{systemMessage(d.reason)}}</td><td><button v-if="['unknown','failed'].includes(d.status)" :disabled="busy" @click="retryId=d.id">{{tr('ui311')}}</button></td></tr></tbody></table></div>
<p>{{tr('ui312')}}</p>
</details>
<div v-if="retryId" class="mail-confirm" role="alert"><p>{{tr('ui313')}}</p><button :disabled="busy" @click="action(retryId+'/retry',{acceptDuplicateRisk:true})">{{tr('ui314')}}</button><button @click="retryId=''">{{tr('ui315')}}</button></div>
<details><summary>{{tr('ui316')}}</summary><p v-if="!data.events.length">{{tr('ui317')}}</p><p v-for="e in data.events" :key="e.id">{{e.equipment_id}} · {{types[e.type]}} · {{states[e.status]}} {{systemMessage(e.reason)}}</p></details>
<details><summary>{{tr('ui318')}}</summary><p v-for="a in data.attempts" :key="a.id">{{dateTime(a.time)}} · {{a.delivery_id}} · {{states[a.result]}} · {{systemMessage(a.reason)}}</p></details>
</template>
</section>
</template>
<style scoped>
.mail-panel{margin-top:32px;padding-top:24px;border-top:1px solid var(--border)}.mail-status,.mail-actions{display:flex;flex-wrap:wrap;gap:12px;align-items:center;margin:16px 0}.mail-status{background:var(--surface-alt);padding:16px;border-radius:12px}.mail-settings{display:grid;grid-template-columns:minmax(240px,1fr) auto auto;gap:16px;align-items:end;margin:20px 0}.mail-settings label{display:grid;gap:8px}.mail-settings .mail-toggle{display:flex;align-items:center;align-self:center}.mail-toggle input{width:auto}.mail-note{font-size:13px;overflow-wrap:anywhere}.mail-panel details{padding:16px 0;border-top:1px solid var(--border)}.mail-panel summary{cursor:pointer;font-weight:600}.mail-scroll{overflow:auto}table{width:100%;border-collapse:collapse;font-size:13px}td,th{text-align:left;padding:12px;border-bottom:1px solid var(--border);overflow-wrap:anywhere}.mail-confirm{padding:16px;background:var(--warning-bg);border-radius:12px}.mail-confirm button{margin:4px}@media(max-width:800px){.mail-settings{grid-template-columns:1fr}.mail-status{align-items:start;flex-direction:column}}
</style>
