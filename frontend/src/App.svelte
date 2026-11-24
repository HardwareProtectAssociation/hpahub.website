<script>
  import { onMount } from 'svelte';

  let token = sessionStorage.getItem('authToken') || '';
  let collection = sessionStorage.getItem('authCollection') || '';
  let checking = !!token;
  let page = 'home';
  let mode = 'login';
  let inviteCode = '';
  let passwordConfirm = '';
  let invitations = [];
  let generating = false;
  let identity = '';
  let password = '';
  let role = 'guest';
  $: isAdmin = role === 'admin' || role === 'octopus';
  $: isOctopus = role === 'octopus';
  let policy = { intervalHours: 0, retainCount: 10 };
  let backups = [];
  let loading = false;
  let saving = false;
  let creating = false;
  let downloading = '';
  let error = '';
  let notice = '';
  let expiryTimer;

  function scheduleExpiry() {
    clearTimeout(expiryTimer);
    try {
      const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
      const delay = payload.exp * 1000 - Date.now();
      if (!Number.isFinite(delay)) throw new Error('Invalid token');
      if (delay <= 0) {
        logout();
        error = '登录已失效，请重新登录。';
      } else {
        expiryTimer = setTimeout(scheduleExpiry, Math.min(delay, 2147483647));
      }
    } catch {
      logout();
      error = '登录已失效，请重新登录。';
    }
  }

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
        logout();
        error = '登录已失效，请重新登录。';
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
      const result = await request('/api/hpahub/login', {
        method: 'POST', body: JSON.stringify({ email: identity, password }),
      });
      collection = result.record.collectionName;
      token = result.token;
      sessionStorage.setItem('authToken', token);
      sessionStorage.setItem('authCollection', collection);
      scheduleExpiry();
      checking = true;
      role = (await request('/api/hpahub/me')).role;
      password = '';
      page = 'home';
    } catch (err) {
      if (token && role === 'guest') logout();
      error = err.message;
    } finally {
      checking = false;
      loading = false;
    }
  }

  async function register() {
    loading = true;
    error = '';
    try {
      const result = await request('/api/hpahub/register', {
        method: 'POST', body: JSON.stringify({ email: identity, password, passwordConfirm, inviteCode }),
      });
      collection = result.record.collectionName;
      token = result.token;
      sessionStorage.setItem('authToken', token);
      sessionStorage.setItem('authCollection', collection);
      scheduleExpiry();
      checking = true;
      role = (await request('/api/hpahub/me')).role;
      password = '';
      passwordConfirm = '';
      inviteCode = '';
      page = 'home';
    } catch (err) {
      if (token && role === 'guest') logout();
      error = err.message;
    } finally {
      loading = false;
      checking = false;
    }
  }

  async function openConsole() {
    page = 'console';
    error = '';
    await Promise.all([loadInvitations(), ...(isOctopus ? [refresh()] : [])]);
  }

  async function loadInvitations() {
    try {
      const result = await request('/api/hpahub/invitations');
      invitations = result.items;
    } catch (err) {
      error = err.message;
    }
  }

  async function generateInvitation() {
    generating = true;
    error = '';
    notice = '';
    try {
      await request('/api/hpahub/invitations', { method: 'POST' });
      await loadInvitations();
      if (!error) notice = '邀请码已生成，有效期为 24 小时。';
    } catch (err) {
      error = err.message;
    } finally {
      generating = false;
    }
  }

  function logout() {
    clearTimeout(expiryTimer);
    token = '';
    collection = '';
    role = 'guest';
    sessionStorage.removeItem('authToken');
    sessionStorage.removeItem('authCollection');
    sessionStorage.removeItem('adminToken');
    backups = [];
    invitations = [];
    password = '';
    page = 'home';
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

  onMount(async () => {
    sessionStorage.removeItem('adminToken');
    if (!token) return;
    if (!['_superusers', 'users'].includes(collection)) {
      logout();
      checking = false;
      return;
    }
    try {
      const result = await request(`/api/collections/${collection}/auth-refresh`, { method: 'POST' });
      token = result.token;
      sessionStorage.setItem('authToken', token);
      scheduleExpiry();
      role = (await request('/api/hpahub/me')).role;
    } catch (err) {
      logout();
      error = '无法验证登录状态，请重新登录。';
    } finally {
      checking = false;
    }
  });
</script>

<svelte:head><title>{token ? (page === 'console' ? '管理控制台' : '首页') : (mode === 'login' ? '登录' : '注册')} · HPA Hub</title></svelte:head>

<div class="app-shell">
  <header class="topbar">
    <a class="brand" href="/" aria-label="HPA Hub 首页"><span class="brand-mark" aria-hidden="true"><i></i><i></i><i></i></span><span>HPA<span class="brand-light">HUB</span></span></a>
    <nav class="topbar-right" aria-label="主导航">
      {#if token && !checking}
        <button class="text-button" on:click={() => { page = 'home'; error = ''; notice = ''; }}>首页</button>
        {#if isAdmin}<button class="text-button" on:click={openConsole}>管理控制台</button>{/if}
        <button class="text-button" on:click={logout}>退出登录</button>
      {/if}
    </nav>
  </header>

  {#if checking}
    <main class="dashboard"><p role="status">正在验证登录状态…</p></main>
  {:else if !token}
    <main class="login-layout">
      <section class="login-intro">
        <h1>欢迎来到<br /><em>HPA Hub。</em></h1>
        <p>登录你的账户，开始使用 HPA Hub。新用户可使用邀请码注册。</p>
        <div class="archive-art" aria-hidden="true"><div class="art-label">HPA<br />HUB</div><div class="art-rings"><span></span><span></span><span></span></div><div class="art-bottom">WELCOME <span>✳</span></div></div>
      </section>
      <section class="login-panel" aria-labelledby="login-title">
        <div class="panel-top"><span>账户访问</span><span>HPA HUB</span></div>
        <h2 id="login-title">{mode === 'login' ? '登录' : '使用邀请码注册'}</h2>
        <p class="muted">{mode === 'login' ? '普通用户与管理员使用同一入口登录。' : '邀请码有效期为 24 小时，仅可使用一次。'}</p>
        <form on:submit|preventDefault={mode === 'login' ? login : register} class="login-form">
          <label for="identity">{mode === 'login' ? '账号或邮箱' : '邮箱'}</label>
          {#if mode === 'login'}
            <input id="identity" type="text" bind:value={identity} autocomplete="username" placeholder="输入账号或邮箱" required />
          {:else}
            <input id="identity" type="email" bind:value={identity} autocomplete="username" placeholder="name@example.com" required />
          {/if}
          <label for="password">密码</label>
          <input id="password" type="password" bind:value={password} autocomplete={mode === 'login' ? 'current-password' : 'new-password'} placeholder="输入密码" required />
          {#if mode === 'register'}
            <label for="password-confirm">确认密码</label>
            <input id="password-confirm" type="password" bind:value={passwordConfirm} autocomplete="new-password" required />
            <label for="invite-code">邀请码</label>
            <input id="invite-code" bind:value={inviteCode} autocomplete="off" placeholder="HPA-XXXXXXXX" required />
          {/if}
          {#if error}<p class="message error" role="alert">{error}</p>{/if}
          <button class="primary login-submit" type="submit" disabled={loading}>{loading ? '请稍候…' : (mode === 'login' ? '登录' : '注册')} <span aria-hidden="true">↗</span></button>
        </form>
        <p class="login-foot">{mode === 'login' ? '还没有账户？' : '已有账户？'} <button class="text-button" on:click={() => { mode = mode === 'login' ? 'register' : 'login'; error = ''; password = ''; passwordConfirm = ''; }}>{mode === 'login' ? '使用邀请码注册' : '返回登录'}</button></p>
      </section>
    </main>
  {:else if page === 'home' || !isAdmin}
    <main class="dashboard" aria-label="首页"></main>
  {:else}
    <main class="dashboard">
      <div class="breadcrumb">管理控制台 <span>/</span> {isOctopus ? '备份管理' : '邀请码'}</div>
      {#if isOctopus}
        <div class="page-heading"><div><h1>备份管理<span class="heading-dot">.</span></h1><p>管理自动备份策略，或随时创建一份备份。</p></div><button class="primary" on:click={createBackup} disabled={creating || loading}><span class="plus" aria-hidden="true">＋</span> {creating ? '正在创建…' : '立即备份'}</button></div>
      {:else}
        <div class="page-heading"><div><h1>邀请码<span class="heading-dot">.</span></h1><p>生成邀请码并查看使用记录。</p></div></div>
      {/if}
      {#if error}<div class="message error" role="alert">{error}</div>{/if}
      {#if notice}<div class="message success" role="status">{notice}</div>{/if}
      {#if isOctopus}
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
      {/if}
      <section class="archive-section invitations" aria-labelledby="invitations-title">
        <div class="archive-head">
          <div><h2 id="invitations-title">邀请码</h2><p>邀请码有效 24 小时，仅可使用一次。</p></div>
          <button class="primary" on:click={generateInvitation} disabled={generating}>{generating ? '正在生成…' : '生成邀请码'}</button>
        </div>
        <div class="table-wrap"><table><thead><tr><th>邀请码</th><th>创建时间</th><th>到期时间</th><th>状态</th><th>使用者</th><th>使用时间</th></tr></thead><tbody>
          {#each invitations as invitation (invitation.code)}
            <tr><td>{invitation.code}</td><td>{formatDate(invitation.created * 1000)}</td><td>{formatDate(invitation.expires * 1000)}</td><td>{invitation.used ? '已使用' : invitation.expires * 1000 <= Date.now() ? '已过期' : '未使用'}</td><td>{invitation.usedBy || '—'}</td><td>{invitation.used ? formatDate(invitation.used * 1000) : '—'}</td></tr>
          {/each}
        </tbody></table></div>
        {#if !invitations.length}<div class="empty"><h3>还没有邀请码</h3><p>生成邀请码后，可在这里查看使用记录。</p></div>{/if}
      </section>
    </main>
  {/if}
  <footer class="site-footer"><span>HPA HUB</span><span>欢迎来到 HPA Hub。</span></footer>
</div>
