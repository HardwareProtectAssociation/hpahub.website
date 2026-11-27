const difficultyRanks = { SSS: 6, S: 5, A: 4, B: 3, C: 2, D: 1 };

export function difficultyRank(value) {
  return difficultyRanks[String(value || '').trim().toUpperCase()] || 1;
}

export function levelRank(level) {
  if (level >= 100) return 5;
  if (level >= 80) return 4;
  if (level >= 60) return 3;
  if (level >= 40) return 2;
  return 1;
}
