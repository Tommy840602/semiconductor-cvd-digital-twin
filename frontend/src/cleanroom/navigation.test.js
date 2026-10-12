import {test} from 'node:test';
import assert from 'node:assert/strict';
import {positions,ids,move,route,status,bounds,nearestWaypoint} from './navigation.js';
test('unique identities and opposing five-tool rows',()=>{assert.equal(new Set(ids).size,10);assert.equal(positions.filter(p=>p.x<0).length,5);assert.equal(positions.filter(p=>p.x>0).length,5);assert.equal(positions[0].rotation,-positions[5].rotation);});
test('continuous boundary protection, including large frame gaps',()=>{let p={x:0,z:0};for(let i=0;i<10000;i++)p=move(p,{x:1,z:1},10);assert.ok(p.x<=bounds.maxX&&p.z<=bounds.maxZ);assert.ok(p.x>2.8&&p.z>19.8);});
test('diagonal does not gain speed',()=>{const a=move({x:0,z:0},{x:1,z:0},.02),b=move({x:0,z:0},{x:1,z:1},.02);assert.ok(Math.abs(Math.hypot(a.x,a.z)-Math.hypot(b.x,b.z))<1e-10);});
test('obstacle protection and sliding',()=>{let p={x:0,z:0};for(let i=0;i<500;i++)p=move(p,{x:1,z:0},.05,[{minX:1,maxX:2,minZ:-1,maxZ:1}]);assert.ok(p.x<=.7);});
test('route visits all ten stations and can resume locally',()=>{assert.equal(route.length,10);for(const [i,p] of route.entries())assert.equal(nearestWaypoint(p),i);});
test('fresh, fault and stale are isolated per equipment',()=>{const now=Date.now(),fresh={sourceTime:new Date(now).toISOString(),communicationQuality:'good'};assert.equal(status({...fresh,paused:true},now).label,'暫停');assert.equal(status({...fresh,faults:1},now).label,'故障');assert.equal(status({...fresh,communicationQuality:'stale'},now).label,'資料過期');assert.equal(status({...fresh,activeLots:1},now).label,'加工中');assert.equal(status({sourceTime:'bad'},now).label,'資料過期');});

test('Enter proximity selects only the reachable same-side Load Port',async()=>{
 const {nearbyLoadPort}=await import('./navigation.js');
 for(const p of positions)assert.equal(nearbyLoadPort({mode:'manual',view:'follow',x:Math.sign(p.x)*2.8,z:p.z}),p.id);
 for(const s of [{x:0,z:0},{x:2.8,z:3.9},{x:3.5,z:0},{x:2.8,z:NaN}])assert.equal(nearbyLoadPort({mode:'manual',view:'follow',...s}),null);
 assert.equal(nearbyLoadPort({mode:'manual',view:'overview',x:2.8,z:0}),null);
});

test('automatic patrol permits entry at all ten frontages in either camera view',async()=>{
 const {nearbyLoadPort}=await import('./navigation.js');
 for(const view of ['overview','follow']){
  for(const point of route){
   const expected=positions.find(p=>Math.sign(p.x)===Math.sign(point.x)&&p.z===point.z).id;
   assert.equal(nearbyLoadPort({mode:'auto',view,...point}),expected);
   assert.equal(nearbyLoadPort({mode:'auto',view,x:point.x*.95,z:point.z+.1}),expected);
  }
  for(const point of [{x:0,z:15.6},{x:2,z:3.9},{x:3.5,z:0},{x:NaN,z:0},{x:2,z:Infinity}])
   assert.equal(nearbyLoadPort({mode:'auto',view,...point}),null);
 }
});
