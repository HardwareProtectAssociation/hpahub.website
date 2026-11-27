<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import Select from './Select.svelte';

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
  let replyContext = null;

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

  function reply(msg) {
    replyContext = msg;
    activeTab = 'compose';
    recipientId = msg.senderId;
    if (!members.some((member) => member.id === msg.senderId)) {
      members = [...members, { id: msg.senderId, name: msg.senderName || msg.senderId }];
    }
    subject = msg.subject ? (/^re:\s/i.test(msg.subject) ? msg.subject : `Re: ${msg.subject}`) : '';
    content = '';
    notice = '';
    markRead(msg);
  }

  function cancelReply() {
    replyContext = null;
    recipientId = '';
    subject = '';
    content = '';
  }

  async function send() {
    if (!recipientId) { error = '请选择成员。'; return; }
    if (!content.trim()) return;
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
      replyContext = null;
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
      <h2>{replyContext ? $t('回复消息') : $t('发送消息')}</h2>
      {#if replyContext}
        <div class="reply-context">
          <span>{$t('回复 {name}', { name: replyContext.senderName || replyContext.senderId })}{#if replyContext.subject}：{replyContext.subject}{/if}</span>
          <button type="button" class="reply-cancel" on:click={cancelReply}>{$t('取消回复')}</button>
        </div>
      {/if}
      <div class="select-field"><span>{$t('收件人')}</span><Select bind:value={recipientId} ariaLabel={$t('选择成员')} options={[{ value: '', label: $t('选择成员') }, ...members.map((member) => ({ value: member.id, label: member.name || member.id }))]} /></div>
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
          <article class:unread={!msg.read} class="message-card">
            <div class="message-head">
              <div>
                <span class="category-tag">{msg.category === 'user' ? $t('用户消息') : msg.category === 'system' ? $t('系统通知') : $t('项目审核')}</span>
                <strong>{msg.subject || (msg.senderName || '—')}</strong>
              </div>
              <div class="message-head-actions">{#if !msg.read}<button type="button" class="read-button" on:click={() => markRead(msg)}>{$t('标记为已读')}</button>{/if}<time>{new Date(msg.created).toLocaleString()}</time></div>
            </div>
            {#if msg.subject}<p class="sender">{$t('来自')}：{msg.senderName || '—'}</p>{/if}
            {#if msg.projectTitle}<p class="project-ref">{$t('相关项目')}：{msg.projectTitle}</p>{/if}
            <p class="message-content">{msg.content}</p>
            {#if msg.category === 'user' && msg.senderId}
              <div class="message-actions">
                <button type="button" class="reply-button" on:click|stopPropagation={() => reply(msg)} on:keydown|stopPropagation>{$t('回复')}</button>
              </div>
            {/if}
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
  .compose-form input, .compose-form textarea { width: 100%; padding: 9px 11px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .compose-form input:focus, .compose-form textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  .compose-form textarea { resize: vertical; }
  .message-list { display: grid; gap: 12px; }
  .message-card { position: relative; padding: 16px 18px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); transition: border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .message-card:hover { border-color: var(--border-strong); box-shadow: var(--shadow-sm); }
  .message-card.unread { border-color: var(--border-strong); background: var(--surface-tint); }
  .message-head-actions { display: flex; align-items: center; gap: 12px; }
  .read-button { min-height: 38px; padding: 0 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--accent-deep); cursor: pointer; }
  .read-button:hover { border-color: var(--border-strong); background: var(--surface-tint); }
  @media (max-width: 600px) { .message-head { align-items: flex-start; flex-direction: column; }.message-head-actions { width: 100%; justify-content: space-between; } }
  .message-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
  .message-head strong { display: block; margin-top: 4px; color: var(--ink-primary); }
  .category-tag { display: inline-block; padding: 2px 9px; border-radius: 999px; background: var(--surface-tint); color: var(--accent-deep); font-size: 11px; font-weight: 600; }
  .message-head time { color: var(--ink-subtle); font-family: var(--font-mono); font-size: 11.5px; white-space: nowrap; }
  .sender, .project-ref { margin: 4px 0; color: var(--ink-muted); font-size: 12px; }
  .message-content { margin: 8px 0 0; color: var(--ink-secondary); font-size: 14px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .message-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
  .reply-button { padding: 3px 11px; border: 1px solid var(--border-default); border-radius: 999px; background: transparent; color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .reply-button:hover { border-color: var(--accent); color: var(--accent-deep); background: var(--accent-soft); }
  .reply-context { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 12px; border: 1px solid var(--accent); border-radius: var(--radius-sm); background: var(--accent-soft); color: var(--accent-deep); font-size: 13px; }
  .reply-cancel { flex: none; padding: 2px 10px; border: 1px solid var(--accent); border-radius: 999px; background: transparent; color: var(--accent-deep); font-size: 12px; cursor: pointer; }
  .reply-cancel:hover { background: var(--surface-elevated); }
  .unread-dot { position: absolute; top: 14px; right: 14px; width: 8px; height: 8px; border-radius: 50%; background: var(--accent-bright); animation: mint-pulse 2.2s ease-in-out infinite; }
  .empty { padding: 30px 0; color: var(--ink-subtle); }
  @media (max-width: 600px) { .message-head { flex-direction: column; } }
</style>
