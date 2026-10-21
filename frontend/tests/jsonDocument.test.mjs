import test from 'node:test';
import assert from 'node:assert/strict';
import { jsonDocument, visibleJsonLines, httpURL } from '../src/jsonDocument.ts';

test('formatting retains integer precision, duplicate keys, exponents and escapes', () => {
  const text = '{"id":9223372036854775807,"same":1,"same":2,"value":1e400,"quoted":"a\\\"b"}';
  const document = jsonDocument(text);
  assert.ok(document);
  const formatted = document.lines.map(line => '  '.repeat(line.depth) + line.tokens.map(token => token.text).join('')).join('\n');
  assert.equal(formatted.replace(/\s/g, ''), text);
  assert.equal(document.lines[0].fold.count, 5);
});

test('folding hides only the chosen subtree and preserves adjacent fields', () => {
  const document = jsonDocument('{"base_resp":{"ret":0,"data":[1,{"ok":true}]},"after":null}');
  const base = document.lines.find(line => line.fold?.label === 'base_resp');
  assert.ok(base);
  const visible = visibleJsonLines(document, new Set([base.fold.id]));
  assert.ok(visible.some(line => line.tokens.some(token => token.text === '"after"')));
  assert.ok(!visible.some(line => line.tokens.some(token => token.text === '"ret"')));
  assert.equal(document.lines[base.fold.end].tokens.at(-1).text, ',');
  assert.equal(visibleJsonLines(document, new Set()).length, document.lines.length);
});

test('empty objects and arrays stay inline; nested array counts are correct', () => {
  const document = jsonDocument('{"empty":{},"array":[],"items":[{},[],null]}');
  assert.equal(document.lines.filter(line => line.fold).length, 2);
  assert.equal(document.lines.find(line => line.fold?.label === 'items').fold.count, 3);
});

test('links open only explicit HTTP(S) values, including JSON escaped slashes', () => {
  assert.equal(httpURL('javascript:alert(1)'), undefined);
  assert.equal(httpURL('file:///etc/passwd'), undefined);
  assert.equal(httpURL('https://example.com/\nscript'), undefined);
  assert.equal(httpURL('https://example.com/docs?a=1&b=2'), 'https://example.com/docs?a=1&b=2');
  const document = jsonDocument('{"url":"https:\\/\\/example.com\\/docs"}');
  assert.equal(document.lines.flatMap(line => line.tokens).find(token => token.href).href, 'https://example.com/docs');
});

test('incomplete, deeply nested or oversized JSON falls back without partial formatting', () => {
  assert.equal(jsonDocument('{"unfinished":'), null);
  assert.equal(jsonDocument('['.repeat(140) + '0' + ']'.repeat(140)), null);
  assert.equal(jsonDocument('[' + Array(51000).fill('0').join(',') + ']'), null);
});
