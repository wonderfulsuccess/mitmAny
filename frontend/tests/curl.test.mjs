import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { requestCurl } from '../src/curl.ts';
const run = promisify(exec);
// Exported commands target POSIX shells; Git Bash provides one on Windows.
const shell = process.env.POSIX_SHELL ?? (process.platform === 'win32'
  ? join(process.env.ProgramFiles ?? 'C:\\Program Files', 'Git', 'bin', 'bash.exe')
  : '/bin/sh');
const detail = overrides => ({ method:'POST', url:'https://example.test/path', host:'example.test', requestHeaders:{}, requestBody:'', requestEncoding:'base64', requestCaptured:0, requestSize:0, ...overrides });

test('generated shell command replays binary, multipart boundary, repeated headers, cookies and shell-sensitive URL', async () => {
  const body = Buffer.from([0,255,13,10,39,96,36,10,128]);
  let received;
  const server = http.createServer(async (req,res) => {
    const chunks=[]; for await (const chunk of req) chunks.push(chunk);
    received={method:req.method,url:req.url,headers:req.headers,body:Buffer.concat(chunks),rawHeaders:req.rawHeaders};
    res.end('ok');
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  try {
    const path = "/a/../b?ids[]=1&value='$(echo%20BAD)`x`";
    const command = requestCurl(detail({
      method:'PATCH',url:`http://127.0.0.1:${server.address().port}${path}`,host:'virtual.test',
      requestBody:body.toString('base64'),requestSize:body.length,requestCaptured:body.length,
      requestHeaders:{'Content-Type':['multipart/form-data; boundary=example'],Cookie:["session='$(echo BAD)"],Authorization:['Bearer token'], 'X-Repeat':['first','second'],Connection:['keep-alive, X-Hop'], 'X-Hop':['drop-me'],'Content-Length':['999']},
    }));
    await run(command, {shell});
    assert.equal(received.method,'PATCH'); assert.equal(received.url,path);
    assert.deepEqual(received.body,body); assert.equal(received.headers.cookie,"session='$(echo BAD)");
    assert.equal(received.headers.authorization,'Bearer token'); assert.equal(received.headers.host,'virtual.test');
    assert.equal(received.headers['content-type'],'multipart/form-data; boundary=example');
    assert.equal(received.headers['content-length'],String(body.length)); assert.equal(received.headers['x-hop'],undefined);
    assert.equal(received.rawHeaders.filter(v=>v==='X-Repeat').length,2);
  } finally { await new Promise(resolve=>server.close(resolve)); }
});

test('decoded bodies drop stale content encoding, compressed bytes retain it', () => {
  const source=detail({requestBody:'e30=',requestSize:2,requestCaptured:2,requestHeaders:{'Content-Encoding':['gzip']}});
  assert.ok(!requestCurl(source).includes('Content-Encoding'));
  assert.ok(requestCurl({...source,requestEncoding:'compressed'}).includes('Content-Encoding: gzip'));
});

test('missing request bytes and CONNECT cannot silently export an incomplete replay', () => {
  assert.throws(()=>requestCurl(detail({requestSize:20,requestCaptured:5})), /not captured completely/);
  assert.throws(()=>requestCurl(detail({method:'CONNECT'})), /tunnel/);
  // An incomplete response does not stop replay of a fully captured request.
  assert.ok(requestCurl(detail({truncated:true,requestBody:'e30=',requestSize:2,requestCaptured:2})).includes('--data-binary @-'));
});
