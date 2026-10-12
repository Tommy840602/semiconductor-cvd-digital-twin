import {t,number} from './i18n.js';
import {maintenanceLabel} from './maintenance.js';
const states=()=>({processing:[t('ui017'),'#21977f',2],paused:[t('ui203'),'#c3a666',2],fault:[t('ui041'),'#df855a',1],stale:[t('ui064'),'#91a2a5',3],idle:[t('idle'),'#619caf',0],maintenance:[t('maintActive'),'#9975b1',0]});
export function appearance(key,label){const [name,color,period]=states()[key];return {key,label:label||name,color,period};}
export function isStale(data,now=Date.now()){return !data?.sourceTime||!Number.isFinite(Date.parse(data.sourceTime))||now-Date.parse(data.sourceTime)>5000||!!(data.communicationQuality&&data.communicationQuality!=='good');}
export function machineStatus(machine,now=Date.now()){
 if(isStale(machine,now))return appearance('stale');
 if(machine.faults)return appearance('fault');
 if(machine.vacuumFault)return appearance('fault',t('phmFault'));
 const maintenance=maintenanceLabel(machine.maintenance);if(maintenance)return appearance('maintenance',maintenance);
 if(machine.vacuumHold)return appearance('paused',t('phmPending'));
 if(machine.paused)return appearance('paused');
 if(machine.vacuumWarning)return appearance('paused',t('phmWarning'));
 return appearance(machine.activeLots?'processing':'idle');
}
export function chamberStatus(data,p,stale){
 if(stale)return appearance('stale');
 if(p.fault)return appearance('fault');
 if(data.paused)return appearance('paused');
 const wafer=(data.lots||[]).flatMap(l=>l.wafers||[]).find(w=>w.id===p.waferId);
 if(p.waferId&&p.remaining>0&&!['exception','awaiting_result','success','scrap'].includes(wafer?.result))return appearance('processing');
 const maintenance=maintenanceLabel(data.maintenance);return maintenance?appearance('maintenance',maintenance):appearance('idle');
}
export function glowStyle(s){return {'--status-color':s.color,'--status-period':`${s.period||1}s`};}
export function glowLevel(s,time,reduced=false){if(!s.period||reduced)return .65;return .08+.92*(1-Math.cos(time/1000/s.period*Math.PI*2))/2;}
