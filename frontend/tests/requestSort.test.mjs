import test from 'node:test';
import assert from 'node:assert/strict';
import { sortRequests, newestRequestIndex } from '../src/requestSort.ts';
const type = mime => mime;
const a={id:'a',method:'POST',path:'/z',host:'z.test',contentType:'JSON',url:'https://z.test/z',status:500,responseSize:1000,started:100,duration:200};
const b={id:'b',method:'GET',path:'/a',host:'a.test',contentType:'HTML',url:'https://a.test/a',status:200,responseSize:10,started:200,duration:5};
test('each request column supports ascending and descending without modifying captured records',()=>{
 for(const key of ['status','method','name','type','size','duration','waterfall']) {
  const input=[a,b];
  assert.deepEqual(sortRequests(input,{key,direction:'asc'},type).map(f=>f.id),['b','a'],key);
  assert.deepEqual(sortRequests(input,{key,direction:'desc'},type).map(f=>f.id),['a','b'],key);
  assert.deepEqual(input,[a,b]);
 }
 assert.deepEqual(sortRequests([a,b],{key:'age',direction:'desc'},type).map(f=>f.id),['b','a']);
 assert.deepEqual(sortRequests([a,b],{key:'age',direction:'asc'},type).map(f=>f.id),['a','b']);
});
test('sizes and durations sort numerically, name uses displayed path then host, ties remain deterministic',()=>{
 const x={...a,id:'x',path:'/same?z=1',host:'b.test'};
 const y={...x,id:'y',path:'/same?a=2',host:'a.test'};
 assert.equal(sortRequests([x,y],{key:'name',direction:'asc'},type)[0].id,'y');
 const pending={...a,id:'pending',status:0};const failed={...a,id:'err',error:'connection failed'};
 assert.deepEqual(sortRequests([a,b,pending,failed],{key:'status',direction:'asc'},type).map(f=>f.id),['err','pending','b','a']);
 assert.equal(newestRequestIndex(sortRequests([a,b],{key:'size',direction:'asc'},type)),0);
 assert.equal(newestRequestIndex(sortRequests([a,b],{key:'size',direction:'desc'},type)),1);
 assert.equal(newestRequestIndex([]),-1);
});
