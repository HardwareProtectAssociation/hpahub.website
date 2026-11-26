<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { difficultyRank } from './tiers.js';

  export let token;
  let data = { initiated: [], participated: [], followed: [] };
  let activeTab = 'initiated';
  let statusFilter = '';
  let loading = true;
  let error = '';
  let showCreate = false;
  let creating = false;
  let newTitle = '';
  let newSummary = '';
  let newDifficulty = '';
  let newStyle = 'project';
  const difficultyLevels = ['D', 'C', 'B', 'A', 'S'];

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

  async function createProject() {
    if (!newTitle.trim()) return;
    creating = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/projects', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: newTitle.trim(),
          summary: newSummary.trim(),
          proposedDifficulty: newDifficulty || undefined,
          adventureStyle: newStyle,
        }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '创建失败，请重试。');
      showCreate = false;
      newTitle = '';
      newSummary = '';
      newDifficulty = '';
      newStyle = 'project';
      await load();
    } catch (err) {
      error = err.message || String(err);
    } finally {
      creating = false;
    }
  }

  onMount(load);
</script>

<main class="dashboard my-projects">
  <div class="page-heading">
    <div><h1>{$t('我的项目')}<span class="heading-dot">.</span></h1><p>{$t('查看我发起、参与和关注的项目。')}</p></div>
    <div class="heading-actions">
      <button type="button" class="primary" on:click={() => { showCreate = true; }}>{$t('发起项目')}</button>
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
              {#if project.difficulty}<span class={`rank rank-pill rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{/if}
              {#if activeTab === 'initiated'}<span class="review">{$t(reviewLabels[project.reviewStatus] || project.reviewStatus)}</span>{/if}
            </div>
            <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
            <p>{project.summary}</p>
            <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {new Date(project.updated).toLocaleDateString()}{/if}</p>
          </article>
        {/each}
      </div>
    {:else}
      <p class="empty">{statusFilter ? $t('该状态下暂无项目。') : activeTab === 'initiated' ? $t('还没有发起项目。') : activeTab === 'participated' ? $t('还没有参与项目。') : $t('还没有关注项目。')}</p>
    {/if}
  {/if}
</main>

{#if showCreate}
  <div class="create-modal-backdrop" on:click={() => { showCreate = false; }} on:keydown={(e) => { if (e.key === 'Escape') showCreate = false; }} role="presentation">
    <div class="create-modal" role="dialog" aria-modal="true" aria-label={$t('发起项目')} on:click|stopPropagation on:keydown|stopPropagation>
      <div class="create-modal-head">
        <h2>{$t('发起项目')}</h2>
        <button type="button" class="create-modal-close" aria-label={$t('关闭')} on:click={() => { showCreate = false; }}>×</button>
      </div>
      <form class="create-form" on:submit|preventDefault={createProject}>
        <label>{$t('项目名称')}<input bind:value={newTitle} maxlength="160" required placeholder={$t('输入项目名称')} /></label>
        <label>{$t('项目简介')}<textarea bind:value={newSummary} maxlength="2000" rows="3" placeholder={$t('简单描述项目目标和内容')}></textarea></label>
        <label>{$t('发起人自评难度')}<select bind:value={newDifficulty}><option value="">{$t('暂不设置')}</option>{#each difficultyLevels as level}<option value={level}>{level}</option>{/each}</select></label>
        <label>{$t('冒险风格')}<select bind:value={newStyle}><option value="project">{$t('项目')}</option><option value="dungeon">{$t('地牢')}</option><option value="boss">{$t('Boss')}</option></select></label>
        <div class="create-actions">
          <button type="submit" class="primary" disabled={creating}>{creating ? $t('正在创建…') : $t('创建项目')}</button>
          <button type="button" on:click={() => { showCreate = false; }}>{$t('取消')}</button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .my-projects { width: min(1100px, 88%); }
  .my-projects .page-heading { margin-top: 0; align-items: center; gap: 20px; }
  .my-projects .page-heading .secondary { width: auto; min-height: 44px; padding: 0 18px; justify-content: center; gap: 0; white-space: nowrap; }
  .project-tabs { display: flex; flex-wrap: wrap; gap: 6px; margin: 10px 0 24px; padding-bottom: 8px; border-bottom: 1px solid #dcded4; }
  .project-tabs button { border: 1px solid transparent; background: none; padding: 7px 12px; color: #52695a; font-size: 13px; }
  .project-tabs button:hover { background: #eef3e8; }
  .project-tabs button.active { background: #274732; color: #fff; }
  .project-tabs button span { margin-left: 5px; opacity: 0.72; }
  .status-filters { display: flex; flex-wrap: wrap; gap: 8px; margin: -12px 0 20px; }
  .status-filters button { padding: 4px 12px; border: 1px solid #c8d4c0; border-radius: 999px; background: none; color: #3f5d3a; font-size: 12px; cursor: pointer; }
  .status-filters button:hover { background: #eef3e8; }
  .status-filters button.active { background: #274732; border-color: #274732; color: #fff; }
  .project-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 320px), 1fr)); gap: 16px; }
  .project-card { padding: 20px; border: 1px solid #dcded4; background: #fafbf7; }
  .project-card h2 { margin: 10px 0 8px; font-size: 20px; }
  .project-card h2 a { color: #274732; text-decoration: none; }
  .project-card p { margin: 0 0 8px; color: #6f7b6d; font-size: 13px; }
  .project-meta { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .project-status { display: inline-block; padding: 3px 9px; border: 1px solid #c8d4c0; border-radius: 999px; color: #3f5d3a; font-size: 12px; }
  .project-status.done { background: #e3eddd; }
  .review { color: #818b7f; font-size: 12px; }
  .project-owner { margin-bottom: 0 !important; }
  .owner-avatar { width: 22px; height: 22px; margin-right: 6px; border-radius: 50%; object-fit: cover; vertical-align: middle; background: #dce8d2; }
  .empty { padding: 30px 0; color: #818b7f; }
  .heading-actions { display: flex; gap: 10px; align-items: center; }
  .heading-actions .primary { min-height: 44px; padding: 0 18px; }
  .create-modal-backdrop { position: fixed; inset: 0; z-index: 50; display: grid; place-items: center; padding: 20px; background: #1c302266; }
  .create-modal { width: min(520px, 100%); max-height: calc(100dvh - 40px); overflow-y: auto; padding: 24px; border: 1px solid #dcded4; border-radius: 8px; background: #fbfcf8; box-shadow: 0 20px 50px #1c302233; }
  .create-modal-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
  .create-modal-head h2 { margin: 0; font-size: 20px; }
  .create-modal-close { border: 0; background: none; color: #52695a; font-size: 26px; line-height: 1; padding: 0 4px; cursor: pointer; }
  .create-form { display: grid; gap: 12px; }
  .create-form label { display: grid; gap: 5px; color: #52695a; font-size: 13px; }
  .create-form input, .create-form select, .create-form textarea { width: 100%; padding: 9px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .create-form textarea { resize: vertical; }
  .create-actions { display: flex; gap: 10px; margin-top: 8px; }
</style>
