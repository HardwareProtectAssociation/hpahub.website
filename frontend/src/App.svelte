<script>
  import { onMount } from 'svelte';

  let token = sessionStorage.getItem('adminToken') || '';
  let identity = '';
  let password = '';
  let policy = { intervalHours: 0, retainCount: 10 };
  let backups = [];
  let loading = false;
  let saving = false;
  let creating = false;
  let downloading = '';
  let error = '';
  let notice = '';

  async function request(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: {
        ...(options.body ? { 'Content-Type': 'application/json' } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options.headers,
      },
    });
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      if (response.status === 401 && token) {
        token = '';
        sessionStorage.removeItem('adminToken');
      }
      throw new Error(data.message || `请求失败（${response.status}）`);
    }
    return response.status === 204 ? null : response.json();
  }

  async function refresh() {
    loading = true;
    error = '';
    try {
      const [nextPolicy, nextBackups] = await Promise.all([
        request('/api/backup-policy'), request('/api/backups'),
      ]);
      policy = nextPolicy;
      backups = nextBackups.sort((a, b) => new Date(b.modified) - new Date(a.modified));
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function login() {
    loading = true;
    error = '';
    try {
      const result = await request('/api/collections/_superusers/auth-with-password', {
        method: 'POST', body: JSON.stringify({ identity, password }),
      });
      token = result.token;
      sessionStorage.setItem('adminToken', token);
      password = '';
      await refresh();
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  function logout() {
    token = '';
    sessionStorage.removeItem('adminToken');
    backups = [];
    password = '';
    error = '';
    notice = '';
  }

  async function savePolicy() {
    saving = true;
    error = '';
    notice = '';
    try {
      policy = await request('/api/backup-policy', {
        method: 'PUT', body: JSON.stringify({ intervalHours: Number(policy.intervalHours), retainCount: Number(policy.retainCount) }),
      });
      notice = '备份策略已保存。';
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }

  async function createBackup() {
    creating = true;
    error = '';
    notice = '';
    try {
      await request('/api/backups', { method: 'POST', body: JSON.stringify({ name: '' }) });
      await refresh();
      if (!error) notice = '备份已创建。';
    } catch (err) {
      error = err.message;
    } finally {
      creating = false;
    }
  }

  async function download(key) {
    downloading = key;
    error = '';
    try {
      const fileToken = await request('/api/files/token', { method: 'POST' });
      const link = document.createElement('a');
      link.href = `/api/backups/${encodeURIComponent(key)}?token=${encodeURIComponent(fileToken.token)}`;
      link.download = key;
      document.body.appendChild(link);
      link.click();
      link.remove();
    } catch (err) {
      error = err.message;
    } finally {
      downloading = '';
    }
  }

  function formatSize(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`;
  }

  function formatDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
  }

  onMount(() => { if (token) refresh(); });
</script>

<svelte:head><title>{token ? '备份管理' : '管理员登录'} · HPA Hub</title></svelte:head>

<div class="app-shell">
  <header class="topbar">
    <a class="brand" href="/" aria-label="HPA Hub 首页"><span class="brand-mark" aria-hidden="true"><i></i><i></i><i></i></span><span>HPA<span class="brand-light">HUB</span></span></a>
    <div class="topbar-right"><span class="workspace">管理控制台</span>{#if token}<button class="text-button" on:click={logout}>退出登录 <span aria-hidden="true">↗</span></button>{/if}</div>
  </header>

  {#if !token}
    <main class="login-layout">
      <section class="login-intro">
        <div class="intro-line"><span></span> HPA HUB / ADMIN</div>
        <h1>重要的数据，<br /><em>值得妥善保存。</em></h1>
        <p>在一个地方管理备份策略与备份文件，让每一次保存都清晰可见。</p>
        <div class="archive-art" aria-hidden="true"><div class="art-label">DATA<br />ARCHIVE</div><div class="art-rings"><span></span><span></span><span></span></div><div class="art-bottom">KEEP WHAT MATTERS <span>✳</span></div></div>
      </section>
      <section class="login-panel" aria-labelledby="login-title">
        <div class="panel-top"><span>安全访问</span><span>HPA / ADMIN</span></div>
        <h2 id="login-title">管理员登录</h2>
        <p class="muted">使用 PocketBase 超级管理员账户继续。</p>
        <form on:submit|preventDefault={login} class="login-form">
          <label for="identity">管理员邮箱</label>
          <input id="identity" type="email" bind:value={identity} autocomplete="username" placeholder="name@example.com" required />
          <label for="password">密码</label>
          <input id="password" type="password" bind:value={password} autocomplete="current-password" placeholder="输入密码" required />
          {#if error}<p class="message error" role="alert">{error}</p>{/if}
          <button class="primary login-submit" type="submit" disabled={loading}>{loading ? '正在登录…' : '登录管理控制台'} <span aria-hidden="true">↗</span></button>
        </form>
        <p class="login-foot">仅限授权管理员访问</p>
      </section>
    </main>
  {:else}
    <main class="dashboard">
      <div class="breadcrumb">管理控制台 <span>/</span> 备份管理</div>
      <div class="page-heading"><div><h1>备份管理<span class="heading-dot">.</span></h1><p>管理自动备份策略，或随时创建一份备份。</p></div><button class="primary" on:click={createBackup} disabled={creating || loading}><span class="plus" aria-hidden="true">＋</span> {creating ? '正在创建…' : '立即备份'}</button></div>
      {#if error}<div class="message error" role="alert">{error}</div>{/if}
      {#if notice}<div class="message success" role="status">{notice}</div>{/if}
      <div class="content-grid">
        <section class="policy-section" aria-labelledby="policy-title">
          <div class="section-head"><div><div class="section-icon" aria-hidden="true">◷</div><h2 id="policy-title">自动备份</h2><p>设置备份频率与保留数量。</p></div><span class:enabled={Number(policy.intervalHours) > 0} class="status"><span></span>{Number(policy.intervalHours) > 0 ? '已启用' : '未启用'}</span></div>
          <form on:submit|preventDefault={savePolicy} class="policy-form">
            <label for="interval">备份间隔 <span>小时</span></label>
            <input id="interval" type="number" min="0" max="8760" step="1" bind:value={policy.intervalHours} required disabled={loading} />
            <p class="field-help">设为 0 时关闭自动备份。</p>
            <label for="retention">保留数量 <span>份</span></label>
            <input id="retention" type="number" min="1" max="100" step="1" bind:value={policy.retainCount} required disabled={loading} />
            <p class="field-help">仅限制自动备份数量，不删除手动备份。</p>
            <button class="secondary" type="submit" disabled={saving || loading}>{saving ? '正在保存…' : '保存策略'} <span aria-hidden="true">↗</span></button>
          </form>
          <div class="policy-note"><span aria-hidden="true">✳</span><p>定期检查备份文件，并将重要副本下载到安全的位置。</p></div>
        </section>
        <section class="archive-section" aria-labelledby="archive-title">
          <div class="archive-head"><div><h2 id="archive-title">备份记录 <span class="count">{backups.length}</span></h2><p>查看和下载已保存的备份文件。</p></div><button class="refresh" on:click={refresh} disabled={loading} aria-label="刷新备份列表" title="刷新备份列表">↻</button></div>
          <div class="table-wrap"><table><thead><tr><th>文件名称</th><th>创建时间</th><th>大小</th><th><span class="sr-only">操作</span></th></tr></thead><tbody>
            {#each backups as backup (backup.key)}<tr><td><div class="file-name"><span class="file-icon" aria-hidden="true">▤</span><span>{backup.key}</span></div></td><td>{formatDate(backup.modified)}</td><td class="size">{formatSize(backup.size)}</td><td><button class="download" on:click={() => download(backup.key)} disabled={!!downloading}>{downloading === backup.key ? '准备中…' : '下载'} <span aria-hidden="true">↓</span></button></td></tr>{/each}
          </tbody></table></div>
          {#if !backups.length}<div class="empty"><div class="empty-symbol" aria-hidden="true">⌑</div><h3>{loading ? '正在加载备份' : '还没有备份记录'}</h3><p>{loading ? '请稍候。' : '创建第一份备份，文件将显示在这里。'}</p></div>{/if}
          <div class="archive-footer"><span>备份文件存储于服务器</span><span>HPA HUB / ARCHIVE</span></div>
        </section>
      </div>
    </main>
  {/if}
  <footer class="site-footer"><span>HPA HUB <span class="footer-divider">/</span> 管理控制台</span><span>让数据安心留存。</span></footer>
</div>
