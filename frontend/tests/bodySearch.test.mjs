import test from 'node:test';
import assert from 'node:assert/strict';
import { searchBody } from '../src/bodySearch.ts';
test('literal search preserves offsets, Unicode and line numbers', () => {
 const text = 'İ Foo [a]\n中文 foo\n[a] Foo';
 assert.deepEqual(searchBody(text, 'foo'), [ {start:2,end:5,line:0}, {start:13,end:16,line:1}, {start:21,end:24,line:2} ]);
 assert.equal(searchBody(text, '[a]').length, 2);
 assert.equal(searchBody(text, '').length, 0);
 assert.deepEqual(searchBody(text, '中文 foo\n[a]'), [{start:10,end:20,line:1}]);
});
test('search finds content after 5000 lines and 1 MiB', () => {
 const text = ('x'.repeat(240) + '\n').repeat(6000) + 'LAST-HTML-TAG';
 assert.deepEqual(searchBody(text, 'last-html-tag'), [{start:1446000,end:1446013,line:6000}]);
});
