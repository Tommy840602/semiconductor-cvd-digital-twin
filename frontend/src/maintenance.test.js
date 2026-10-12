import {test} from 'node:test';
import assert from 'node:assert/strict';
import {status} from './cleanroom/navigation.js';
import {maintenanceLabel} from './maintenance.js';
test('maintenance shown per tool without hiding stale or faults',()=>{
 const fresh={sourceTime:new Date().toISOString(),communicationQuality:'good'};
 const held={...fresh,maintenance:{phase:'maintenance'},paused:true};
 assert.equal(status(held).label,'保養中');
 assert.equal(status({...held,faults:1}).label,'故障');
 assert.equal(status({...held,communicationQuality:'stale'}).label,'資料過期');
 assert.notEqual(status(fresh).label,'保養中');
 assert.equal(maintenanceLabel({phase:'released'}),'');
});
