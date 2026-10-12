import {ref} from 'vue';
const system=window.matchMedia('(prefers-color-scheme: dark)');
let saved;try{saved=localStorage.getItem('cvd.theme')}catch{}
let explicit=['light','dark'].includes(saved);
export const theme=ref(explicit?saved:system.matches?'dark':'light');
function apply(){document.documentElement.dataset.theme=theme.value;document.documentElement.style.colorScheme=theme.value;}
export function toggleTheme(){explicit=true;theme.value=theme.value==='dark'?'light':'dark';try{localStorage.setItem('cvd.theme',theme.value)}catch{}apply();}
system.addEventListener('change',event=>{if(!explicit){theme.value=event.matches?'dark':'light';apply();}});
apply();
