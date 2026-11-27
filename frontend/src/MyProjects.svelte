<script>
  import RedactedText from './RedactedText.svelte';
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { difficultyRank } from './tiers.js';

  export let token;
  let data = { initiated: [], participated: [], followed: [] };
  let activeTab = 'initiated';
  let statusFilter = '';
  let loading = true;
  let error = '';
  const statusLabels = { ongoing: '进行中', paused: '已暂停', terminated: '已终止', completed: '已完成' };
  const statusFilters = ['ongoing', 'completed', 'paused', 'terminated'];
  const reviewLabels = { pending: '待审核', approved: '已通过', rejected: '已拒绝' };

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/projects', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        cache: 'no-store',
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || `请求失败（${response.status}）`);
      data = { initiated: body.initiated || [], participated: body.participated || [], followed: body.followed || [] };
      const first = [['initiated', data.initiated], ['participated', data.participated], ['followed', data.followed]].find(([, items]) => items.length > 0);
      activeTab = first?.[0] || 'initiated';
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  }

  function projectsFor(tab) {
    const items = data[tab] || [];
    return statusFilter ? items.filter((project) => project.status === statusFilter) : items;
  }

  onMount(load);
</script>

<main class="dashboard my-projects">
  <div class="page-heading">
    <div><h1>{$t('我的项目')}<span class="heading-dot">.</span></h1><p>{$t('查看我发起、参与和关注的项目。')}</p></div>
    <div class="heading-actions">
      <a class="primary create-link" href="/my-projects/new">{$t('发起项目')}</a>
      <a class="secondary" href="/showcase">{$t('项目大厅')}</a>
    </div>
  </div>
  {#if loading}
    <p role="status">{$t('正在加载…')}</p>
  {:else if error}
    <p class="message error" role="alert">{$t(error)}</p>
    <button class="secondary" type="button" on:click={load}>{$t('重试')}</button>
  {:else}
    <nav class="project-tabs" aria-label={$t('我的项目分类')}>
      <button type="button" class:active={activeTab === 'initiated'} on:click={() => activeTab = 'initiated'}>{$t('我发起的项目')}<span>{data.initiated.length}</span></button>
      <button type="button" class:active={activeTab === 'participated'} on:click={() => activeTab = 'participated'}>{$t('我参与的项目')}<span>{data.participated.length}</span></button>
      <button type="button" class:active={activeTab === 'followed'} on:click={() => activeTab = 'followed'}>{$t('我关注的项目')}<span>{data.followed.length}</span></button>
    </nav>
    <div class="status-filters" role="group" aria-label={$t('执行状态')}>
      {#each statusFilters as status}
        <button type="button" class:active={statusFilter === status} aria-pressed={statusFilter === status} on:click={() => statusFilter = statusFilter === status ? '' : status}>{$t(statusLabels[status])}</button>
      {/each}
    </div>
    {#if projectsFor(activeTab).length}
      <div class="project-list">
        {#each projectsFor(activeTab) as project}
          <article class="project-card">
            <div class="project-meta">
              <span class:done={project.status === 'completed'} class="project-status">{$t(statusLabels[project.status] || project.status)}</span>
              {#if project.reviewStatus === 'approved' && project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{:else}<span class="rank-pending">{$t('难度')} {$t('待定')}</span>{/if}
              {#if activeTab === 'initiated'}<span class="review">{$t(reviewLabels[project.reviewStatus] || project.reviewStatus)}</span>{/if}
            </div>
            <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
            <p>{project.summary}</p>
            <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：<RedactedText text={project.ownerName || '—'} hidden={!!project.ownerRedacted} />{#if project.updated} · {$t('更新于')} {new Date(project.updated).toLocaleDateString()}{/if}</p>
          </article>
        {/each}
      </div>
    {:else}
      <p class="empty">{statusFilter ? $t('该状态下暂无项目。') : activeTab === 'initiated' ? $t('还没有发起项目。') : activeTab === 'participated' ? $t('还没有参与项目。') : $t('还没有关注项目。')}</p>
    {/if}
  {/if}
</main>

<style>
  .my-projects { width: min(1100px, 88%); }
  .my-projects .page-heading { margin-top: 0; align-items: center; gap: 20px; }
  .my-projects .page-heading .secondary { width: auto; min-height: 44px; padding: 0 18px; justify-content: center; gap: 0; white-space: nowrap; }
  .project-tabs { display: flex; flex-wrap: wrap; gap: 6px; margin: 10px 0 24px; padding-bottom: 10px; border-bottom: 1px solid var(--border-subtle); }
  .project-tabs button { border: 1px solid transparent; border-radius: var(--radius-sm); background: none; padding: 8px 14px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; transition: background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .project-tabs button:hover { background: var(--surface-tint); color: var(--accent-deep); }
  .project-tabs button.active { background: linear-gradient(135deg, var(--accent), var(--accent-deep)); color: #fff; box-shadow: var(--shadow-glow); }
  .project-tabs button span { margin-left: 6px; opacity: 0.78; font-family: var(--font-mono); font-size: 11.5px; }
  .status-filters { display: flex; flex-wrap: wrap; gap: 8px; margin: -12px 0 20px; }
  .status-filters button { padding: 5px 13px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-elevated); color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out); }
  .status-filters button:hover { background: var(--surface-tint); border-color: var(--border-strong); }
  .status-filters button.active { background: var(--accent); border-color: var(--accent); color: #fff; }
  .project-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 320px), 1fr)); gap: 16px; }
  .project-card { padding: 20px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); transition: transform var(--t-med) var(--ease-out), border-color var(--t-med) var(--ease-out), box-shadow var(--t-med) var(--ease-out); }
  .project-card:hover { transform: translateY(-2px); border-color: var(--border-strong); box-shadow: var(--shadow-sm); }
  .project-card h2 { margin: 10px 0 8px; font-size: 20px; }
  .project-card h2 a { color: var(--ink-primary); text-decoration: none; transition: color var(--t-fast) var(--ease-out); }
  .project-card:hover h2 a { color: var(--accent-deep); }
  .project-card p { margin: 0 0 8px; color: var(--ink-muted); font-size: 13px; }
  .project-meta { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .project-status { display: inline-block; padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-soft); color: var(--ink-secondary); font-size: 12px; }
  .project-status.done { background: var(--surface-tint); border-color: var(--border-strong); color: var(--accent-deep); }
  .review { color: var(--ink-subtle); font-size: 12px; }
  .project-owner { margin-bottom: 0 !important; }
  .owner-avatar { width: 22px; height: 22px; margin-right: 6px; border-radius: 50%; object-fit: cover; vertical-align: middle; background: var(--surface-tint); border: 1px solid var(--border-subtle); }
  .empty { padding: 30px 0; color: var(--ink-subtle); }
  .heading-actions { display: flex; gap: 10px; align-items: center; }
  .heading-actions .primary { min-height: 44px; padding: 0 18px; }
  .create-link { display: inline-flex; align-items: center; text-decoration: none; }
</style>
