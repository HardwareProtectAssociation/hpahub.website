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

  async function load() {
    loading = true;
    error = '';
    try {
      const [msgData, memberData] = await Promise.all([
        request('/api/hpahub/me/messages?limit=50'),
        request('/api/hpahub/members'),
      ]);
      messages = msgData.items || [];
      unreadCount = msgData.unreadCount || 0;
      members = (memberData.items || []).filter((m) => m.id !== memberData.items?.find((x) => x.id)?.id);
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

  onMount(load);
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
    {/if}
  {/if}
</main>

<style>
  .messages-page { width: min(900px, 88%); }
  .messages-page .page-heading { margin-top: 0; }
  .message-tabs { display: flex; gap: 6px; margin: 10px 0 22px; padding-bottom: 8px; border-bottom: 1px solid #dcded4; }
  .message-tabs button { border: 1px solid transparent; background: none; padding: 7px 14px; color: #52695a; font-size: 13px; cursor: pointer; }
  .message-tabs button.active { background: #274732; color: #fff; }
  .badge { display: inline-block; margin-left: 5px; padding: 1px 7px; border-radius: 999px; background: #e36b82; color: #fff; font-size: 11px; }
  .compose-form { display: grid; gap: 12px; padding: 20px; border: 1px solid #dcded4; background: #fafbf7; }
  .compose-form h2 { margin: 0; font-size: 18px; }
  .compose-form label { display: grid; gap: 5px; color: #52695a; font-size: 13px; }
  .compose-form input, .compose-form select, .compose-form textarea { width: 100%; padding: 9px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .compose-form textarea { resize: vertical; }
  .message-list { display: grid; gap: 10px; }
  .message-card { position: relative; padding: 16px; border: 1px solid #dcded4; background: #fafbf7; cursor: pointer; }
  .message-card.unread { border-left: 3px solid #e36b82; background: #fff8f0; }
  .message-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
  .message-head strong { display: block; margin-top: 2px; }
  .category-tag { display: inline-block; padding: 1px 8px; border-radius: 999px; background: #eef3e8; color: #4d6448; font-size: 11px; }
  .message-head time { color: #8b9587; font-size: 12px; white-space: nowrap; }
  .sender, .project-ref { margin: 4px 0; color: #818b7f; font-size: 12px; }
  .message-content { margin: 8px 0 0; color: #354839; font-size: 14px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .unread-dot { position: absolute; top: 12px; right: 12px; width: 8px; height: 8px; border-radius: 50%; background: #e36b82; }
  .empty { padding: 30px 0; color: #818b7f; }
  @media (max-width: 600px) { .message-head { flex-direction: column; } }
</style>
