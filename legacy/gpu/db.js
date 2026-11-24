'use strict';
/**
 * db.js —— 显卡数据库的数据层（Node 内置 node:sqlite，零依赖）
 *
 * 表结构：
 *   gpus          显卡主表
 *   cooling       散热/功耗（1:1）
 *   mark3d        3DMark 跑分（1:1）
 *   blender       Blender 渲染（1:1）
 *   productivity  生产力测试（1:N）
 *   games         游戏测试（1:N）
 *   codec_runs    编解码测试（1:N，kind 区分 encoding / decoding）
 */
const path = require('node:path');
const fs = require('node:fs');
const { DatabaseSync } = require('node:sqlite');

const DATA_DIR = '/app/data';
const DB_PATH = path.join(DATA_DIR, 'gpu.db');
fs.mkdirSync(DATA_DIR, { recursive: true });

const db = new DatabaseSync(DB_PATH);
db.exec('PRAGMA foreign_keys = ON');
db.exec('PRAGMA journal_mode = WAL');

const version = db.prepare('PRAGMA user_version').get().user_version;
if (version > 1) throw new Error(`Unsupported database version: ${version}`);
if (version === 0) {
db.exec('BEGIN IMMEDIATE');
try {
// ---------------------------------------------------------------- 建表
db.exec(`
CREATE TABLE IF NOT EXISTS gpus (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  model         TEXT NOT NULL DEFAULT '',
  brand         TEXT NOT NULL DEFAULT '',
  vram_size     TEXT NOT NULL DEFAULT '',
  vram_type     TEXT NOT NULL DEFAULT '',
  vram_clock    TEXT NOT NULL DEFAULT '',
  gpu_clock     TEXT NOT NULL DEFAULT '',
  tgp           TEXT NOT NULL DEFAULT '',
  actual_power  TEXT NOT NULL DEFAULT '',
  driver_version TEXT NOT NULL DEFAULT '',
  test_date     TEXT NOT NULL DEFAULT '',
  test_platform TEXT NOT NULL DEFAULT '',
  ambient_temp  TEXT NOT NULL DEFAULT '',
  note          TEXT NOT NULL DEFAULT '',
  sort_order    INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT NOT NULL DEFAULT '',
  updated_at    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS cooling (
  gpu_id          INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  default_fan_temp TEXT NOT NULL DEFAULT '',
  max_fan_temp     TEXT NOT NULL DEFAULT '',
  temp_45db        TEXT NOT NULL DEFAULT '',
  core_temp        TEXT NOT NULL DEFAULT '',
  hotspot_temp     TEXT NOT NULL DEFAULT '',
  vram_temp        TEXT NOT NULL DEFAULT '',
  default_fan_rpm  TEXT NOT NULL DEFAULT '',
  max_fan_rpm      TEXT NOT NULL DEFAULT '',
  rpm_45db         TEXT NOT NULL DEFAULT '',
  full_load_power  TEXT NOT NULL DEFAULT '',
  power_45db       TEXT NOT NULL DEFAULT '',
  temp_limit       TEXT NOT NULL DEFAULT '',
  power_limit      TEXT NOT NULL DEFAULT '',
  throttling       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mark3d (
  gpu_id           INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  solar_bay        TEXT NOT NULL DEFAULT '',
  steel_nomad      TEXT NOT NULL DEFAULT '',
  steel_nomad_light TEXT NOT NULL DEFAULT '',
  speed_way        TEXT NOT NULL DEFAULT '',
  port_royal       TEXT NOT NULL DEFAULT '',
  time_spy         TEXT NOT NULL DEFAULT '',
  fire_strike      TEXT NOT NULL DEFAULT '',
  dx_raytracing    TEXT NOT NULL DEFAULT '',
  pcie_bandwidth   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS blender (
  gpu_id    INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  monster   TEXT NOT NULL DEFAULT '',
  junkshop  TEXT NOT NULL DEFAULT '',
  classroom TEXT NOT NULL DEFAULT ''
);

-- SolidWorks（SW）基准的 5 项细分，和 blender 一样是 1:1 的表：
--   图形 / 处理器 / 输入输出 / RealView 性能 / 模拟
CREATE TABLE IF NOT EXISTS sw (
  gpu_id     INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  graphics   TEXT NOT NULL DEFAULT '',
  processor  TEXT NOT NULL DEFAULT '',
  io         TEXT NOT NULL DEFAULT '',
  realview   TEXT NOT NULL DEFAULT '',
  simulation TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS productivity (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  gpu_id     INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  name       TEXT NOT NULL DEFAULT '',
  score      TEXT NOT NULL DEFAULT '',
  unit       TEXT NOT NULL DEFAULT '',
  note       TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS games (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  gpu_id           INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  game             TEXT NOT NULL DEFAULT '',
  resolution       TEXT NOT NULL DEFAULT '',
  quality          TEXT NOT NULL DEFAULT '',
  render_mode      TEXT NOT NULL DEFAULT '',
  upscale_tech     TEXT NOT NULL DEFAULT '',
  upscale_quality  TEXT NOT NULL DEFAULT '',
  frame_gen        TEXT NOT NULL DEFAULT '',
  avg_fps          TEXT NOT NULL DEFAULT '',
  min_fps          TEXT NOT NULL DEFAULT '',
  one_percent_low  TEXT NOT NULL DEFAULT '',
  ray_tracing      TEXT NOT NULL DEFAULT '',
  gpu_temp         TEXT NOT NULL DEFAULT '',
  gpu_usage        TEXT NOT NULL DEFAULT '',
  cpu_usage        TEXT NOT NULL DEFAULT '',
  ram_usage        TEXT NOT NULL DEFAULT '',
  vram_usage       TEXT NOT NULL DEFAULT '',
  power            TEXT NOT NULL DEFAULT '',
  note             TEXT NOT NULL DEFAULT '',
  sort_order       INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS codec_runs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  gpu_id     INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL,
  codec      TEXT NOT NULL DEFAULT '',
  resolution TEXT NOT NULL DEFAULT '',
  time_sec   TEXT NOT NULL DEFAULT '',
  power      TEXT NOT NULL DEFAULT '',
  gpu_usage  TEXT NOT NULL DEFAULT '',
  cpu_usage  TEXT NOT NULL DEFAULT '',
  note       TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_productivity_gpu ON productivity(gpu_id);
CREATE INDEX IF NOT EXISTS idx_games_gpu        ON games(gpu_id);
CREATE INDEX IF NOT EXISTS idx_codec_gpu        ON codec_runs(gpu_id, kind);

-- 通用品类表：CPU / 设备 等新品类走这里。
-- fields / rows 用 JSON 存，字段布局由前端 schema.js 定义，
-- 加减字段不需要动数据库结构。
CREATE TABLE IF NOT EXISTS items (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  kind       TEXT NOT NULL,
  name       TEXT NOT NULL DEFAULT '',
  brand      TEXT NOT NULL DEFAULT '',
  subtitle   TEXT NOT NULL DEFAULT '',
  fields     TEXT NOT NULL DEFAULT '{}',
  rows       TEXT NOT NULL DEFAULT '{}',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_items_kind ON items(kind, sort_order, id);

-- 显卡自定义字段：编辑「基本参数 / 散热与功耗 / 3DMark」时，
--   · hidden = 1 的记录表示用户在界面上删掉的内置项目（存下来好记住不再显示，值已清空）
--   · hidden = 0 的记录是用户手动新增的项目（label + value 一起存在这里）
-- 这样加减项目不用改数据库结构，也不影响原有固定列。
CREATE TABLE IF NOT EXISTS gpu_custom (
  gpu_id     INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  section    TEXT NOT NULL,
  field_key  TEXT NOT NULL,
  label      TEXT NOT NULL DEFAULT '',
  value      TEXT NOT NULL DEFAULT '',
  hidden     INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (gpu_id, section, field_key)
);
CREATE INDEX IF NOT EXISTS idx_gpu_custom ON gpu_custom(gpu_id, section, sort_order);

-- 下拉候选值：目前用于游戏名。存的是「库里还没有对应游戏记录、但用户想先加进来」的名字；
-- 实际下拉 = 这张表 + games 表里已有的游戏（去重，见 listGameOptions）。
CREATE TABLE IF NOT EXISTS option_values (
  kind       TEXT NOT NULL,
  value      TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (kind, value)
);
`);

// ---------------------------------------------------------------- 轻量迁移
// CREATE TABLE IF NOT EXISTS 不会给老库补列，这里按清单逐个补，已存在则跳过。
//   min_fps     赛博朋克2077 专用的「最低帧数」（低帧写小数点的那种）
//   ray_tracing 光线追踪档位（关/低/中/高/超级/变态/超速/路径追踪）
{
  const want = [
    ['min_fps', "TEXT NOT NULL DEFAULT ''"],
    ['ray_tracing', "TEXT NOT NULL DEFAULT ''"],
    ['gpu_temp', "TEXT NOT NULL DEFAULT ''"],
    ['gpu_usage', "TEXT NOT NULL DEFAULT ''"],
    ['cpu_usage', "TEXT NOT NULL DEFAULT ''"],
    ['ram_usage', "TEXT NOT NULL DEFAULT ''"],
  ];
  const have = db.prepare('PRAGMA table_info(games)').all().map(c => c.name);
  for (const [name, decl] of want) {
    if (!have.includes(name)) {
      db.exec(`ALTER TABLE games ADD COLUMN ${name} ${decl}`);
      console.log(`[db] 已为 games 表补上 ${name} 列`);
    }
  }
}
  const gpuColumns = db.prepare('PRAGMA table_info(gpus)').all().map(c => c.name);
  if (!gpuColumns.includes('related_media')) {
    db.exec("ALTER TABLE gpus ADD COLUMN related_media TEXT NOT NULL DEFAULT '[]'");
  }
  db.exec('PRAGMA user_version = 1');
  db.exec('COMMIT');
} catch (error) {
  db.exec('ROLLBACK');
  throw error;
}
}

// ---------------------------------------------------------------- 字段定义
const GPUS_COLS = ['model', 'brand', 'vram_size', 'vram_type', 'vram_clock', 'gpu_clock',
  'tgp', 'actual_power', 'driver_version', 'test_date', 'test_platform',
  'ambient_temp', 'note', 'sort_order'];
const COOLING_COLS = ['default_fan_temp', 'max_fan_temp', 'temp_45db', 'core_temp',
  'hotspot_temp', 'vram_temp', 'default_fan_rpm', 'max_fan_rpm', 'rpm_45db',
  'full_load_power', 'power_45db', 'temp_limit', 'power_limit', 'throttling'];
const MARK3D_COLS = ['solar_bay', 'steel_nomad', 'steel_nomad_light', 'speed_way',
  'port_royal', 'time_spy', 'fire_strike', 'dx_raytracing', 'pcie_bandwidth'];
const BLENDER_COLS = ['monster', 'junkshop', 'classroom'];
const SW_COLS = ['graphics', 'processor', 'io', 'realview', 'simulation'];
const PROD_COLS = ['name', 'score', 'unit', 'note'];
const GAME_COLS = ['game', 'resolution', 'quality', 'ray_tracing', 'render_mode',
  'upscale_tech', 'upscale_quality', 'frame_gen', 'avg_fps', 'min_fps',
  'one_percent_low', 'gpu_temp', 'gpu_usage', 'cpu_usage', 'ram_usage',
  'vram_usage', 'power', 'note'];
const CODEC_COLS = ['codec', 'resolution', 'time_sec', 'power', 'gpu_usage',
  'cpu_usage', 'note'];

const now = () => new Date().toISOString();

function pick(obj, cols) {
  const out = {};
  for (const c of cols) {
    const v = obj?.[c];
    out[c] = (v === null || v === undefined) ? '' : String(v);
  }
  return out;
}

function insertRow(table, data, cols) {
  const keys = Object.keys(data);
  const sql = `INSERT INTO ${table} (${keys.join(',')}) VALUES (${keys.map(() => '?').join(',')})`;
  return db.prepare(sql).run(...keys.map(k => data[k]));
}

// ---------------------------------------------------------------- 自定义字段
// 三个可编辑区块：基本参数 / 散热与功耗 / 3DMark
const CUSTOM_SECTIONS = ['basic', 'cooling', 'mark3d'];

function emptyCustom() {
  const out = {};
  for (const s of CUSTOM_SECTIONS) out[s] = { hidden: [], extra: [] };
  return out;
}

function readCustom(gpuId) {
  const out = emptyCustom();
  const rows = db.prepare(
    'SELECT section, field_key, label, value, hidden FROM gpu_custom WHERE gpu_id = ? ORDER BY section, sort_order, rowid'
  ).all(gpuId);
  for (const r of rows) {
    const sec = out[r.section];
    if (!sec) continue;
    if (r.hidden) sec.hidden.push(r.field_key);
    else sec.extra.push({ key: r.field_key, label: r.label || '', value: r.value || '' });
  }
  return out;
}

function writeCustom(gpuId, custom) {
  db.prepare('DELETE FROM gpu_custom WHERE gpu_id = ?').run(gpuId);
  if (!custom) return;
  const ins = db.prepare(
    'INSERT INTO gpu_custom (gpu_id, section, field_key, label, value, hidden, sort_order) VALUES (?,?,?,?,?,?,?)'
  );
  for (const s of CUSTOM_SECTIONS) {
    const c = custom[s] || {};
    const hidden = Array.isArray(c.hidden) ? c.hidden : [];
    const extra = Array.isArray(c.extra) ? c.extra : [];
    let i = 0;
    for (const k of hidden) {
      if (!k) continue;
      ins.run(gpuId, s, String(k), '', '', 1, i++);
    }
    for (const e of extra) {
      if (!e || !e.key) continue;
      ins.run(gpuId, s, String(e.key), String(e.label || ''), String(e.value ?? ''), 0, i++);
    }
  }
}

// ---------------------------------------------------------------- 下拉候选（游戏名）
function listGameOptions() {
  return db.prepare(`
    SELECT value FROM (
      SELECT value FROM option_values WHERE kind = 'game'
      UNION
      SELECT game AS value FROM games WHERE game IS NOT NULL AND game <> ''
    ) ORDER BY value COLLATE NOCASE
  `).all().map(r => r.value);
}

function addGameOption(value) {
  const v = String(value ?? '').trim();
  if (!v) return false;
  db.prepare('INSERT OR IGNORE INTO option_values (kind, value) VALUES (?, ?)').run('game', v);
  return true;
}

// ---------------------------------------------------------------- 读
function listGpus() {
  const rows = db.prepare(`
    SELECT g.*,
      (SELECT COUNT(*) FROM games        WHERE gpu_id = g.id) AS game_count,
      (SELECT COUNT(*) FROM productivity WHERE gpu_id = g.id) AS prod_count,
      (SELECT COUNT(*) FROM codec_runs   WHERE gpu_id = g.id AND kind='encoding') AS enc_count,
      (SELECT COUNT(*) FROM codec_runs   WHERE gpu_id = g.id AND kind='decoding') AS dec_count,
      (SELECT steel_nomad_light FROM mark3d WHERE gpu_id = g.id) AS mark3d_light,
      (SELECT time_spy         FROM mark3d WHERE gpu_id = g.id) AS time_spy,
      (SELECT solar_bay        FROM mark3d WHERE gpu_id = g.id) AS solar_bay
    FROM gpus g
    ORDER BY g.sort_order, g.id
  `).all();
  return rows;
}

function one(table, gpuId) {
  return db.prepare(`SELECT * FROM ${table} WHERE gpu_id = ?`).get(gpuId) || null;
}

function many(table, gpuId, extraWhere = '', order = 'sort_order, id') {
  return db.prepare(`SELECT * FROM ${table} WHERE gpu_id = ? ${extraWhere} ORDER BY ${order}`).all(gpuId);
}

function getGpu(id) {
  const gpu = db.prepare('SELECT * FROM gpus WHERE id = ?').get(id);
  if (!gpu) return null;
  return {
    gpu,
    cooling: one('cooling', id),
    mark3d: one('mark3d', id),
    blender: one('blender', id),
    sw: one('sw', id),
    productivity: many('productivity', id),
    games: many('games', id),
    encoding: many('codec_runs', id, "AND kind='encoding'"),
    decoding: many('codec_runs', id, "AND kind='decoding'"),
    custom: readCustom(id),
  };
}

// ---------------------------------------------------------------- 写
function insertGpu(payload) {
  const t = now();
  const g = pick(payload.gpu || {}, GPUS_COLS);
  g.created_at = t;
  g.updated_at = t;
  const r = insertRow('gpus', g, GPUS_COLS);
  const id = Number(r.lastInsertRowid);
  writeChildren(id, payload);
  writeCustom(id, payload.custom);
  return id;
}

function updateGpu(id, payload) {
  const g = pick(payload.gpu || {}, GPUS_COLS);
  g.updated_at = now();
  const keys = Object.keys(g);
  db.prepare(`UPDATE gpus SET ${keys.map(k => k + '=?').join(',')} WHERE id=?`)
    .run(...keys.map(k => g[k]), id);
  for (const t of ['cooling', 'mark3d', 'blender', 'sw', 'productivity', 'games', 'codec_runs', 'gpu_custom']) {
    db.prepare(`DELETE FROM ${t} WHERE gpu_id = ?`).run(id);
  }
  writeChildren(id, payload);
  writeCustom(id, payload.custom);
}

function writeChildren(id, p) {
  if (p.cooling) {
    const d = pick(p.cooling, COOLING_COLS); d.gpu_id = id;
    insertRow('cooling', d, COOLING_COLS);
  }
  if (p.mark3d) {
    const d = pick(p.mark3d, MARK3D_COLS); d.gpu_id = id;
    insertRow('mark3d', d, MARK3D_COLS);
  }
  if (p.blender) {
    const d = pick(p.blender, BLENDER_COLS); d.gpu_id = id;
    insertRow('blender', d, BLENDER_COLS);
  }
  if (p.sw) {
    const d = pick(p.sw, SW_COLS); d.gpu_id = id;
    insertRow('sw', d, SW_COLS);
  }
  (p.productivity || []).forEach((row, i) => {
    const d = pick(row, PROD_COLS); d.gpu_id = id; d.sort_order = i;
    insertRow('productivity', d, PROD_COLS);
  });
  (p.games || []).forEach((row, i) => {
    const d = pick(row, GAME_COLS); d.gpu_id = id; d.sort_order = i;
    insertRow('games', d, GAME_COLS);
  });
  for (const kind of ['encoding', 'decoding']) {
    (p[kind] || []).forEach((row, i) => {
      const d = pick(row, CODEC_COLS); d.gpu_id = id; d.kind = kind; d.sort_order = i;
      insertRow('codec_runs', d, CODEC_COLS.concat(['kind']));
    });
  }
}

function deleteGpu(id) {
  db.prepare('DELETE FROM gpus WHERE id = ?').run(id);
}

function copyGpu(id) {
  const src = getGpu(id);
  if (!src) return null;
  const g = { ...src.gpu };
  delete g.id;
  g.model = (g.model || '未命名显卡') + ' 副本';
  g.sort_order = (g.sort_order || 0) + 1;
  const r = insertRow('gpus', g, Object.keys(g));
  const nid = Number(r.lastInsertRowid);
  for (const t of ['cooling', 'mark3d', 'blender', 'sw']) {
    const row = src[t];
    if (!row) continue;
    const d = { ...row, gpu_id: nid };
    delete d.id;
    insertRow(t, d, Object.keys(d));
  }
  for (const [t, key] of [['productivity', 'productivity'], ['games', 'games'],
    ['codec_runs', 'encoding'], ['codec_runs', 'decoding']]) {
    for (const row of src[key] || []) {
      const d = { ...row, gpu_id: nid };
      delete d.id;
      insertRow(t, d, Object.keys(d));
    }
  }
  for (const row of db.prepare('SELECT * FROM gpu_custom WHERE gpu_id = ?').all(id)) {
    const d = { ...row, gpu_id: nid };
    insertRow('gpu_custom', d, Object.keys(d));
  }
  return nid;
}

function reorder(ids) {
  const st = db.prepare('UPDATE gpus SET sort_order = ? WHERE id = ?');
  ids.forEach((id, i) => st.run(i, id));
}

// ---------------------------------------------------------------- 通用品类（CPU / 设备 …）
function rowToItem(r) {
  if (!r) return null;
  let fields = {}, rows = {};
  try { fields = JSON.parse(r.fields || '{}'); } catch { fields = {}; }
  try { rows = JSON.parse(r.rows || '{}'); } catch { rows = {}; }
  return { ...r, fields, rows };
}

function listItems(kind) {
  const sql = kind
    ? 'SELECT id, kind, name, brand, subtitle, fields, rows, sort_order, created_at, updated_at FROM items WHERE kind = ? ORDER BY sort_order, id'
    : 'SELECT id, kind, name, brand, subtitle, fields, rows, sort_order, created_at, updated_at FROM items ORDER BY kind, sort_order, id';
  const rs = kind ? db.prepare(sql).all(kind) : db.prepare(sql).all();
  return rs.map(rowToItem);
}

function getItem(id) {
  return rowToItem(db.prepare('SELECT * FROM items WHERE id = ?').get(id));
}

function insertItem(p) {
  const t = now();
  const r = db.prepare(`INSERT INTO items (kind,name,brand,subtitle,fields,rows,sort_order,created_at,updated_at)
    VALUES (?,?,?,?,?,?,?,?,?)`).run(
    String(p.kind || ''), String(p.name || ''), String(p.brand || ''), String(p.subtitle || ''),
    JSON.stringify(p.fields || {}), JSON.stringify(p.rows || {}),
    Number(p.sort_order || 0), t, t);
  return Number(r.lastInsertRowid);
}

function updateItem(id, p) {
  const cur = getItem(id);
  if (!cur) return false;
  db.prepare(`UPDATE items SET kind=?,name=?,brand=?,subtitle=?,fields=?,rows=?,sort_order=?,updated_at=?
    WHERE id=?`).run(
    String(p.kind ?? cur.kind), String(p.name ?? cur.name), String(p.brand ?? cur.brand),
    String(p.subtitle ?? cur.subtitle),
    JSON.stringify(p.fields ?? cur.fields), JSON.stringify(p.rows ?? cur.rows),
    Number(p.sort_order ?? cur.sort_order), now(), id);
  return true;
}

function copyItem(id) {
  const it = getItem(id);
  if (!it) return null;
  return insertItem({
    kind: it.kind, name: (it.name || '未命名') + ' 副本', brand: it.brand,
    subtitle: it.subtitle, fields: it.fields, rows: it.rows,
    sort_order: (it.sort_order || 0) + 1,
  });
}

function deleteItem(id) {
  db.prepare('DELETE FROM items WHERE id = ?').run(id);
}

function stats() {
  const q = s => db.prepare(s).get();
  const kinds = db.prepare('SELECT kind, COUNT(*) n FROM items GROUP BY kind').all();
  const byKind = {};
  for (const k of kinds) byKind[k.kind] = k.n;
  return {
    gpus: q('SELECT COUNT(*) n FROM gpus').n,
    games: q('SELECT COUNT(*) n FROM games').n,
    productivity: q('SELECT COUNT(*) n FROM productivity').n,
    encoding: q("SELECT COUNT(*) n FROM codec_runs WHERE kind='encoding'").n,
    decoding: q("SELECT COUNT(*) n FROM codec_runs WHERE kind='decoding'").n,
    withMark3d: q('SELECT COUNT(*) n FROM mark3d').n,
    itemsByKind: byKind,
  };
}


module.exports = {
  db, DB_PATH,
  listGpus, getGpu, insertGpu, updateGpu, deleteGpu, copyGpu, reorder, stats,
  listItems, getItem, insertItem, updateItem, copyItem, deleteItem,
  listGameOptions, addGameOption,
};
