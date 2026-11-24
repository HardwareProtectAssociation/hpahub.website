'use strict';
const http = require('node:http');
const db = require('./db');

const PORT = process.env.PORT ? Number(process.env.PORT) : 13001;

// ---------------------------------------------------------------- 工具
function sendJson(res, code, obj) {
  const body = JSON.stringify(obj);
  res.writeHead(code, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(body),
    'Cache-Control': 'no-store',
  });
  res.end(body);
}

function readBody(req, limit = 8 * 1024 * 1024) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const chunks = [];
    req.on('data', c => {
      size += c.length;
      if (size > limit) { reject(new Error('请求体过大')); req.destroy(); return; }
      chunks.push(c);
    });
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8');
      if (!raw) { resolve({}); return; }
      try { resolve(JSON.parse(raw)); } catch (e) { reject(new Error('JSON 解析失败: ' + e.message)); }
    });
    req.on('error', reject);
  });
}

// ---------------------------------------------------------------- API
async function handleApi(req, res, url) {
  const seg = url.pathname.replace(/^\/api\/?/, '').split('/').filter(Boolean);
  const method = req.method.toUpperCase();

  // GET /api/stats
  if (seg[0] === 'stats' && method === 'GET') {
    return sendJson(res, 200, db.stats());
  }

  if (seg[0] === 'related-media') {
    const gpuId = parseInt(url.searchParams.get('gpu_id') || '0');
    if (method === 'GET') {
      const row = db.db.prepare('SELECT related_media FROM gpus WHERE id = ?').get(gpuId);
      const media = row ? JSON.parse(row.related_media || '[]') : [];
      return sendJson(res, 200, { ok: true, media });
    }
    if (method === 'POST') {
      const body = await readBody(req);
      const row = db.db.prepare('SELECT related_media FROM gpus WHERE id = ?').get(gpuId);
      let media = row ? JSON.parse(row.related_media || '[]') : [];
      if (body.action === 'add' && body.path && !media.includes(body.path)) {
        media.push(body.path);
      } else if (body.action === 'remove') {
        media = media.filter(p => p !== body.path);
      }
      db.db.prepare('UPDATE gpus SET related_media = ? WHERE id = ?').run(JSON.stringify(media), gpuId);
      return sendJson(res, 200, { ok: true, media });
    }
  }

  // ---------------- 下拉选项：游戏名 ----------------
  // GET  /api/options/games   -> ['CS2','2077', ...]
  // POST /api/options/games   body: { value }  新加一个游戏名
  if (seg[0] === 'options' && seg[1] === 'games' && seg.length === 2) {
    if (method === 'GET') return sendJson(res, 200, db.listGameOptions());
    if (method === 'POST') {
      const body = await readBody(req);
      db.addGameOption(body.value);
      return sendJson(res, 200, { ok: true });
    }
  }

  // ---------------- 通用品类（CPU / 设备 …） ----------------
  // GET /api/items?kind=cpu
  if (seg[0] === 'items' && seg.length === 1 && method === 'GET') {
    const kind = url.searchParams.get('kind') || '';
    return sendJson(res, 200, db.listItems(kind));
  }
  // POST /api/items
  if (seg[0] === 'items' && seg.length === 1 && method === 'POST') {
    const body = await readBody(req);
    if (!body.kind) return sendJson(res, 400, { error: '缺少 kind' });
    return sendJson(res, 201, { id: db.insertItem(body) });
  }
  // /api/items/:id
  if (seg[0] === 'items' && seg.length >= 2) {
    const iid = Number(seg[1]);
    if (!Number.isFinite(iid)) return sendJson(res, 400, { error: 'id 非法' });
    if (seg.length === 2 && method === 'GET') {
      const it = db.getItem(iid);
      return it ? sendJson(res, 200, it) : sendJson(res, 404, { error: '未找到' });
    }
    if (seg.length === 2 && method === 'PUT') {
      const body = await readBody(req);
      const ok = db.updateItem(iid, body);
      return ok ? sendJson(res, 200, { ok: true, id: iid }) : sendJson(res, 404, { error: '未找到' });
    }
    if (seg.length === 2 && method === 'DELETE') {
      db.deleteItem(iid);
      return sendJson(res, 200, { ok: true });
    }
    if (seg.length === 3 && seg[2] === 'copy' && method === 'POST') {
      const nid = db.copyItem(iid);
      return nid ? sendJson(res, 201, { id: nid }) : sendJson(res, 404, { error: '未找到' });
    }
  }

  // GET /api/gpus
  if (seg[0] === 'gpus' && seg.length === 1 && method === 'GET') {
    return sendJson(res, 200, db.listGpus());
  }

  // POST /api/gpus
  if (seg[0] === 'gpus' && seg.length === 1 && method === 'POST') {
    const body = await readBody(req);
    const id = db.insertGpu(body);
    return sendJson(res, 201, { id });
  }

  // POST /api/reorder   body: { ids: [..] }
  if (seg[0] === 'reorder' && method === 'POST') {
    const body = await readBody(req);
    if (!Array.isArray(body.ids)) return sendJson(res, 400, { error: '缺少 ids' });
    db.reorder(body.ids.map(Number));
    return sendJson(res, 200, { ok: true });
  }

  // /api/gpus/:id
  if (seg[0] === 'gpus' && seg.length >= 2) {
    const id = Number(seg[1]);
    if (!Number.isFinite(id)) return sendJson(res, 400, { error: 'id 非法' });

    if (seg.length === 2 && method === 'GET') {
      const g = db.getGpu(id);
      return g ? sendJson(res, 200, g) : sendJson(res, 404, { error: '未找到' });
    }
    if (seg.length === 2 && method === 'PUT') {
      const body = await readBody(req);
      if (!db.getGpu(id)) return sendJson(res, 404, { error: '未找到' });
      db.updateGpu(id, body);
      return sendJson(res, 200, { ok: true, id });
    }
    if (seg.length === 2 && method === 'DELETE') {
      db.deleteGpu(id);
      return sendJson(res, 200, { ok: true });
    }
    if (seg.length === 3 && seg[2] === 'copy' && method === 'POST') {
      const nid = db.copyGpu(id);
      return nid ? sendJson(res, 201, { id: nid }) : sendJson(res, 404, { error: '未找到' });
    }
  }

  return sendJson(res, 404, { error: '接口不存在: ' + url.pathname });
}


// ---------------------------------------------------------------- 启动
const server = http.createServer((req, res) => {
  const url = new URL(req.url || '/', 'http://localhost');
  if (url.pathname.startsWith('/api/')) {
    handleApi(req, res, url).catch(e => {
      console.error('[API 错误]', e);
      sendJson(res, 500, { error: String(e.message || e) });
    });
    return;
  }
  sendJson(res, 404, { error: 'Not found' });
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`[gpu-db] 监听 0.0.0.0:${PORT}`);
  console.log(`[gpu-db] 数据库: ${db.DB_PATH}`);
});

process.on('SIGINT', () => { server.close(() => process.exit(0)); });
process.on('SIGTERM', () => { server.close(() => process.exit(0)); });
