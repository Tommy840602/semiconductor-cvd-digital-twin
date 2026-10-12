import {test} from 'node:test';
import assert from 'node:assert/strict';
import {utilizationText} from './utilization.js';
test('unknown evidence is not a numeric utilization',()=>{
 assert.equal(utilizationText({quality:'partial',utilization:0},'utilization'),'資料不完整');
 assert.equal(utilizationText({quality:'no-data',utilization:null},'utilization'),'無資料');
 assert.equal(utilizationText({quality:'complete',utilization:0},'utilization'),'0.0%');
 assert.equal(utilizationText({quality:'complete',pmUtilization:25},'pmUtilization'),'25.0%');
});
