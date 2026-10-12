<script setup>
import {t as tr,dateTime,number} from "./i18n.js";
import { computed, useId } from 'vue';
const props=defineProps({robot:Object,moving:Boolean,waypoints:Array});
const uid=useId().replaceAll(':',''),metal=`robot-metal-${uid}`,dark=`robot-dark-${uid}`,wafer=`robot-wafer-${uid}`;
const vacuum=computed(()=>props.robot.name==='VAC');
const center=computed(()=>props.robot.center);
const frames=computed(()=>{
 const [cx,cy]=center.value;
 const targets=props.moving?props.waypoints:props.robot.waferId?[[cx+(vacuum.value?-29:29),cy-9]]:[[cx+(vacuum.value?-33:34),cy-12]];
 return targets.map(([tx,ty])=>{
  const dx=tx-cx,dy=ty-cy,d=Math.hypot(dx,dy)||1;
  const bend=Math.min(vacuum.value?28:19,Math.max(12,d*.3));
  const side=vacuum.value?1:-1;
  const elbow=[cx+dx*.48-dy/d*bend*side,cy+dy*.48+dx/d*bend*side];
  const angle=Math.atan2(ty-elbow[1],tx-elbow[0]);
  const wrist=[tx-Math.cos(angle)*13,ty-Math.sin(angle)*13];
  return {elbow,wrist,tip:[tx,ty],angle:angle*180/Math.PI};
 });
});
// All links, joints, fork and carried wafer share the same timed keyframes.
const first=computed(()=>frames.value[0]);
const times=computed(()=>frames.value.map((_,i)=>i/(frames.value.length-1)).join(';'));
const values=(part,axis)=>frames.value.map(f=>f[part][axis]).join(';');
const rotation=computed(()=>{
 let last=frames.value[0].angle;
 return frames.value.map((f,i)=>{let a=f.angle;if(i){while(a-last>180)a-=360;while(a-last< -180)a+=360;}last=a;return `${a}`;}).join(';');
});
const links=computed(()=>[
 {from:center.value,to:first.value.elbow,start:null,end:'elbow',wide:15},
 {from:first.value.elbow,to:first.value.wrist,start:'elbow',end:'wrist',wide:11}
]);
</script>
<template>
 <g class="precision-robot" :class="{vacuum,moving}" :aria-label="tr('robotDescription',{name:robot.name})">
  <defs>
   <linearGradient :id="metal" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#f5faf8"/><stop offset=".3" stop-color="#cfdfd9"/><stop offset=".55" stop-color="#91b0a4"/><stop offset=".8" stop-color="#e6efeb"/><stop offset="1" stop-color="#a1bdb1"/></linearGradient>
   <linearGradient :id="dark" x1="0" y1="0" x2="1" y2="1"><stop stop-color="#547d70"/><stop offset="1" stop-color="#234e44"/></linearGradient>
   <radialGradient :id="wafer" cx=".32" cy=".28"><stop stop-color="#d6faf0"/><stop offset=".65" stop-color="#89d3be"/><stop offset="1" stop-color="#48a98e"/></radialGradient>
  </defs>
  <g :transform="`translate(${center})`">
   <ellipse cx="1" cy="5" :rx="vacuum?27:32" ry="20" class="base-shadow"/>
   <template v-if="vacuum">
    <circle r="25" :fill="`url(#${metal})`" class="housing-edge"/>
    <circle r="21" fill="#e6efea" stroke="#92b1a4" stroke-width=".8"/>
    <path v-for="angle in [0,60,120,180,240,300]" :key="angle" d="M0 -19 V-22" :transform="`rotate(${angle})`" class="index-mark"/>
    <g v-for="angle in [30,150,270]" :key="angle" :transform="`rotate(${angle}) translate(0 -19)`"><circle r="2.1" class="bolt"/><path d="M-1.1 0H1.1" class="bolt-slot"/></g>
    <circle r="15" :fill="`url(#${dark})`" stroke="#315f50"/>
   </template>
   <template v-else>
    <rect x="-34" y="-12" width="68" height="24" rx="6" fill="#d4e4dd" stroke="#a6c1b5"/>
    <path d="M-28 -7H28 M-28 7H28" stroke="#eef6f2" stroke-width="2"/>
    <rect x="-21" y="-18" width="42" height="36" rx="8" :fill="`url(#${metal})`" class="housing-edge"/>
    <g v-for="x in [-15,15]" :key="x"><circle v-for="y in [-12,12]" :key="y" :cx="x" :cy="y" r="1.7" class="bolt"/></g>
    <circle r="14" :fill="`url(#${dark})`" stroke="#315f50"/>
   </template>
  </g>
  <g v-for="(link,i) in links" :key="i">
   <line v-for="layer in ['edge','metal','highlight']" :key="layer" :x1="link.from[0]" :y1="link.from[1]" :x2="link.to[0]" :y2="link.to[1]"
    :stroke="layer==='edge'?'#527b6d':layer==='metal'?`url(#${metal})`:'#f4faf7'" :stroke-width="layer==='edge'?link.wide:layer==='metal'?link.wide-2:1.3" stroke-linecap="round" :opacity="layer==='highlight'?.7:1">
    <template v-if="moving">
     <animate v-if="link.start" attributeName="x1" :values="values(link.start,0)" :keyTimes="times" dur=".85s" fill="freeze"/>
     <animate v-if="link.start" attributeName="y1" :values="values(link.start,1)" :keyTimes="times" dur=".85s" fill="freeze"/>
     <animate attributeName="x2" :values="values(link.end,0)" :keyTimes="times" dur=".85s" fill="freeze"/>
     <animate attributeName="y2" :values="values(link.end,1)" :keyTimes="times" dur=".85s" fill="freeze"/>
    </template>
   </line>
  </g>
  <g v-for="joint in ['elbow','wrist']" :key="joint">
   <circle v-for="(radius,i) in joint==='elbow'?[8,5,1.8]:[5.3,3,1.2]" :key="i" :cx="first[joint][0]" :cy="first[joint][1]" :r="radius" :fill="i===0?'#567e70':i===1?`url(#${metal})`:'#52786b'" :stroke="i===0?'#3e6859':'none'" stroke-width=".8">
    <template v-if="moving"><animate attributeName="cx" :values="values(joint,0)" :keyTimes="times" dur=".85s" fill="freeze"/><animate attributeName="cy" :values="values(joint,1)" :keyTimes="times" dur=".85s" fill="freeze"/></template>
   </circle>
  </g>
  <g :transform="`translate(${center})`">
   <circle r="8" :fill="`url(#${metal})`" stroke="#406b59" stroke-width="1.2"/><circle r="3" fill="#3d6e5b"/><path d="M-1.5 0H1.5" stroke="#b8d8c9" stroke-width="1"/>
   <circle :cx="vacuum?18:27" :cy="vacuum?9:0" r="2.4" :fill="robot.waferId?'#36c99b':'#94baa9'" stroke="#e6f7ef" stroke-width=".8"/>
  </g>
  <g :transform="`translate(${first.tip})`">
   <animateTransform v-if="moving" attributeName="transform" type="translate" :values="frames.map(f=>f.tip.join(' ')).join(';')" :keyTimes="times" dur=".85s" fill="freeze"/>
   <g :transform="`rotate(${first.angle})`">
    <animateTransform v-if="moving" attributeName="transform" type="rotate" :values="rotation" :keyTimes="times" dur=".85s" fill="freeze"/>
    <path d="M-14 -4 L-5 -4 L0 -9 L13 -9 L15 -7 L1 -6 L-3 0 L1 6 L15 7 L13 9 L0 9 L-5 4 L-14 4 Z" :fill="`url(#${metal})`" stroke="#527b6d" stroke-width="1"/>
    <path d="M1 -7H11 M1 7H11" stroke="#f7fbf9" stroke-width="1.1"/>
   </g>
   <g v-if="robot.waferId"><circle r="12" :fill="`url(#${wafer})`" stroke="#328b71" stroke-width="1.2"/><path d="M-7 -7 A10 10 0 0 1 6 -8 M-4 10 H4" fill="none" stroke="#e2fff5" stroke-width="1.1"/><circle r="8.5" fill="none" stroke="#e1f7ee" stroke-width=".5" opacity=".5"/><title>{{tr('ui527')}}</title></g>
  </g>

 </g>
</template>
<style scoped>
.precision-robot{pointer-events:none}.base-shadow{fill:var(--fg);opacity:.1}.housing-edge{stroke:var(--muted);stroke-width:1.3}.index-mark{stroke:var(--muted);stroke-width:1}.bolt{fill:var(--muted);stroke:var(--border);stroke-width:.5}.bolt-slot{stroke:var(--border);stroke-width:.7}
</style>
