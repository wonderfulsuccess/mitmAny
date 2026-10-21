import test from 'node:test';
import assert from 'node:assert/strict';
import { groupDomains, searchDomainGroups } from '../src/domainGroups.ts';
const flow = (host,domain,rootDomain,started=0) => ({host,domain,rootDomain,started,url:`https://${host}/`});
const flows = [flow('wx.qlogo.cn:443','wx.qlogo.cn','qlogo.cn'),flow('wx.qlogo.cn','wx.qlogo.cn','qlogo.cn'),flow('cdn.qlogo.cn','cdn.qlogo.cn','qlogo.cn'),flow('qlogo.cn','qlogo.cn','qlogo.cn'),flow('api.example.co.uk','api.example.co.uk','example.co.uk')];
test('main domains aggregate subdomains and normalize port duplicates',()=>{
 const groups=groupDomains(flows);
 assert.deepEqual(Object.fromEntries(groups.map(g=>[g.name,g.count])),{'qlogo.cn':4,'example.co.uk':1});
 const group=groups.find(g=>g.name==='qlogo.cn');
 assert.equal(group.children.find(c=>c.name==='wx.qlogo.cn').count,2);
 assert.equal(group.children.reduce((sum,c)=>sum+c.count,0),group.count);
});
test('search matches child domains without losing parent totals; parent search shows all children',()=>{
 const groups=groupDomains(flows);
 const filtered=searchDomainGroups(groups,' WX. ');
 assert.equal(filtered[0].name,'qlogo.cn');assert.equal(filtered[0].count,4);
 assert.deepEqual(filtered[0].children,[{name:'wx.qlogo.cn',count:2}]);
 assert.equal(searchDomainGroups(groups,'qlogo')[0].children.length,3);
 assert.deepEqual(searchDomainGroups(groups,'no-match'),[]);
 assert.equal(searchDomainGroups(groups,'').length,2);
});

test('recent requests promote both the parent group and the child regardless of counts or input order',()=>{
 const records=[
  flow('busy.old.test','busy.old.test','old.test',50),
  flow('recent.new.test','recent.new.test','new.test',300),
  flow('busy.old.test','busy.old.test','old.test',100),
  flow('quiet.old.test','quiet.old.test','old.test',200),
  flow('busy.old.test','busy.old.test','old.test',150),
 ];
 const groups=groupDomains(records);
 assert.deepEqual(groups.map(g=>g.name),['new.test','old.test']);
 assert.deepEqual(groups[1].children.map(c=>[c.name,c.count]),[['quiet.old.test',1],['busy.old.test',3]]);
 const updated=groupDomains([...records,flow('busy.old.test','busy.old.test','old.test',400)]);
 assert.deepEqual(updated.map(g=>g.name),['old.test','new.test']);
 assert.equal(updated[0].children[0].name,'busy.old.test');
 // Finishing an older request must not promote it over a newly started one.
 assert.deepEqual(groupDomains(records.map(f=>({...f,duration:9999,revision:500}))),groups);
});
test('flat mode lists all hostnames separately in recent-request order',()=>{
 const source=[flow('a.example.test','a.example.test','example.test',10),flow('b.example.test','b.example.test','example.test',20),flow('a.example.test:443','a.example.test','example.test',30)];
 const grouped=groupDomains(source);
 assert.equal(grouped.length,1);assert.equal(grouped[0].count,3);
 const flat=groupDomains(source,false);
 assert.deepEqual(flat.map(g=>[g.name,g.count]),[['a.example.test',2],['b.example.test',1]]);
 assert.ok(flat.every(group=>group.children.length===1 && group.children[0].name===group.name));
});
