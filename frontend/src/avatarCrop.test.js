import { test } from 'node:test';
import assert from 'node:assert/strict';
import { cropRect } from './avatarCrop.js';

test('crop stays square and moves within the image', () => {
  assert.deepEqual(cropRect(800, 400, 1, 50, 50), { side: 400, left: 200, top: 0 });
  assert.deepEqual(cropRect(800, 400, 2, 100, 0), { side: 200, left: 600, top: 0 });
});
