import test from 'node:test';
import assert from 'node:assert/strict';
import { relativeRequestTime } from '../src/relativeTime.ts';
test('request age uses compact S/M/H units at minute and hour boundaries',()=>{
 const started=100000;
 for(const [elapsed,want] of [[0,'0S'],[1000,'1S'],[59999,'59S'],[60000,'1M'],[3599999,'59M'],[3600000,'1H'],[86400000,'24H']])
   assert.equal(relativeRequestTime(started,started+elapsed),want);
 assert.equal(relativeRequestTime(started,started-2000),'0S');
 assert.equal(relativeRequestTime(0,started),'—');
});
