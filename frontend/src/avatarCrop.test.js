import { test } from 'node:test';
import assert from 'node:assert/strict';
import { cropRect, zoomCropPosition } from './avatarCrop.js';

test('crop stays square and moves within the image', () => {
  assert.deepEqual(cropRect(800, 400, 1, 50, 50), { side: 400, left: 200, top: 0 });
  assert.deepEqual(cropRect(800, 400, 2, 100, 0), { side: 200, left: 600, top: 0 });
});

test('zoom keeps the image point at the viewport center when starting at an edge', () => {
  const position = zoomCropPosition(800, 400, 1, 0, 0, 2);
  const zoomed = cropRect(800, 400, 2, position.x, position.y);
  assert.equal(zoomed.side, 200);
  assert.ok(Math.abs(zoomed.left - 100) < 1e-9);
  assert.ok(Math.abs(zoomed.top - 100) < 1e-9);
  assert.ok(Math.abs(zoomed.left + zoomed.side / 2 - 200) < 1e-9);
  assert.ok(Math.abs(zoomed.top + zoomed.side / 2 - 200) < 1e-9);
});
