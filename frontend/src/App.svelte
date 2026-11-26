<script>
  import { onMount } from 'svelte';
  import { locale, languages, t } from './i18n.js';
  import Hardware from './Hardware.svelte';
  import News from './News.svelte';
  import PersonalCenter from './PersonalCenter.svelte';
  import SkillEditor from './SkillEditor.svelte';
  import AdminDirectory from './AdminDirectory.svelte';
  import MyProjects from './MyProjects.svelte';
  import Messages from './Messages.svelte';
  import PublicPages from './PublicPages.svelte';
  import Editors from './Editors.svelte';

  const pathname = location.pathname.replace(/\/+$/, '') || '/';
  if (pathname === '/gpu') location.replace('/hardware');

  let token = sessionStorage.getItem('authToken') || '';
  let collection = sessionStorage.getItem('authCollection') || '';
  let checking = !!token;
  let page = 'home';
  let menuOpen = false;
  let mode = pathname === '/join' ? 'register' : 'login';
  let inviteCode = '';
  let passwordConfirm = '';
  let identity = '';
  let password = '';
  let role = 'guest';
  $: isAdmin = role === 'admin' || role === 'octopus';
  let unreadMessages = 0;
  let messagePollTimer = null;
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
        error = $t('登录已失效，请重新登录。');
      } else {
        expiryTimer = setTimeout(scheduleExpiry, Math.min(delay, 2147483647));
      }
    } catch {
      logout();
      error = $t('登录已失效，请重新登录。');
    }
  }

  async function request(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: {
        ...(options.body && !(options.body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options.headers,
      },
    });
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      if (response.status === 401 && token) {
        logout();
        error = $t('登录已失效，请重新登录。');
      }
      throw new Error(data.message || $t('请求失败（{status}）', { status: response.status }));
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
      location.href = '/';
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
      location.href = '/';
    } catch (err) {
      if (token && role === 'guest') logout();
      error = err.message;
    } finally {
      loading = false;
      checking = false;
    }
  }

  async function openConsole() {
    menuOpen = false;
    page = 'console';
    error = '';
    if (role === 'octopus') await refresh();
  }

  function logout() {
    menuOpen = false;
    clearTimeout(expiryTimer);
    if (messagePollTimer) clearInterval(messagePollTimer);
    token = '';
    collection = '';
    role = 'guest';
    sessionStorage.removeItem('authToken');
    sessionStorage.removeItem('authCollection');
    sessionStorage.removeItem('adminToken');
    backups = [];
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
      notice = $t('备份策略已保存。');
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
      if (!error) notice = $t('备份已创建。');
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

  function formatDate(value, language) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(language, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
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
      fetchUnreadMessages();
      messagePollTimer = setInterval(fetchUnreadMessages, 60000);
      if (['admin', 'octopus'].includes(role) && new URLSearchParams(location.search).has('console')) await openConsole();
    } catch (err) {
      logout();
      error = $t('无法验证登录状态，请重新登录。');
    } finally {
      checking = false;
    }
  });

  async function fetchUnreadMessages() {
    if (!token) return;
    try {
      const result = await request('/api/hpahub/me/messages/unread-count');
      unreadMessages = result.unreadCount || 0;
    } catch {}
  }
</script>

<svelte:head><title>HPA Hub</title></svelte:head>

<svelte:window on:keydown={(event) => { if (event.key === 'Escape') menuOpen = false; }} />

<div class="app-shell">
  {#if menuOpen}<button class="menu-backdrop" aria-label={$t('关闭导航菜单')} on:click={() => menuOpen = false}></button>{/if}
  <header class:menu-open={menuOpen} class="topbar">
    <a class="brand" href="/" aria-label={$t('HPA Hub 首页')}><span class="brand-mark" aria-hidden="true"><i></i><i></i><i></i></span><span>HPA<span class="brand-light">HUB</span></span></a>
    <nav id="main-navigation" class="topbar-right" aria-label={$t('主导航')}>
      <a class="text-button" href="/" aria-current={pathname === '/' && page === 'home' ? 'page' : undefined}>{$t('首页')}</a>
      <a class="text-button" href="/hardware">{$t('硬件数据库')}</a>
      <a class="text-button" href="/news">{$t('硬件新闻')}</a>
      <a class="text-button" href="/community">{$t('成员')}</a>
      <a class="text-button" href="/showcase">{$t('项目大厅')}</a>
      {#if token && !checking}
        <a class="text-button" href="/me">{$t('个人中心')}</a>
        {#if ['member', 'admin', 'octopus'].includes(role)}<a class="text-button" href="/messages">{$t('站内消息')}{#if unreadMessages > 0}<span class="nav-badge">{unreadMessages}</span>{/if}</a>{/if}
        {#if ['member', 'admin', 'octopus'].includes(role)}<a class="text-button" href="/my-projects">{$t('我的项目')}</a>{/if}
        {#if ['member', 'admin'].includes(role)}<a class="text-button" href="/settings">{$t('个人设置')}</a>{/if}
        {#if isAdmin}
          <a class="text-button" href="/manage/showcase">{$t('项目大厅管理')}</a>
          {#if pathname === '/'}<button class="text-button" on:click={openConsole}>{$t('管理控制台')}</button>
          {:else}<a class="text-button" href="/?console=1">{$t('管理控制台')}</a>{/if}
        {/if}
      {/if}
      <div class="sidebar-footer">
        {#if token && !checking}
          <button class="text-button" on:click={logout}>{$t('退出登录')}</button>
        {:else if !checking}
          <a class="text-button" href="/sign-in">{$t('登录')}</a>
        {/if}
        <label><span class="sr-only">{$t('语言')}</span><select bind:value={$locale} aria-label={$t('语言')}>{#each Object.entries(languages) as [code, name]}<option value={code} lang={code}>{name}</option>{/each}</select></label>
      </div>
    </nav>
  </header>
  <button class="menu-toggle" aria-label={menuOpen ? $t('关闭导航菜单') : $t('打开导航菜单')} aria-expanded={menuOpen} aria-controls="main-navigation" on:click={() => menuOpen = !menuOpen}>
    <span aria-hidden="true">{menuOpen ? '×' : '☰'}</span>
  </button>

  {#if checking}
    <main class="dashboard"><p role="status">{$t('正在验证登录状态…')}</p></main>
  {:else if pathname === '/hardware'}
    <Hardware {token} {role} />
  {:else if pathname === '/news'}
    <News {token} />
  {:else if pathname === '/me/skills'}
    {#if token}
      <SkillEditor {token} />
    {:else}
      <main class="dashboard"><h1>{$t('技能管理')}</h1><p>{$t('请先登录后管理技能。')}</p><a href="/sign-in">{$t('登录')}</a></main>
    {/if}
  {:else if pathname === '/messages'}
    {#if token}
      <Messages {token} />
    {:else}
      <main class="dashboard"><h1>{$t('站内消息')}</h1><p>{$t('请先登录后查看消息。')}</p><a href="/sign-in">{$t('登录')}</a></main>
    {/if}
  {:else if pathname === '/my-projects'}
    {#if token}
      <MyProjects {token} />
    {:else}
      <main class="dashboard"><h1>{$t('我的项目')}</h1><p>{$t('请先登录后查看我的项目。')}</p><a href="/sign-in">{$t('登录')}</a></main>
    {/if}
  {:else if pathname === '/me'}
    {#if token}
      <PersonalCenter {token} />
    {:else}
      <main class="dashboard"><h1>{$t('个人中心')}</h1><p>{$t('请先登录后查看个人中心。')}</p><a href="/sign-in">{$t('登录')}</a></main>
    {/if}
  {:else if pathname === '/settings' || pathname === '/manage/showcase'}
    {#if token && (pathname === '/settings' ? ['member', 'admin'].includes(role) : isAdmin)}
      <Editors {token} {role} manage={pathname === '/manage/showcase'} />
    {:else}
      <main class="dashboard"><h1>{$t('无权访问')}</h1><p>{$t('请使用有权限的账户登录。')}</p><a href="/sign-in">{$t('登录')}</a></main>
    {/if}
  {:else if pathname === '/sign-in' || pathname === '/join'}
    <main class="login-layout">
      <section class="login-intro">
        <h1>{$t('欢迎来到')}<br /><em>HPA Hub。</em></h1>
        <p>{$t('登录你的账户，开始使用 HPA Hub。新用户可使用邀请码注册。')}</p>
        <div class="archive-art" aria-hidden="true"><div class="art-label">HPA<br />HUB</div><div class="art-rings"><span></span><span></span><span></span></div><div class="art-bottom">WELCOME <span>✳</span></div></div>
      </section>
      <section class="login-panel" aria-labelledby="login-title">
        <div class="panel-top"><span>{$t('账户访问')}</span><span>HPA HUB</span></div>
        <h2 id="login-title">{mode === 'login' ? $t('登录') : $t('使用邀请码注册')}</h2>
        <p class="muted">{mode === 'login' ? $t('普通用户与管理员使用同一入口登录。') : $t('邀请码有效期为 24 小时，仅可使用一次。')}</p>
        <form on:submit|preventDefault={mode === 'login' ? login : register} class="login-form">
          <label for="identity">{mode === 'login' ? $t('账号或邮箱') : $t('邮箱')}</label>
          {#if mode === 'login'}
            <input id="identity" type="text" bind:value={identity} autocomplete="username" placeholder={$t('输入账号或邮箱')} required />
          {:else}
            <input id="identity" type="email" bind:value={identity} autocomplete="username" placeholder="name@example.com" required />
          {/if}
          <label for="password">{$t('密码')}</label>
          <input id="password" type="password" bind:value={password} autocomplete={mode === 'login' ? 'current-password' : 'new-password'} placeholder={$t('输入密码')} required />
          {#if mode === 'register'}
            <label for="password-confirm">{$t('确认密码')}</label>
            <input id="password-confirm" type="password" bind:value={passwordConfirm} autocomplete="new-password" required />
            <label for="invite-code">{$t('邀请码')}</label>
            <input id="invite-code" bind:value={inviteCode} autocomplete="off" placeholder="HPA-XXXXXXXX" required />
          {/if}
          {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
          <button class="primary login-submit" type="submit" disabled={loading}>{loading ? $t('请稍候…') : (mode === 'login' ? $t('登录') : $t('注册'))} <span aria-hidden="true">↗</span></button>
        </form>
        <p class="login-foot">{mode === 'login' ? $t('还没有账户？') : $t('已有账户？')} <a class="text-button" href={mode === 'login' ? '/join' : '/sign-in'}>{mode === 'login' ? $t('使用邀请码注册') : $t('返回登录')}</a></p>
      </section>
    </main>
  {:else if pathname !== '/' || page === 'home' || !isAdmin}
    <PublicPages {pathname} {token} />
  {:else}
    <main class="dashboard">
      <div class="breadcrumb">{$t('管理控制台')} <span>/</span> {isOctopus ? $t('备份管理') : $t('成员与项目')}</div>
      {#if isOctopus}
        <div class="page-heading"><div><h1>{$t('备份管理')}<span class="heading-dot">.</span></h1><p>{$t('管理自动备份策略，或随时创建一份备份。')}</p></div><button class="primary" on:click={createBackup} disabled={creating || loading}><span class="plus" aria-hidden="true">＋</span> {creating ? $t('正在创建…') : $t('立即备份')}</button></div>
      {:else}
        <div class="page-heading"><div><h1>{$t('管理控制台')}<span class="heading-dot">.</span></h1><p>{$t('查看成员信息、审批加入申请和项目状态。')}</p></div></div>
      {/if}
      {#if error}<div class="message error" role="alert">{$t(error)}</div>{/if}
      {#if notice}<div class="message success" role="status">{notice}</div>{/if}
      {#if isOctopus}
      <div class="content-grid">
        <section class="policy-section" aria-labelledby="policy-title">
          <div class="section-head"><div><div class="section-icon" aria-hidden="true">◷</div><h2 id="policy-title">{$t('自动备份')}</h2><p>{$t('设置备份频率与保留数量。')}</p></div><span class:enabled={Number(policy.intervalHours) > 0} class="status"><span></span>{Number(policy.intervalHours) > 0 ? $t('已启用') : $t('未启用')}</span></div>
          <form on:submit|preventDefault={savePolicy} class="policy-form">
            <label for="interval">{$t('备份间隔')} <span>{$t('小时')}</span></label>
            <input id="interval" type="number" min="0" max="8760" step="1" bind:value={policy.intervalHours} required disabled={loading} />
            <p class="field-help">{$t('设为 0 时关闭自动备份。')}</p>
            <label for="retention">{$t('保留数量')} <span>{$t('份')}</span></label>
            <input id="retention" type="number" min="1" max="100" step="1" bind:value={policy.retainCount} required disabled={loading} />
            <p class="field-help">{$t('仅限制自动备份数量，不删除手动备份。')}</p>
            <button class="secondary" type="submit" disabled={saving || loading}>{saving ? $t('正在保存…') : $t('保存策略')} <span aria-hidden="true">↗</span></button>
          </form>
          <div class="policy-note"><span aria-hidden="true">✳</span><p>{$t('定期检查备份文件，并将重要副本下载到安全的位置。')}</p></div>
        </section>
        <section class="archive-section" aria-labelledby="archive-title">
          <div class="archive-head"><div><h2 id="archive-title">{$t('备份记录')} <span class="count">{backups.length}</span></h2><p>{$t('查看和下载已保存的备份文件。')}</p></div><button class="refresh" on:click={refresh} disabled={loading} aria-label={$t('刷新备份列表')} title={$t('刷新备份列表')}>↻</button></div>
          <div class="table-wrap"><table><thead><tr><th>{$t('文件名称')}</th><th>{$t('创建时间')}</th><th>{$t('大小')}</th><th><span class="sr-only">{$t('操作')}</span></th></tr></thead><tbody>
            {#each backups as backup (backup.key)}<tr><td><div class="file-name"><span class="file-icon" aria-hidden="true">▤</span><span>{backup.key}</span></div></td><td>{formatDate(backup.modified, $locale)}</td><td class="size">{formatSize(backup.size)}</td><td><button class="download" on:click={() => download(backup.key)} disabled={!!downloading}>{downloading === backup.key ? $t('准备中…') : $t('下载')} <span aria-hidden="true">↓</span></button></td></tr>{/each}
          </tbody></table></div>
          {#if !backups.length}<div class="empty"><div class="empty-symbol" aria-hidden="true">⌑</div><h3>{loading ? $t('正在加载备份') : $t('还没有备份记录')}</h3><p>{loading ? $t('请稍候。') : $t('创建第一份备份，文件将显示在这里。')}</p></div>{/if}
          <div class="archive-footer"><span>{$t('备份文件存储于服务器')}</span><span>HPA HUB / ARCHIVE</span></div>
        </section>
      </div>
      {/if}
      <AdminDirectory {token} />
    </main>
  {/if}
  <footer class="site-footer"><span>HPA HUB</span><span>{$t('欢迎来到 HPA Hub。')}</span></footer>
</div>
