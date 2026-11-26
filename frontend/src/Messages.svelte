<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;

  let messages = [];
  let unreadCount = 0;
  let loading = true;
  let error = '';
  let notice = '';
  let sending = false;
  let recipientId = '';
  let subject = '';
  let content = '';
  let members = [];
  let activeTab = 'inbox';
  let hasMore = false;
  let offset = 0;

  const categoryLabels = { user: '用户消息', system: '系统通知', project_approval: '项目审核' };

  async function request(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: { Authorization: `Bearer ${token}`, ...(options.headers || {}) },
      cache: 'no-store',
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.message || `请求失败（${response.status}）`);
    return body;
  }

  async function load(append = false) {
    loading = true;
    error = '';
    try {
      const [msgData, memberData, center] = await Promise.all([
        request(`/api/hpahub/me/messages?limit=51&offset=${append ? offset : 0}`),
        request('/api/hpahub/members'),
        request('/api/hpahub/me/center'),
      ]);
      const page = msgData.items || [];
      hasMore = page.length > 50;
      messages = append ? [...messages, ...page.slice(0, 50)] : page.slice(0, 50);
      offset = messages.length;
      unreadCount = msgData.unreadCount || 0;
      members = (memberData.items || []).filter((m) => m.id && m.id !== center.profile.id);
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  }

  async function markRead(msg) {
    if (msg.read) return;
    try {
      await request(`/api/hpahub/me/messages/${msg.id}/read`, { method: 'POST' });
      msg.read = true;
      unreadCount = Math.max(0, unreadCount - 1);
    } catch {}
  }

  async function markAllRead() {
    try {
      await request('/api/hpahub/me/messages/read-all', { method: 'POST' });
      messages = messages.map((m) => ({ ...m, read: true }));
      unreadCount = 0;
    } catch {}
  }

  async function send() {
    if (!recipientId || !content.trim()) return;
    sending = true;
    error = '';
    try {
      await request('/api/hpahub/me/messages', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ recipientId, subject, content }),
      });
      subject = '';
      content = '';
      notice = '消息已发送。';
    } catch (err) {
      error = err.message || '发送失败，请重试。';
    } finally {
      sending = false;
    }
  }

  onMount(() => load());
</script>

<main class="dashboard messages-page">
  <div class="page-heading">
    <div><h1>{$t('站内消息')}<span class="heading-dot">.</span></h1><p>{$t('与其他成员交流，接收项目审核通知。')}</p></div>
    {#if unreadCount > 0}
      <button type="button" class="secondary" on:click={markAllRead}>{$t('全部标为已读')}</button>
    {/if}
  </div>

  {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
  {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}

  <nav class="message-tabs" aria-label={$t('消息分类')}>
    <button type="button" class:active={activeTab === 'inbox'} on:click={() => activeTab = 'inbox'}>{$t('收件箱')}{#if unreadCount > 0}<span class="badge">{unreadCount}</span>{/if}</button>
    <button type="button" class:active={activeTab === 'compose'} on:click={() => activeTab = 'compose'}>{$t('发消息')}</button>
  </nav>

  {#if activeTab === 'compose'}
    <form class="compose-form" on:submit|preventDefault={send}>
      <h2>{$t('发送消息')}</h2>
      <label>{$t('收件人')}
        <select bind:value={recipientId} required>
          <option value="">{$t('选择成员')}</option>
          {#each members as member}
            <option value={member.id}>{member.name || member.id}</option>
          {/each}
        </select>
      </label>
      <label>{$t('主题')}<input bind:value={subject} maxlength="200" /></label>
      <label>{$t('内容')}<textarea bind:value={content} maxlength="5000" rows="5" required></textarea></label>
      <button type="submit" class="primary" disabled={sending}>{sending ? $t('正在发送…') : $t('发送')}</button>
    </form>
  {:else}
    {#if loading}
      <p role="status">{$t('正在加载…')}</p>
    {:else if !messages.length}
      <p class="empty">{$t('暂无消息。')}</p>
    {:else}
      <div class="message-list">
        {#each messages as msg (msg.id)}
          <article class:unread={!msg.read} class="message-card" on:click={() => markRead(msg)} on:keydown={(e) => e.key === 'Enter' && markRead(msg)} tabindex="0">
            <div class="message-head">
              <div>
                <span class="category-tag">{msg.category === 'user' ? $t('用户消息') : msg.category === 'system' ? $t('系统通知') : $t('项目审核')}</span>
                <strong>{msg.subject || (msg.senderName || '—')}</strong>
              </div>
              <time>{new Date(msg.created).toLocaleString()}</time>
            </div>
            {#if msg.subject}<p class="sender">{$t('来自')}：{msg.senderName || '—'}</p>{/if}
            {#if msg.projectTitle}<p class="project-ref">{$t('相关项目')}：{msg.projectTitle}</p>{/if}
            <p class="message-content">{msg.content}</p>
            {#if !msg.read}<span class="unread-dot" aria-label={$t('未读')}></span>{/if}
          </article>
        {/each}
      </div>
      {#if hasMore}<button type="button" class="secondary" on:click={() => load(true)} disabled={loading}>{$t('加载更多')}</button>{/if}
    {/if}
  {/if}
</main>

<style>
  .messages-page { width: min(900px, 88%); }
  .messages-page .page-heading { margin-top: 0; }
  .message-tabs { display: flex; gap: 6px; margin: 10px 0 22px; padding-bottom: 10px; border-bottom: 1px solid var(--border-subtle); }
  .message-tabs button { border: 1px solid transparent; border-radius: var(--radius-sm); background: none; padding: 8px 14px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .message-tabs button:hover { background: var(--surface-tint); color: var(--accent-deep); }
  .message-tabs button.active { background: linear-gradient(135deg, var(--accent), var(--accent-deep)); color: #fff; box-shadow: var(--shadow-glow); }
  .badge { display: inline-block; margin-left: 6px; padding: 1px 7px; border-radius: 999px; background: var(--badge-rose); color: #fff; font-family: var(--font-mono); font-size: 11px; }
  .compose-form { display: grid; gap: 12px; padding: 22px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-sm); }
  .compose-form h2 { margin: 0; font-size: 18px; }
  .compose-form label { display: grid; gap: 5px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; }
  .compose-form input, .compose-form select, .compose-form textarea { width: 100%; padding: 9px 11px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .compose-form input:focus, .compose-form select:focus, .compose-form textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  .compose-form textarea { resize: vertical; }
  .message-list { display: grid; gap: 12px; }
  .message-card { position: relative; padding: 16px 18px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); cursor: pointer; box-shadow: var(--shadow-xs); transition: transform var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .message-card:hover { transform: translateY(-2px); border-color: var(--border-strong); box-shadow: var(--shadow-sm); }
  .message-card.unread { border-left: 3px solid var(--accent-bright); background: #f2fbf6; }
  .message-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
  .message-head strong { display: block; margin-top: 4px; color: var(--ink-primary); }
  .category-tag { display: inline-block; padding: 2px 9px; border-radius: 999px; background: var(--surface-tint); color: var(--accent-deep); font-size: 11px; font-weight: 600; }
  .message-head time { color: var(--ink-subtle); font-family: var(--font-mono); font-size: 11.5px; white-space: nowrap; }
  .sender, .project-ref { margin: 4px 0; color: var(--ink-muted); font-size: 12px; }
  .message-content { margin: 8px 0 0; color: var(--ink-secondary); font-size: 14px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .unread-dot { position: absolute; top: 14px; right: 14px; width: 8px; height: 8px; border-radius: 50%; background: var(--accent-bright); animation: mint-pulse 2.2s ease-in-out infinite; }
  .empty { padding: 30px 0; color: var(--ink-subtle); }
  @media (max-width: 600px) { .message-head { flex-direction: column; } }
</style>
