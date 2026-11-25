<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;
  let data = { initiated: [], participated: [], followed: [] };
  let activeTab = 'initiated';
  let loading = true;
  let error = '';

  const statusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };
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
    return data[tab] || [];
  }

  onMount(load);
</script>

<main class="dashboard my-projects">
  <div class="page-heading">
    <div><h1>{$t('我的项目')}<span class="heading-dot">.</span></h1><p>{$t('查看我发起、参与和关注的项目。')}</p></div>
    <a class="secondary" href="/showcase">{$t('项目大厅')}</a>
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
    {#if projectsFor(activeTab).length}
      <div class="project-list">
        {#each projectsFor(activeTab) as project}
          <article class="project-card">
            <div class="project-meta">
              <span class:done={project.status === 'completed'} class="project-status">{$t(statusLabels[project.status] || project.status)}</span>
              {#if project.difficulty}<span class={`project-difficulty difficulty-${project.difficulty}`}>{$t('难度')} {project.difficulty}</span>{/if}
              {#if activeTab === 'initiated'}<span class="review">{$t(reviewLabels[project.reviewStatus] || project.reviewStatus)}</span>{/if}
            </div>
            <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
            <p>{project.summary}</p>
            <p class="project-owner">{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {new Date(project.updated).toLocaleDateString()}{/if}</p>
          </article>
        {/each}
      </div>
    {:else}
      <p class="empty">{activeTab === 'initiated' ? $t('还没有发起项目。') : activeTab === 'participated' ? $t('还没有参与项目。') : $t('还没有关注项目。')}</p>
    {/if}
  {/if}
</main>

<style>
  .my-projects { width: min(1100px, 88%); }
  .my-projects .page-heading { margin-top: 0; align-items: center; gap: 20px; }
  .my-projects .page-heading .secondary { width: auto; min-height: 44px; padding: 0 18px; justify-content: center; gap: 0; white-space: nowrap; }
  .project-tabs { display: flex; flex-wrap: wrap; gap: 6px; margin: 10px 0 24px; padding-bottom: 8px; border-bottom: 1px solid #dcded4; }
  .project-tabs button { border: 1px solid transparent; background: none; padding: 7px 12px; color: #52695a; font-size: 13px; }
  .project-tabs button:hover { background: #eef3e8; }
  .project-tabs button.active { background: #274732; color: #fff; }
  .project-tabs button span { margin-left: 5px; opacity: 0.72; }
  .project-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 320px), 1fr)); gap: 16px; }
  .project-card { padding: 20px; border: 1px solid #dcded4; background: #fafbf7; }
  .project-card h2 { margin: 10px 0 8px; font-size: 20px; }
  .project-card h2 a { color: #274732; text-decoration: none; }
  .project-card p { margin: 0 0 8px; color: #6f7b6d; font-size: 13px; }
  .project-meta { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .project-status { display: inline-block; padding: 3px 9px; border: 1px solid #c8d4c0; border-radius: 999px; color: #3f5d3a; font-size: 12px; }
  .project-status.done { background: #e3eddd; }
  .project-difficulty { display: inline-block; padding: 3px 9px; border: 1px solid #d7d9cf; border-radius: 999px; font-size: 12px; font-weight: 600; }
  .difficulty-D { color: #6b6f66; background: #f1f2ed; }
  .difficulty-C { color: #3f5d3a; background: #eef3e8; }
  .difficulty-B { color: #285a78; background: #e8f4fb; }
  .difficulty-A { color: #7a4b12; background: #fff3df; }
  .difficulty-S { color: #7b2340; background: #fbe9ef; }
  .review { color: #818b7f; font-size: 12px; }
  .project-owner { margin-bottom: 0 !important; }
  .empty { padding: 30px 0; color: #818b7f; }
</style>
