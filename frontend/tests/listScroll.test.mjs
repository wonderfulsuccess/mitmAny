import test from 'node:test';
import assert from 'node:assert/strict';
import { listAnchor, anchorOffset, REQUEST_ROW_HEIGHT as h } from '../src/listScroll.ts';
const rows = ['a','b','c','d'].map(id=>({id}));
test('pinned viewport keeps the same row and pixel offset when newer rows are prepended',()=>{
 const anchor=listAnchor(rows,h+7);
 assert.deepEqual(anchor,{id:'b',offset:7});
 assert.equal(anchorOffset([{id:'new'},...rows],anchor),2*h+7);
 assert.equal(anchorOffset([...rows,{id:'new'}],anchor),h+7);
});
test('retiring old rows at the capture limit does not shift a surviving reading anchor',()=>{
 const anchor=listAnchor(rows,2*h+4);
 assert.equal(anchorOffset([...rows.slice(1),{id:'new'}],anchor),h+4);
 assert.equal(anchorOffset([{id:'x'}],anchor),null);
 assert.equal(listAnchor([],0),null);
});
