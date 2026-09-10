import {test} from 'node:test';
import assert from 'node:assert/strict';
import {scoped,severity,evaluate,append,acknowledge,seed} from '../dist/model.mjs';
test('R01/R02 產線統計範圍一致',()=>{const ds=seed();assert.equal(scoped(ds,'all').length,6);assert.equal(scoped(ds,'A').length,3);assert.equal(scoped(ds,'B').filter(d=>d.running).length,2)});
test('R05 閾值邊界',()=>{assert.equal(severity(85),null);assert.equal(severity(85.1),'警告');assert.equal(severity(95),'警告');assert.equal(severity(95.1),'嚴重')});
test('R05/R06 去重、確認不清除異常、恢復後再觸發',()=>{const d=seed()[0],as=[];d.samples.at(-1).temperature=90;evaluate(d,as,1);evaluate(d,as,2);assert.equal(as.length,1);acknowledge(as,as[0].id);assert.equal(as[0].acknowledged,true);assert.equal(d.active,'警告');evaluate(d,as,3);assert.equal(as.length,1);d.samples.at(-1).temperature=80;evaluate(d,as,4);d.samples.at(-1).temperature=90;evaluate(d,as,5);assert.equal(as.length,2);d.samples.at(-1).temperature=96;evaluate(d,as,6);assert.equal(as[0].severity,'嚴重')});
test('R04 樣本限制',()=>{const d=seed()[0];append(d,{time:999,temperature:70,pressure:3,vibration:1});assert.equal(d.samples.length,150);assert.equal(d.samples.at(-1).time,999)});
test('R05 停機不觸發告警',()=>{const d=seed()[4],as=[];d.samples.at(-1).temperature=100;evaluate(d,as,1);assert.equal(as.length,0)});
