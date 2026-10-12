import test from 'node:test';
import assert from 'node:assert/strict';
import {machineStatus,chamberStatus,isStale,appearance,glowLevel} from './statusGlow.js';
const now=Date.now(),fresh={sourceTime:new Date(now).toISOString(),communicationQuality:'good'};
test('precedence and independently changing equipment',()=>{
 const m={...fresh,activeLots:1,paused:true,faults:1};
 assert.equal(machineStatus(m,now).key,'fault');assert.equal(machineStatus(m,now+5001).key,'stale');
 assert.equal(machineStatus({...m,faults:0},now).key,'paused');
 assert.equal(machineStatus({...fresh,activeLots:1},now).key,'processing');assert.equal(machineStatus(fresh,now).key,'idle');
 assert.equal(machineStatus({...m,faults:0,maintenance:{phase:'maintenance'}},now).key,'maintenance');
});
test('stale without a new event and invalid timestamps',()=>{for(const data of [{},{sourceTime:'bad'},{...fresh,communicationQuality:'stale'}])assert.ok(isStale(data,now));assert.equal(isStale(fresh,now+5000),false);assert.equal(isStale(fresh,now+5001),true);});
test('PM fault, global pause, dispatch hold and retained wafers',()=>{
 const p={waferId:'w',remaining:20},d={dispatchPaused:true,lots:[{wafers:[{id:'w',result:'pending'}]}]};
 assert.equal(chamberStatus(d,p,false).key,'processing');assert.equal(chamberStatus({...d,paused:true},p,false).key,'paused');assert.equal(chamberStatus({...d,paused:true},{...p,fault:true},false).key,'fault');assert.equal(chamberStatus(d,{...p,fault:true},true).key,'stale');
 for(const result of ['exception','awaiting_result','success','scrap'])assert.equal(chamberStatus({lots:[{wafers:[{id:'w',result}]}]},p,false).key,'idle');
});
test('complete periods and reduced motion',()=>{for(const [key,period] of [['processing',2],['paused',2],['fault',1],['stale',3]]){const s=appearance(key);assert.equal(s.period,period);assert.equal(glowLevel(s,0),glowLevel(s,period*1000));assert.ok(glowLevel(s,period*500)>glowLevel(s,0));assert.equal(glowLevel(s,0,true),glowLevel(s,700,true));}});

test('vacuum warnings and faults preserve freshness and maintenance priority',()=>{
 const now=Date.now(),base={sourceTime:new Date(now).toISOString(),vacuumWarning:true};
 assert.equal(machineStatus(base,now).key,'paused');
 assert.equal(machineStatus({...base,vacuumFault:true},now).key,'fault');
 assert.equal(machineStatus({...base,sourceTime:'2020-01-01'},now).key,'stale');
 assert.equal(machineStatus({...base,maintenance:{phase:'maintenance'}},now).key,'maintenance');
});
