import {createI18n} from 'vue-i18n';
import core from './locales/core.js';
import ui from './locales/interface.js';
import system from './locales/system.js';
import enums from './locales/enums.js';
import metrology from './locales/metrology.js';
import vacuum from './locales/vacuum.js';
import vision from './locales/vision.js';
import dummy from './locales/dummy.js';
import reports from './locales/reports.js';
const legacySystem=new Map(system.map(row=>[row.zh,row.code]));
export const messages=Object.fromEntries(Object.keys(core).map(lang=>[lang,{...core[lang],...ui[lang],...metrology[lang],...vacuum[lang],...vision[lang],...dummy[lang],...reports[lang],...Object.fromEntries(Object.entries(enums).map(([key,labels])=>['enum_'+key,labels[['zh-TW','en','ja'].indexOf(lang)]])),...Object.fromEntries(system.map(row=>[row.code,row[lang==='zh-TW'?'zh':lang]]))}]));
export const supportedLocales=['zh-TW','en','ja'];
function initialLocale(){try{const value=localStorage.getItem('cvd.locale');if(supportedLocales.includes(value))return value;}catch{}return 'zh-TW';}
export const i18n=createI18n({legacy:false,locale:initialLocale(),fallbackLocale:'zh-TW',messages:structuredClone(messages),missingWarn:false,fallbackWarn:false});
export const locale=i18n.global.locale;
export function t(key,params={}){if(!i18n.global.te(key,locale.value)&&!i18n.global.te(key,'zh-TW'))return i18n.global.t('unavailable');return i18n.global.t(key,params);}
export function setLocale(value){if(!supportedLocales.includes(value))throw new Error('Unsupported locale');locale.value=value;try{localStorage.setItem('cvd.locale',value);}catch{}if(typeof document!=='undefined')document.documentElement.lang=value==='zh-TW'?'zh-Hant':value;}
export function dateTime(value,options={}){if(!value)return '—';const date=new Date(value);if(Number.isNaN(date.getTime()))return '—';return new Intl.DateTimeFormat(locale.value,{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23',...options,timeZone:'Asia/Taipei'}).format(date)+' UTC+08:00';}
export function number(value,options={}){if(value==null||!Number.isFinite(Number(value)))return '—';return new Intl.NumberFormat(locale.value,options).format(Number(value));}
setLocale(locale.value);

// Only call for fields guaranteed to contain system-generated messages.
export function systemMessage(value){
 if(value==null||value==='')return '';
 if(value instanceof LiveMessage)return String(value);
 const message=typeof value==='string'?value:value.message||value.reason||'';
 const code=typeof value==='object'?(value.code||value.reasonCode):null;
 if(code&&code!=='original'&&i18n.global.te(code,'zh-TW'))return t(code,value.params||{});
 if(message.startsWith('vacuum.')&&i18n.global.te(message,'zh-TW'))return t(message);
 const legacy=legacySystem.get(message);if(legacy)return t(legacy);
 return t('original',{message});
}

export function fixed(value,digits=1){return number(value,{minimumFractionDigits:digits,maximumFractionDigits:digits,useGrouping:false});}
export function taipeiInputISO(value){if(!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?$/.test(value))throw new Error('Invalid local datetime');return new Date(value+'+08:00').toISOString();}
export function roleLabel(role){return ['Operator','Engineer','Leader'].includes(role)?t('role'+role):role;}
// Vue interpolation invokes this method during render, so pending feedback follows locale.
class LiveMessage {
 constructor(render){this.render=render;}
 toString(){return this.render();}
}
export const uiMessage=(key,params={})=>new LiveMessage(()=>t(key,params));
export const systemText=value=>new LiveMessage(()=>systemMessage(value));
export function localizedError(key){const error=new Error();error.message=uiMessage(key);return error;}

export function evidenceReason(value){if(!value?.reason)return '';return value.reason_code||value.reasonCode?systemMessage({code:value.reason_code||value.reasonCode,message:value.reason}):t('original',{message:value.reason});}

export function enumLabel(code){if(!code)return "—";return enums[code]?t("enum_"+code):t("original",{message:code});}

export function localizeValidation(input){
 if(!input?.validity||!input.setCustomValidity)return;
 input.setCustomValidity('');const v=input.validity;let message='';
 if(v.valueMissing)message=t('requiredField');
 else if(v.rangeUnderflow)message=t('minimumValue',{value:input.min});
 else if(v.rangeOverflow)message=t('maximumValue',{value:input.max});
 else if(v.tooShort)message=t('minimumLength',{value:input.minLength});
 else if(v.stepMismatch)message=t('integerStep');
 else if(!v.valid)message=t('invalidField');
 input.setCustomValidity(message);if(message)input.dataset.i18nValidation='true';else delete input.dataset.i18nValidation;
}
