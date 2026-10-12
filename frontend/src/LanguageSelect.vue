<script setup>
import {ref} from 'vue';
import {t as tr,uiMessage,locale,setLocale} from './i18n.js';
const props=defineProps({user:Object,api:Function});
const emit=defineEmits(['selected']);
const busy=ref(false),error=ref('');
async function change(event){const value=event.target.value;event.target.value=locale.value;busy.value=true;error.value='';try{if(props.user)await props.api('/auth/preferences',{method:'PATCH',body:JSON.stringify({locale:value})});setLocale(value);emit('selected',value);}catch{error.value=uiMessage('saveFailed');}finally{busy.value=false;}}
</script>
<template><div class="language-switch"><label><span aria-hidden="true">🌐</span><select :aria-label="tr('language')" :value="locale" :disabled="busy" @change="change"><option value="zh-TW">繁體中文</option><option value="en">English</option><option value="ja">日本語</option></select></label><p v-if="error" role="alert">{{error}}</p></div></template>
<style scoped>.language-switch{display:flex;flex-direction:column;align-items:flex-end;gap:4px}.language-switch label{display:flex;align-items:center;gap:8px}.language-switch select{width:auto;min-width:130px}.language-switch p{font-size:12px;max-width:280px}.language-switch{margin:0 0 12px}</style>
