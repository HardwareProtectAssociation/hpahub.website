<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;
  let news = [];
  let loading = true;
  let error = '';
  let notice = '';
  let posting = false;
  let title = '';
  let content = '';
  let link = '';
  let busyId = '';

  async function request(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(options.headers || {}) },
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
      const body = await request('/api/hpahub/news');
      news = body.items || [];
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  }

  async function publish() {
    posting = true;
    error = '';
    notice = '';
    try {
      await request('/api/hpahub/news', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, content, link }),
      });
      title = '';
      content = '';
      link = '';
      await load();
      notice = '新闻发布成功。';
    } catch (err) {
      error = err.message || '发布失败，请重试。';
    } finally {
      posting = false;
    }
  }

  async function react(item, reaction) {
    const next = item.myReaction === reaction ? 'none' : reaction;
    busyId = item.id;
    error = '';
    try {
      const result = await request(`/api/hpahub/news/${encodeURIComponent(item.id)}/reaction`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reaction: next }),
      });
      news = news.map((entry) => entry.id === item.id
        ? { ...entry, likeCount: result.likeCount, uselessCount: result.uselessCount, myReaction: result.myReaction }
        : entry);
    } catch (err) {
      error = err.message || '操作失败，请重试。';
    } finally {
      busyId = '';
    }
  }

  async function remove(item) {
    if (!confirm($t('确定删除这条新闻吗？'))) return;
    busyId = item.id;
    error = '';
    try {
      await request(`/api/hpahub/news/${encodeURIComponent(item.id)}`, { method: 'DELETE' });
      await load();
      notice = '新闻删除成功。';
    } catch (err) {
      error = err.message || '删除失败，请重试。';
    } finally {
      busyId = '';
    }
  }

  onMount(load);
</script>

<main class="dashboard news-page">
  <div class="page-heading">
    <div><h1>{$t('硬件新闻')}<span class="heading-dot">.</span></h1><p>{$t('成员分享的硬件、工具和行业动态。')}</p></div>
  </div>

  {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
  {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}

  {#if token}
    <form class="news-form" on:submit|preventDefault={publish}>
      <h2>{$t('发布新闻')}</h2>
      <label>{$t('新闻标题')}<input bind:value={title} maxlength="160" required /></label>
      <label>{$t('新闻内容')}<textarea bind:value={content} maxlength="5000" rows="4" required></textarea></label>
      <label>{$t('原文链接（可选）')}<input bind:value={link} maxlength="500" placeholder="https://" /></label>
      <div><button type="submit" class="primary" disabled={posting}>{posting ? $t('正在发布…') : $t('发布')}</button></div>
    </form>
  {:else}
    <p class="news-login"><a href="/sign-in">{$t('登录')}</a>{$t('后可发布新闻。')}</p>
  {/if}

  {#if loading}
    <p role="status">{$t('正在加载…')}</p>
  {:else if !news.length}
    <p class="empty">{$t('暂无新闻。')}</p>
  {:else}
    <div class="news-list">
      {#each news as item (item.id)}
        <article class="news-card">
          <div class="news-head">
            <div>
              <h2>{#if item.link}<a href={item.link} target="_blank" rel="noreferrer">{item.title}</a>{:else}{item.title}{/if}</h2>
              <p class="news-meta">{item.authorName || '—'} · {new Date(item.created).toLocaleString()}</p>
            </div>
            {#if item.canDelete}<button type="button" class="small" on:click={() => remove(item)} disabled={busyId === item.id}>{$t('删除')}</button>{/if}
          </div>
          <p class="news-content">{item.content}</p>
          {#if item.link}<a class="news-link" href={item.link} target="_blank" rel="noreferrer">{$t('查看原文')}</a>{/if}
          {#if token}
            {#if item.isAuthor}
              <p class="news-own">{$t('这是你发布的新闻，不能给自己投票。')}</p>
            {:else}
              <div class="news-reactions">
                <button type="button" class:active={item.myReaction === 'like'} on:click={() => react(item, 'like')} disabled={busyId === item.id}>
                  <span aria-hidden="true">♥</span> {$t('点赞')} {item.likeCount}
                </button>
                <button type="button" class:active={item.myReaction === 'useless'} on:click={() => react(item, 'useless')} disabled={busyId === item.id}>
                  <span aria-hidden="true">👎</span> {$t('觉得没用')} {item.uselessCount}
                </button>
              </div>
            {/if}
          {/if}
        </article>
      {/each}
    </div>
  {/if}
</main>

<style>
  .news-page { width: min(980px, 88%); }
  .news-page .page-heading { margin-top: 0; }
  .news-form { display: grid; gap: 12px; margin: 0 0 26px; padding: 20px; border: 1px solid #dcded4; background: #fafbf7; }
  .news-form h2 { margin: 0; font-size: 18px; }
  .news-form label { display: grid; gap: 5px; color: #52695a; font-size: 13px; }
  .news-form input, .news-form textarea { width: 100%; padding: 9px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .news-form textarea { resize: vertical; }
  .news-login { color: #6f7b6d; font-size: 14px; }
  .news-list { display: grid; gap: 14px; }
  .news-card { padding: 20px; border: 1px solid #dcded4; background: #fafbf7; }
  .news-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
  .news-head h2 { margin: 0 0 4px; font-size: 20px; }
  .news-head h2 a { color: #274732; text-decoration: none; }
  .news-meta { margin: 0; color: #818b7f; font-size: 12px; }
  .news-content { margin: 12px 0; color: #52695a; font-size: 14px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .news-link { color: #345d3d; font-size: 13px; }
  .news-reactions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 14px; }
  .news-reactions button { display: inline-flex; align-items: center; gap: 5px; padding: 5px 10px; border: 1px solid #d6ddd2; border-radius: 999px; background: #fff; color: #52695a; font-size: 12px; }
  .news-reactions button.active { border-color: #e3bcc4; background: #fff4f6; color: #a84358; }
  .news-own { margin: 12px 0 0; color: #8b9587; font-size: 12px; }
  .empty { padding: 30px 0; color: #818b7f; }
  @media (max-width: 600px) { .news-head { flex-direction: column; } }
</style>
