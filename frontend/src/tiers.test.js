import { test } from 'node:test';
import assert from 'node:assert/strict';
import { difficultyRank, levelRank } from './tiers.js';

test('difficulty letters map S..D to ranks 5..1', () => {
  assert.deepEqual(['S', 'A', 'B', 'C', 'D'].map(difficultyRank), [5, 4, 3, 2, 1]);
  assert.equal(difficultyRank('a'), 4);
  assert.equal(difficultyRank(''), 1);
});

test('level ranks follow the 20-level bands with 100 on top', () => {
  assert.deepEqual([39, 40, 59, 60, 79, 80, 99, 100].map(levelRank), [1, 2, 2, 3, 3, 4, 4, 5]);
});
