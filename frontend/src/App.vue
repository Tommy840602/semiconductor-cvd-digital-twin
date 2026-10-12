<script setup>
import ReportPanel from './ReportPanel.vue';
import VisionAlerts from './VisionAlerts.vue';
import EventRecords from "./EventRecords.vue";
import MetrologyPanel from "./MetrologyPanel.vue";
import LanguageSelect from "./LanguageSelect.vue";
import {t as tr,enumLabel,localizedError,uiMessage,locale,setLocale,dateTime,taipeiInputISO,number,fixed,systemMessage,systemText,roleLabel,localizeValidation} from "./i18n.js";
import NotificationPanel from "./NotificationPanel.vue";
import SidebarButton from "./SidebarButton.vue";
import {appearance,glowStyle,isStale} from "./statusGlow.js";
import { ref, computed, watch, onMounted, onUnmounted, defineAsyncComponent } from "vue";
import "./style.css";
import "./theme.css";
import ThemeToggle from "./ThemeToggle.vue";
import VacuumPanel from "./VacuumPanel.vue";
import MaintenancePanel from "./MaintenancePanel.vue";
import RecoveryPanel from "./RecoveryPanel.vue";
import ProcessPanel from "./ProcessPanel.vue";
import EquipmentTopology from "./EquipmentTopology.vue";
const CleanroomOverview = defineAsyncComponent(() => import("./CleanroomOverview.vue"));
import {createState} from "./cleanroom/navigation.js";
function invalidInput(event){localizeValidation(event.target);}
function clearValidation(event){if(event.target.dataset?.i18nValidation){event.target.setCustomValidity('');delete event.target.dataset.i18nValidation;}}
watch(locale,()=>document.querySelectorAll('[data-i18n-validation]').forEach(localizeValidation));
onMounted(()=>{document.addEventListener('invalid',invalidInput,true);document.addEventListener('input',clearValidation,true);});
onUnmounted(()=>{document.removeEventListener('invalid',invalidInput,true);document.removeEventListener('input',clearValidation,true);});
const cleanroomState=ref(createState());
const phmOpen=ref(false);
const equipmentId=ref('TEL-CVDTIN-08A-01'), fleet=ref([]), showFleet=ref(true), recipeVersions=ref(null), copySource=ref('TEL-CVDTIN-08A-02'), copyVersions=ref([]), copyVersion=ref('');
const user = ref(null), checking = ref(true), loginName = ref(''), loginPassword = ref(''), authError = ref('');
const recoveryCase=ref('');
const loginLocale=ref(null);
const linkParams=new URLSearchParams(window.location.search);
let pendingOAuth=window.location.pathname==='/oauth/outlook'?{code:linkParams.get('code'),state:linkParams.get('state')}:null;
if(pendingOAuth)window.history.replaceState({},'', '/');
let pendingLink={equipment:linkParams.get('equipment'),view:linkParams.get('view'),caseId:linkParams.get('case')};
async function followNotification(){
 if(pendingOAuth){const callback=pendingOAuth;pendingOAuth=null;showFleet.value=false;tab.value='accounts';await loadAccounts();try{await api('/notifications/callback',{method:'POST',body:JSON.stringify(callback)});accountMessage.value=uiMessage('ui000');}catch(e){accountMessage.value=e.message;}return;}
 if(/^TEL-CVDTIN-08A-(?:0[1-9]|10)$/.test(pendingLink.equipment||'')){selectEquipment(pendingLink.equipment);if(pendingLink.view==='disposition'){tab.value='disposition';recoveryCase.value=/^[A-Za-z0-9_-]{1,128}$/.test(pendingLink.caseId||'')?pendingLink.caseId:'';}pendingLink={};}
}
const recoveryWafer=ref(''),accountRows=ref([]),newDepartment=ref('');
const newName = ref(''), newPassword = ref(''), newRole = ref('Operator'), accountMessage = ref(''), auditRows=ref([]);
const canEngineer = computed(() => ['Engineer','Leader'].includes(user.value?.role));
let csrf = '', authEpoch=0, lastActivity=0;
function clearAuth() {
  authEpoch++; user.value=null; clearTimeout(reconnect); ws?.close(); connected.value=false;
  data.value={lots:[],locks:[],chambers:[],atm:{},vac:{}};
  history.value=[];alarms.value=[];commands.value=[];trend.value=[];historicalTrend.value=[];auditRows.value=[];
  tab.value='overview';recipeDirty.value=false;recipeCommand.value='';recipeFeedback.value='';newPassword.value='';newName.value='';accountMessage.value='';
}
async function login() {
  busy.value=true;authError.value='';
  try {
    csrf=(await api('/auth/csrf')).token;
    const result=await api('/auth/login',{method:'POST',body:JSON.stringify({username:loginName.value,password:loginPassword.value})});
    csrf=result.csrf;
    if(loginLocale.value){await api('/auth/preferences',{method:'PATCH',body:JSON.stringify({locale:loginLocale.value})});setLocale(loginLocale.value);loginLocale.value=null;}
    else setLocale((await api('/auth/preferences')).locale);
    user.value=result.user;loginPassword.value='';error.value='';authEpoch++;connect();await refresh();await followNotification();
  } catch(e){authError.value=e.message;} finally {busy.value=false;}
}
async function logout() {
  try {await api('/auth/logout',{method:'POST'});clearAuth();csrf='';}
  catch(e){error.value=e.message;}
}
async function createUser() {
  busy.value=true;accountMessage.value='';
  try { const created=await api('/users',{method:'POST',body:JSON.stringify({username:newName.value,password:newPassword.value,role:newRole.value,department:newDepartment.value||null})});
    accountMessage.value=uiMessage('accountCreated',{name:created.username,role:created.role});newName.value='';newPassword.value='';await loadAccounts();
  }catch(e){accountMessage.value=e.message;}finally{busy.value=false;}
}
async function loadAccounts(){try{accountRows.value=await api('/users')}catch(e){accountMessage.value=e.message;}}
async function saveEmail(account){try{await api('/users/'+account.id+'/email',{method:'POST',body:JSON.stringify({email:account.email,enabled:account.email_enabled})});accountMessage.value=uiMessage('ui001');await loadAccounts();}catch(e){accountMessage.value=e.message;}}
async function assignDepartment(account,department){try{await api('/users/'+account.id+'/department',{method:'POST',body:JSON.stringify({department:department||null})});accountMessage.value=uiMessage('ui002');await loadAccounts();}catch(e){accountMessage.value=e.message;}}
function activity(event) {
  if(!event.isTrusted || !user.value || Date.now()-lastActivity<60000)return;
  lastActivity=Date.now();api('/auth/activity',{method:'POST'}).catch(()=>{});
}
const data = ref({ lots: [], locks: [], chambers: [], atm: {}, vac: {} }),
  selected = ref("PM1"),
  tab = ref("overview"),
  history = ref([]),
  alarms = ref([]),
  commands = ref([]),
  trend = ref([]),
  error = ref(""),
  connected = ref(false),
  now = ref(Date.now()),
  lotId = ref(""),
  waferId = ref(""),
  lp = ref("LP1"),
  ll = ref("LL1"),
  recipeDraft = ref({preheat:6,stabilize:6,deposition:13,purge:5}),
  recipeDirty = ref(false), recipeCommand = ref(""), recipeFeedback = ref(""),
  busy = ref(false),
  page = ref(0),
  rangeFrom = ref(""),
  rangeTo = ref(""),
  historicalTrend = ref([]);
let ws,
  timer,
  reconnect,
  closed = false;
const stale = computed(() => isStale(data.value,now.value));
const lots = computed(() => (data.value.lots || []).filter(l=>l.purpose!=='dummy')),
  wafers = computed(() => lots.value.flatMap((x) => x.wafers)),
  success = computed(
    () => wafers.value.filter((x) => x.result === "success").length,
  ),
  scrap = computed(
    () => wafers.value.filter((x) => x.result === "scrap").length,
  ),
  active = computed(() =>
    lots.value.find(
      (x) => !["complete", "unloading", "venting"].includes(x.phase),
    ),
  ),
  pm = computed(() =>
    data.value.chambers?.find((x) => x.id === selected.value),
  );
const recipeKeys=['preheat','stabilize','deposition','purge'];
const recipeLabels=computed(()=>[tr('ui003'),tr('ui004'),tr('ui005'),tr('ui006')]);
const recipeTotal=computed(()=>recipeKeys.reduce((sum,k)=>sum+Number(recipeDraft.value[k]||0),0));
const recipeValid=computed(()=>recipeKeys.every(k=>Number.isInteger(recipeDraft.value[k])&&recipeDraft.value[k]>=1&&recipeDraft.value[k]<=3600)&&recipeTotal.value<=3600);
const recipeLocked=computed(()=>data.value.maintenance?.phase==='qualifying');
function currentRecipe(){
 let steps=data.value.recipeSteps;
 if(!steps){
  const remaining=(data.value.processSeconds||30)-4, weights=[.2,.2,.45,.15];
  steps=weights.map(w=>1+Math.floor(remaining*w));
  const order=[0,1,2,3].sort((a,b)=>(remaining*weights[b]%1)-(remaining*weights[a]%1));
  for(let n=(data.value.processSeconds||30)-steps.reduce((a,b)=>a+b,0),i=0;i<n;i++)steps[order[i]]++;
 }
 return Object.fromEntries(recipeKeys.map((k,i)=>[k,steps[i]]));
}
function resetRecipe(){recipeDraft.value=currentRecipe();recipeDirty.value=false;recipeFeedback.value='';}
watch(()=>JSON.stringify([data.value.recipeSteps,data.value.processSeconds]),()=>{if(!recipeDirty.value)resetRecipe();});
watch(commands,()=>{
 const result=commands.value.find(c=>c.id===recipeCommand.value);
 if(result?.status==='succeeded'){recipeFeedback.value=uiMessage('ui007');recipeDirty.value=false;recipeCommand.value='';}
 if(result?.status==='rejected'){recipeFeedback.value=systemMessage(result.result)||tr('ui008');recipeCommand.value='';}
});
async function loadRecipes(){recipeVersions.value=await api('/recipes');}
async function loadCopyVersions(){const r=await api('/equipment/'+copySource.value+'/recipes');copyVersions.value=r.versions;copyVersion.value=r.currentVersion;}
async function applyRecipe(copy=false){
 if(!copy&&!recipeValid.value)return;
 try {if(!recipeVersions.value)await loadRecipes();
 const body={expectedVersion:recipeVersions.value.currentVersion,...(copy?{sourceEquipment:copySource.value,sourceVersion:copyVersion.value}:{settings:{stepSeconds:{...recipeDraft.value}}})};
 const result=await api('/recipes',{method:'POST',body:JSON.stringify(body)});recipeVersions.value=result.recipe;recipeCommand.value=result.commandId;recipeDirty.value=false;recipeFeedback.value=uiMessage('ui009');
 }catch(e){recipeFeedback.value=e.message;await loadRecipes();}
}
function selectEquipment(id){
 authEpoch++;equipmentId.value=id;showFleet.value=false;clearTimeout(reconnect);ws?.close();
 data.value={lots:[],locks:[],chambers:[],atm:{},vac:{}};history.value=[];alarms.value=[];commands.value=[];trend.value=[];historicalTrend.value=[];waferId.value='';lotId.value='';recoveryWafer.value='';recoveryCase.value='';selected.value='PM1';recipeDirty.value=false;recipeCommand.value='';recipeFeedback.value='';recipeVersions.value=null;copySource.value=id==='TEL-CVDTIN-08A-01'?'TEL-CVDTIN-08A-02':'TEL-CVDTIN-08A-01';copyVersions.value=[];copyVersion.value='';tab.value='overview';error.value='';
 connect();refresh();loadRecipes().catch(e=>error.value=e.message);
}

const names = {
  get empty(){return tr('ui010')},
  get atmosphere(){return tr('ui011')},
  get pumping(){return tr('ui012')},
  get vacuum(){return tr('ui013')},
  get venting(){return tr('ui014')},
  get loading(){return tr('ui015')},
  get ready(){return tr('ui016')},
  get processing(){return tr('ui017')},
  get unloading(){return tr('ui018')},
  get complete(){return tr('ui019')},
  get pending(){return tr('ui020')},
  get success(){return tr('ui021')},
  get scrap(){return tr('ui022')}, get awaiting_result(){return tr('ui023')},
  get exception(){return tr('ui024')},
  get received(){return tr('ui025')},
  get accepted(){return tr('ui026')},
  get succeeded(){return tr('ui027')},
  get rejected(){return tr('ui028')}, get start(){return tr('ui029')},get pause(){return tr('ui030')},get stop(){return tr('ui031')},get resume(){return tr('ui032')},get clear(){return tr('ui033')},get rework(){return tr('ui034')},get configure(){return tr('ui035')},
  get preheat(){return tr('ui036')},
  get stabilize(){return tr('ui037')},
  get deposition(){return tr('ui038')},
  get purge(){return tr('ui039')},
  get idle(){return tr('ui040')},
  get fault(){return tr('ui041')},
};
function label(s) {
  return names[s] || s || "—";
}
async function api(path, options = {}) {
  const headers={...options.headers};
  if(options.method && options.method!=='GET') {headers['X-CSRF-Token']=csrf;headers['Content-Type']='application/json';}
  const epoch=authEpoch;
  const global=/^\/(auth|users|audit|equipment|notifications|metrology|vision|reports)(\/|$|\?)/.test(path);
  const url='/api'+(global?'':'/equipment/'+equipmentId.value)+path;
  const r = await fetch(url,{...options,headers,credentials:'same-origin'});
  if (!r.ok) {
    if(r.status===401 && path!=='/auth/login') {clearAuth();authError.value=uiMessage('ui042');}
    let message=uiMessage('ui043');
    try {message=systemText(await r.json())||message;}catch{}
    const error=Error();error.message=message;error.status=r.status;throw error;
  }
  const result=await r.json();if(epoch!==authEpoch)throw localizedError('ui044');return result;
}
function accept(s) {
  if (s.equipmentId===equipmentId.value && s.version >= (data.value.version || 0)) data.value = s;
}
let refreshTask=null,lastTrendLoad=0;
function refresh({poll=false}={}) {
  if(!user.value||document.hidden)return Promise.resolve();
  const epoch=authEpoch;
  if(refreshTask?.epoch===epoch)return refreshTask.promise;
  const task={epoch,promise:null};
  task.promise=(async()=>{
    try {
      const jobs=[];
      if(showFleet.value){jobs.push(api('/equipment').then(value=>{if(epoch===authEpoch)fleet.value=value;}));}
      else {
        // WebSocket carries live state. HTTP is an initial/manual refresh or
        // fallback when the stream is disconnected/stale.
        if(!poll||!connected.value||stale.value)jobs.push(api('/equipment/'+equipmentId.value).then(accept));
        if(['overview','alarms'].includes(tab.value))jobs.push(api('/commands').then(value=>{if(epoch===authEpoch)commands.value=value;}));
        if(tab.value==='overview'&&(!poll||Date.now()-lastTrendLoad>=15000)){lastTrendLoad=Date.now();jobs.push(api('/telemetry').then(value=>{if(epoch===authEpoch)trend.value=value.reverse();}));}
        if(tab.value==='alarms')jobs.push(api('/alarms').then(value=>{if(epoch===authEpoch)alarms.value=value;}),api('/equipment/'+equipmentId.value+'/audit').then(value=>{if(epoch===authEpoch)auditRows.value=value;}));
      }
      await Promise.all(jobs);
      if(epoch===authEpoch)error.value='';
    }catch(e){if(epoch===authEpoch)error.value=e.message;}
    finally{if(refreshTask===task)refreshTask=null;}
  })();
  refreshTask=task;
  return task.promise;
}

function connect() {
  if(!user.value || closed||showFleet.value||document.hidden)return;
  if(ws&&[WebSocket.CONNECTING,WebSocket.OPEN].includes(ws.readyState))return;
  const epoch=authEpoch;
  ws = new WebSocket(
    `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/ws?equipmentId=${equipmentId.value}`,
  );
  const socket=ws;
  ws.onopen = () => {
    if(epoch!==authEpoch)return;
    connected.value = true;
    refresh();
  };
  ws.onmessage = (e) => {if(user.value && epoch===authEpoch)accept(JSON.parse(e.data));};
  ws.onclose = () => {
    if(socket!==ws)return;
    connected.value = false;
    if (!closed && user.value&&!showFleet.value&&!document.hidden) reconnect = setTimeout(connect, 2000);
  };
}
watch(showFleet,()=>{
  phmOpen.value=false;
  clearTimeout(reconnect);
  if(showFleet.value){ws?.close();ws=null;connected.value=false;}
  else connect();
  refresh();
});
function visibilityRefresh(){
  if(document.hidden){clearTimeout(reconnect);ws?.close();ws=null;connected.value=false;}
  else if(user.value){connect();refresh();}
}

async function send(
  type,
  targetId = "",
  id = active.value?.id || "",
  parameters = {},
) {
  busy.value = true;
  try {
    const commandId=crypto.randomUUID();
    await api("/commands", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        commandId,
        type,
        targetId,
        lotId: id,
        parameters,
        expiresAt: new Date(Date.now() + 30000).toISOString(),
      }),
    });
    await refresh();
    return commandId;
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
function start() {
  send("start", "", `${equipmentId.value}-LOT-${crypto.randomUUID()}`, {
    lp: lp.value,
    ll: ll.value,
  });
}
async function loadHistory() {
  try {
    const timeQuery =
      (rangeFrom.value
        ? "&from=" + encodeURIComponent(taipeiInputISO(rangeFrom.value))
        : "") +
      (rangeTo.value
        ? "&to=" + encodeURIComponent(taipeiInputISO(rangeTo.value))
        : "");
    history.value = await api(
      (waferId.value
        ? `/wafers/${encodeURIComponent(waferId.value)}/history`
        : "/history") +
        `?lotId=${encodeURIComponent(lotId.value)}&page=${page.value}` +
        timeQuery,
    );
    historicalTrend.value = (
      await api(
        "/telemetry?waferId=" + encodeURIComponent(waferId.value) + timeQuery,
      )
    ).reverse();
  } catch (e) {
    error.value = e.message;
  }
}
function points(source = trend.value) {
  const values = source
    .slice(-90)
    .map(
      (t) =>
        t.payload?.chambers?.find((p) => p.id === selected.value)
          ?.temperature ??
        t.payload?.meanTemperature ??
        0,
    );
  return values
    .map(
      (v, i) =>
        `${(i * 500) / Math.max(values.length - 1, 1)},${100 - Math.min(v, 500) / 5}`,
    )
    .join(" ");
}
function returnToFleet(event) {
  if(event.key!=='Escape'||event.repeat||event.isComposing||event.defaultPrevented||!user.value||showFleet.value||tab.value!=='overview')return;
  if(document.querySelector('dialog[open], [aria-modal="true"]')||event.target?.closest('input, textarea, select, [contenteditable="true"]'))return;
  event.preventDefault();
  showFleet.value=true;
}
onMounted(async () => {
  document.addEventListener('keydown',returnToFleet);
  document.addEventListener('pointerdown',activity); document.addEventListener('keydown',activity);
  try {csrf=(await api('/auth/csrf')).token; const identity=await api('/auth/me');setLocale((await api('/auth/preferences')).locale);user.value=identity;authError.value='';connect();await refresh();await followNotification();}
  catch {authError.value='';}finally{checking.value=false;}
  timer=setInterval(()=>{now.value=Date.now();refresh({poll:true});},3000);
  document.addEventListener('visibilitychange',visibilityRefresh);
});
onUnmounted(() => {
  document.removeEventListener('visibilitychange',visibilityRefresh);
  document.removeEventListener('keydown',returnToFleet);
  document.removeEventListener('pointerdown',activity);document.removeEventListener('keydown',activity);
  closed = true;
  clearInterval(timer);
  clearTimeout(reconnect);
  ws?.close();
});
</script>
<template>
  <div v-if="!user" class="auth-shell">
    <form class="panel auth-card" @submit.prevent="login">
      <div class="theme-tools"><ThemeToggle/><LanguageSelect :api="api" @selected="loginLocale=$event" /></div><div class="eyebrow">TiN · PROCESS INTELLIGENCE</div><h1>{{tr('ui045')}}</h1>
      <p>{{tr('ui046')}}</p>
      <label>{{tr('ui047')}}<input v-model="loginName" autocomplete="username" required :disabled="checking" /></label>
      <label>{{tr('ui048')}}<input v-model="loginPassword" type="password" autocomplete="current-password" required :disabled="checking" /></label>
      <p v-if="authError" role="alert" class="notice">{{authError}}</p>
      <button class="primary" :disabled="busy || checking">{{checking?tr('ui049'):busy?tr('ui050'):tr('ui051')}}</button>
    </form>
  </div>
  <div v-else class="shell">
    <aside class="nav">
      <div class="brand">T<span>i</span>N<span class="brand-dot"></span></div>
      <div class="nav-title">PROCESS INTELLIGENCE</div>
      <SidebarButton :icon="['equipment','history','alarm','disposition'][i]"
        v-for="(page, i) in ['overview', 'history', 'alarms', 'disposition']"
        :class="{ chosen: tab === page }"
        @click="
          showFleet=false;tab = page;
          if (i === 1) loadHistory();
        "
      >
        {{ tr(page) }}
      </SidebarButton>
      <SidebarButton icon="report" :class="{chosen:tab==='reports'}" @click="showFleet=false;tab='reports'">{{tr('reports')}}</SidebarButton>
      <SidebarButton icon="accounts" v-if="user.role==='Leader'" :class="{chosen:tab==='accounts'}" @click="showFleet=false;tab='accounts';loadAccounts()">{{tr('ui052')}}</SidebarButton>
      <div class="nav-bottom">
        <b>{{user.username}}</b><p>{{roleLabel(user.role)}} · {{user.department||tr('ui053')}}</p><SidebarButton icon="logout" @click="logout">{{tr('ui054')}}</SidebarButton>
        <div class="nav-equipment"><b>{{equipmentId}}</b>
        <p>{{tr('ui055')}}</p>
        <small>SECS/GEM → Sparkplug B → Kafka</small></div>
      </div>
    </aside>
    <main>
      <header v-if="showFleet"><div><div class="eyebrow">{{tr('digitalTwin')}}</div><h1>{{tr('ui056')}}<span class="badge">{{tr('ui057')}}</span></h1><p>{{tr('ui058')}}</p></div></header>
      <div class="theme-tools"><ThemeToggle/><LanguageSelect :user="user" :api="api" /></div><div class="fleet-toolbar"><button @click="showFleet=true" :title="!showFleet && tab==='overview' ? tr('ui059') : tr('ui060')">{{tr('ui060')}}{{!showFleet && tab==='overview' ? ' · Esc' : ''}}</button><label>{{tr('ui061')}}<select :value="equipmentId" @change="selectEquipment($event.target.value)"><option v-for="n in 10" :key="n">{{'TEL-CVDTIN-08A-'+String(n).padStart(2,'0')}}</option></select></label></div>
      <CleanroomOverview :api="api" v-if="showFleet" :fleet="fleet" :active="showFleet" :state="cleanroomState" :now="now" @select="selectEquipment" />
      <template v-if="!showFleet">
      <header>
        <div>
          <div class="eyebrow">{{tr('digitalTwin')}}</div>
          <h1>{{ tr(tab) }}<span class="badge">{{tr('ui062')}}</span></h1>
          <p>{{tr('ui063')}}</p>
        </div>
        <div class="connection">
          <i class="status-glow" :data-status="stale?'stale':'idle'" :style="glowStyle(appearance(stale?'stale':'idle'))" :class="{ online: connected && !stale }"></i
          >{{ stale ? tr('ui064') : connected ? tr('ui065') : tr('ui066')
          }}<small>{{
            data.sourceTime
              ? dateTime(data.sourceTime)
              : tr('ui067')
          }}</small>
        </div>
      </header>
      <div v-if="error" class="notice">{{ error }}</div>
      <div v-if="tab!=='reports' && data.paused" class="notice">
        {{tr('ui068')}}<button v-if="canEngineer"
          :title="data.maintenance&&data.maintenance.phase!=='released'?tr('ui069'):data.dispositionHold?tr('ui070'):''" :disabled="busy || stale || (!!data.maintenance&&data.maintenance.phase!=='released')" @click="data.dispositionHold?(tab='disposition'):send('resume')"
        >
          {{tr('ui071')}}
        </button>
      </div>
      <div v-if="tab!=='reports' && data.dispositionHold" class="notice">{{tr('ui072')}}<button @click="tab='disposition'">{{tr('ui073')}}</button></div>
      <section class="metrics" v-if="tab!=='reports'">
        <article>
          <span>{{tr('ui074')}}</span
          ><strong
            >{{
              data.chambers.filter(
                (p) => p.waferId && !p.fault && p.remaining > 0,
              ).length
            }}<small>/ 4</small></strong
          ><em>{{tr('ui075')}}</em>
        </article>
        <article>
          <span>{{tr('ui076')}}</span><strong>{{ success }}<small>{{tr('ui077')}}</small></strong
          ><em>{{tr('ui078')}}</em>
        </article>
        <article>
          <span>{{tr('ui079')}}</span
          ><strong
            >{{ lots.filter((l) => l.phase !== "complete").length
            }}<small>{{tr('ui080')}}</small></strong
          ><em>{{tr('ui081')}}</em>
        </article>
        <article>
          <span>{{tr('ui082')}}</span><strong>{{ scrap }}<small>{{tr('ui077')}}</small></strong
          ><em>{{tr('ui083')}}</em>
        </article>
      </section>
      <template v-if="tab === 'overview'"
        ><MaintenancePanel :key="equipmentId" :data="data" :user="user" :api="api" :stale="stale" @refresh="refresh" @wafer="waferId=$event;tab='history';loadHistory()" /><details class="panel" @toggle="phmOpen=$event.target.open"><summary>{{tr('phmTitle')}}</summary><VacuumPanel :active="phmOpen" :data="data" :api="api" :user="user" @refresh="refresh" /></details><div class="workspace">
          <section class="panel topology">
            <div class="panel-title">
              <h2>{{tr('ui084')}}</h2>
              <span>3 LP · 3 LL · 4 PM</span>
            </div>
            <div class="legend">
              <span><i class="green"></i>{{tr('ui021')}}</span
              ><span><i class="blue"></i>{{tr('ui020')}}</span
              ><span><i class="amber"></i>{{tr('ui085')}}</span
              ><span><i class="gray"></i>{{tr('ui086')}}</span>
            </div>
            <EquipmentTopology :key="equipmentId" :data="data" :selected="selected" :stale="stale" :label="label"
              @select="selected = $event" @wafer="waferId = $event; tab = 'history'; loadHistory()" />
          </section>
          <aside class="panel detail">
            <div class="panel-title">
              <h2>{{ selected }} {{tr('ui087')}}</h2>
              <span>LIVE</span>
            </div>
            <div class="reading">
              <span>{{tr('ui088')}} {{ label(pm?.step) }}</span
              ><strong
                >{{ fixed(pm?.temperature || 0,1)
                }}<small>°C</small></strong
              ><small>{{tr('ui035')}} {{ pm?.targetTemperature || 0 }} °C</small>
            </div>
            <svg viewBox="0 0 500 110" class="chart">
              <path d="M0 100H500 M0 50H500" />
              <polyline :points="points()" />
            </svg>
            <div class="pair">
              <div>
                <small>{{tr('ui089')}}</small><b>{{ pm?.pressure == null ? "—" : fixed(pm.pressure,2) }} mbar</b>
              </div>
              <div>
                <small>{{tr('ui090')}}</small><b>{{ pm?.flow == null ? "—" : fixed(pm.flow,2) }} sccm</b>
              </div>
            </div>
            <h3 v-if="canEngineer">{{tr('ui091')}}</h3>
            <div v-if="canEngineer" class="buttons exception-actions">
              <button
                :disabled="busy || stale"
                @click="send('fault', selected)"
              >
                {{tr('ui092')}}</button
              ><button
                :disabled="busy || stale"
                @click="send('clear', selected)"
              >
                {{tr('ui033')}}</button
              ><button
                :disabled="busy || stale"
                @click="recoveryWafer='';tab='disposition'"
              >
                {{tr('ui093')}}</button
              ><button
                class="danger"
                :disabled="busy || stale"
                @click="recoveryWafer='';tab='disposition'"
              >
                {{tr('ui094')}}
              </button>
              <button v-if="data.paused" class="wide" :disabled="busy || stale" @click="send('recovery-clock')">{{tr('ui095')}}</button>
            </div>
            <h3>{{tr('ui096')}}</h3>
            <div class="buttons">
              <button :disabled="!active || busy" @click="send('pause')">
                {{tr('ui030')}}</button
              ><button :disabled="!active || busy" @click="send('stop')">
                {{tr('ui031')}}</button
              ><button
                class="primary wide"
                v-if="canEngineer" :title="data.maintenance&&data.maintenance.phase!=='released'?tr('ui069'):data.dispositionHold?tr('ui070'):''" :disabled="busy || stale || (!!data.maintenance&&data.maintenance.phase!=='released')" @click="data.dispositionHold?(tab='disposition'):send('resume')"
              >
                {{tr('ui097')}}
              </button>
            </div>
            <h3>{{tr('ui098')}}</h3>
            <div class="buttons">
              <select v-model="lp">
                <option v-for="p in ['LP1', 'LP2', 'LP3']">
                  {{ p }}
                </option></select
              ><select v-model="ll">
                <option v-for="l in ['LL1', 'LL2', 'LL3']">
                  {{ l }}
                </option></select
              ><button
                class="primary wide"
                :disabled="busy || stale"
                @click="start"
              >
                {{tr('ui099')}}
              </button>
            </div>
            <details v-if="canEngineer" class="recipe-editor">
              <summary @click="loadRecipes().catch(e=>recipeFeedback=e.message)">{{tr('ui100')}}</summary>
              <form @submit.prevent="applyRecipe(false)">
                <label v-for="(key,i) in recipeKeys" :key="key">{{recipeLabels[i]}}{{tr('ui101')}}
                  <input type="number" v-model.number="recipeDraft[key]" min="1" max="3600" step="1" required :disabled="recipeLocked || busy || !!recipeCommand" @input="recipeDirty=true; recipeFeedback=''" />
                </label>
                <p>{{tr('ui102')}}<b>{{recipeTotal}}</b> {{tr('ui103')}}</p>
                <small>{{tr('ui104')}}</small>
                <p>{{tr('ui105')}}</p>
                <p v-if="recipeDirty && !recipeCommand">{{tr('ui106')}}</p>
                <p v-if="recipeFeedback" role="status">{{recipeFeedback}}</p>
                <div class="buttons"><button class="primary" :disabled="!recipeValid || recipeLocked || busy || stale || !!recipeCommand">{{tr('ui107')}}</button>
                <button type="button" :disabled="!!recipeCommand" @click="resetRecipe">{{tr('ui108')}}</button></div><hr/><h4>{{tr('ui109')}}</h4><label>{{tr('ui110')}}<select v-model="copySource" @change="loadCopyVersions"><option v-for="m in fleet.filter(m=>m.equipmentId!==equipmentId)" :value="m.equipmentId">{{m.equipmentId}}</option></select></label><button type="button" @click="loadCopyVersions">{{tr('ui111')}}</button><label>{{tr('ui112')}}<select v-model="copyVersion"><option v-for="v in copyVersions" :value="v.id">{{v.id}}</option></select></label><p v-if="copyVersion">{{tr('ui113')}}{{copyVersions.find(v=>v.id===copyVersion)?.settings.stepSeconds}}</p><button type="button" :disabled="!copyVersion || !!recipeCommand || stale" @click="applyRecipe(true)">{{tr('ui114')}} {{equipmentId}}</button><p v-for="v in (recipeVersions?.versions||[]).slice(0,3)">{{v.id}} · {{v.source_equipment ? tr('ui115')+v.source_equipment+' / '+v.source_version : tr('ui116')}} · {{v.actor?.username}}</p>
              </form>
            </details>
          </aside>
        </div></template
      >
      <section v-else-if="tab === 'history'" class="panel">
<MetrologyPanel :api="api" :equipment-id="equipmentId" :wafer-id="waferId" :user="user" />
        <div class="panel-title">
          <h2>{{tr('ui117')}}</h2><button @click="recoveryWafer=waferId;tab='disposition'">{{tr('ui118')}}</button>
          <span>{{tr('ui119')}}</span>
        </div>
        <div class="filters">
          <input v-model="lotId" :placeholder="tr('ui120')" /><input
            v-model="waferId"
            :placeholder="tr('ui121')"
          /><input
            type="datetime-local"
            v-model="rangeFrom"
            :aria-label="tr('ui122')"
          /><input
            type="datetime-local"
            v-model="rangeTo"
            :aria-label="tr('ui123')"
          /><button
            class="primary"
            @click="
              page = 0;
              loadHistory();
            "
          >
            {{tr('ui124')}}
          </button>
        </div>
        <div class="panel-title">
          <h3>{{tr('ui125')}} {{ selected }}</h3>
          <select v-model="selected" style="width: 100px">
            <option v-for="p in ['PM1', 'PM2', 'PM3', 'PM4']">{{ p }}</option>
          </select>
        </div>
        <svg viewBox="0 0 500 110" class="chart">
          <polyline :points="points(historicalTrend)" />
        </svg>
        <ProcessPanel :key="equipmentId" :equipment-id="selected" :data="data" :wafer-id="waferId" :historical="true" />
        <EventRecords :rows="history" history />
        <div v-if="!history.length" class="empty-state">
          {{tr('ui133')}}
        </div>
        <div class="filters">
          <button
            :disabled="page === 0"
            @click="
              page--;
              loadHistory();
            "
          >
            {{tr('ui134')}}</button
          ><span>{{tr('pageCount',{page:page+1})}}</span
          ><button
            :disabled="history.length < 100"
            @click="
              page++;
              loadHistory();
            "
          >
            {{tr('ui137')}}
          </button>
        </div>
      </section>
      <RecoveryPanel :key="equipmentId" v-else-if="tab==='disposition'" :user="user" :data="data" :api="api" :initial-pm="selected" :initial-wafer="recoveryWafer" :initial-case="recoveryCase" />
      <ReportPanel v-else-if="tab==='reports'" :api="api" :equipment-id="equipmentId" />
      <section v-else-if="tab==='accounts' && user.role==='Leader'" class="panel">
        <h2>{{tr('ui138')}}</h2><p>{{tr('ui139')}}</p>
        <form class="account-form" @submit.prevent="createUser">
          <label>{{tr('ui140')}}<input v-model="newName" autocomplete="off" pattern="[A-Za-z0-9][A-Za-z0-9._-]{2,63}" required /></label>
          <label>{{tr('ui141')}}<input v-model="newPassword" type="password" autocomplete="new-password" minlength="12" required /></label>
          <label>{{tr('ui142')}}<select v-model="newRole"><option value="Operator">{{roleLabel('Operator')}}</option><option value="Engineer">{{roleLabel('Engineer')}}</option></select></label>
          <label>{{tr('ui143')}}<select v-model="newDepartment"><option value="">{{tr('ui144')}}</option><option>EE</option><option>PE</option></select></label>
          <button class="primary" :disabled="busy">{{tr('ui145')}}</button><p role="status">{{accountMessage}}</p>
        </form>
        <h3>{{tr('ui146')}}</h3><p>{{tr('ui147')}}</p>
        <div v-for="a in accountRows" :key="a.id" class="queue-row email-account"><b>{{a.username}}</b><span>{{roleLabel(a.role)}}</span><label>{{tr('ui143')}}<select :aria-label="a.username+tr('ui148')" :value="a.department||''" @change="assignDepartment(a,$event.target.value)"><option value="">{{tr('ui144')}}</option><option>EE</option><option>PE</option></select></label><form class="email-account-form" @submit.prevent="saveEmail(a)"><label>Email<input v-model="a.email" type="email" :aria-label="a.username+' Email'" /></label><label><input v-model="a.email_enabled" type="checkbox" />{{tr('ui149')}}</label><button>{{tr('ui150')}}</button></form></div>
        <NotificationPanel :api="api" />
      </section>
      <template v-else
        ><section class="panel">
          <div class="panel-title">
            <h2>{{tr('ui151')}}</h2>
            <span>{{ alarms.length }} {{tr('ui152')}}</span>
          </div>
          <VisionAlerts :api="api" :equipment-id="equipmentId" :user="user"/>
          <EventRecords :rows="alarms" />
          <div v-if="!alarms.length" class="empty-state">
            {{tr('ui153')}}
          </div>
        </section>
        <section class="panel command-panel"><h2>{{tr('ui154')}}</h2>
          <div v-for="a in auditRows" :key="a.id" class="queue-row"><span>{{dateTime(a.time)}}</span><b>{{a.username || tr('ui132')}}</b><span>{{enumLabel(a.action)}} · {{a.target}}</span><span>{{enumLabel(a.outcome)}} / {{enumLabel(a.execution_status)}}</span></div>
        </section>
        <section class="panel command-panel">
          <div class="panel-title">
            <h2>{{tr('ui155')}}</h2>
            <span>{{tr('ui156')}}</span>
          </div>
          <div v-for="c in commands" class="queue-row">
            <span class="mono">{{ c.id.slice(0, 12) }}</span><span>{{c.actor || tr('ui132')}}</span>
            <span>{{label(c.type)}}<small>{{c.target_id||c.lot_id}}</small></span>
            <span>{{
              c.timed_out && ["received", "accepted"].includes(c.status)
                ? tr('ui157')
                : label(c.status)
            }}</span
            ><span>{{
              c.result ? systemMessage(JSON.parse(c.result)) : tr('ui158')
            }}</span>
          </div>
        </section></template
      >
      </template>
      <footer>
        {{tr('ui159')}}
        <span>PostgreSQL / TimescaleDB / Redis</span>
      </footer>
    </main>
  </div>
</template>
