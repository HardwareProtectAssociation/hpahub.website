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
  const statusLabels = { not_started: '未开始', ongoing: '已启动', long_term: '长期项目', paused: '暂停中', terminated: '已终止', completed: '已完成' };
  const statusFilters = ['not_started', 'ongoing', 'long_term', 'paused', 'completed', 'terminated'];
  const reviewLabels = { pending: '待审核', approved: '已通过', rejected: '已拒绝' };
  const natureLabels = { commercial: '商业盈利', learning: '学习交流', showcase: '技术展示', research: '科学研究' };
  const needOptions = [['funding', '资金支持'], ['technical', '技术支持'], ['equipment', '设备支持'], ['venue', '场地支持'], ['manpower', '人手支持'], ['promotion', '宣传支持'], ['emotional', '情绪支持']];
  const offerOptions = [['technical', '技术'], ['funding', '资金'], ['equipment', '设备'], ['venue', '场地'], ['manpower', '人手'], ['promotion', '宣传']];
  let manageProject = null;
  let editForm = null;
  let versions = [];
  let manageBusy = false;
  let manageError = '';
  let manageNotice = '';

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

  async function openManage(project) {
    manageError = '';
    manageNotice = '';
    versions = [];
    editForm = null;
    try {
      const [detail, versionList] = await Promise.all([
        request(`/api/hpahub/me/projects/${encodeURIComponent(project.id)}`),
        request(`/api/hpahub/me/projects/${encodeURIComponent(project.id)}/versions`),
      ]);
      editForm = {
        title: detail.title || '',
        summary: detail.summary || '',
        description: detail.description || '',
        projectNature: detail.projectNature || '',
        executionStatus: detail.executionStatus || 'ongoing',
        projectNeeds: detail.projectNeeds || [],
        providerOffers: detail.providerOffers || [],
      };
      versions = versionList.items || [];
      manageProject = project;
    } catch (err) {
      error = err.message || String(err);
    }
  }

  function closeManage() {
    manageProject = null;
    editForm = null;
    versions = [];
    manageError = '';
    manageNotice = '';
  }

  async function saveManage() {
    if (!manageProject || !editForm) return;
    manageBusy = true;
    manageError = '';
    manageNotice = '';
    try {
      const result = await request(`/api/hpahub/me/projects/${encodeURIComponent(manageProject.id)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(editForm),
      });
      manageNotice = $t('项目已保存为版本 {version}。', { version: result.version });
      manageProject = { ...manageProject, title: editForm.title, version: result.version };
      const versionList = await request(`/api/hpahub/me/projects/${encodeURIComponent(manageProject.id)}/versions`);
      versions = versionList.items || [];
      load();
    } catch (err) {
      manageError = err.message || String(err);
    } finally {
      manageBusy = false;
    }
  }

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
              {#if activeTab === 'initiated'}<span class="review">{$t(reviewLabels[project.reviewStatus] || project.reviewStatus)}</span><span class="version">{$t('版本')} {project.version || 1}</span>{/if}
            </div>
            <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
            <p>{project.summary}</p>
            <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：<RedactedText text={project.ownerName || '—'} hidden={!!project.ownerRedacted} />{#if project.updated} · {$t('更新于')} {new Date(project.updated).toLocaleDateString()}{/if}</p>
            {#if activeTab === 'initiated'}<div class="card-actions"><button type="button" class="manage-button" on:click={() => openManage(project)}>{$t('管理项目')}</button></div>{/if}
          </article>
        {/each}
      </div>
    {:else}
      <p class="empty">{statusFilter ? $t('该状态下暂无项目。') : activeTab === 'initiated' ? $t('还没有发起项目。') : activeTab === 'participated' ? $t('还没有参与项目。') : $t('还没有关注项目。')}</p>
    {/if}
  {/if}
</main>

{#if manageProject && editForm}
  <div class="manage-backdrop" role="presentation" on:click={(event) => { if (event.target === event.currentTarget) closeManage(); }}>
    <div class="manage-modal" role="dialog" aria-modal="true" aria-label={$t('管理项目')}>
      <div class="manage-head">
        <h2>{$t('管理项目')} · {manageProject.title}</h2>
        <button type="button" class="manage-close" aria-label={$t('关闭')} on:click={closeManage}>×</button>
      </div>
      {#if manageError}<p class="message error" role="alert">{$t(manageError)}</p>{/if}
      {#if manageNotice}<p class="message success" role="status">{$t(manageNotice)}</p>{/if}
      <form on:submit|preventDefault={saveManage}>
        <label>{$t('项目名称')}<input bind:value={editForm.title} maxlength="160" required /></label>
        <label>{$t('项目简介')}<textarea bind:value={editForm.summary} maxlength="2000" rows="2"></textarea></label>
        <label>{$t('项目详情')}<textarea bind:value={editForm.description} maxlength="10000" rows="4"></textarea></label>
        <div class="manage-grid">
          <label>{$t('项目状态')}<select bind:value={editForm.executionStatus}>{#each Object.entries(statusLabels) as [value, label]}<option {value}>{$t(label)}</option>{/each}</select></label>
          <label>{$t('项目性质')}<select bind:value={editForm.projectNature}><option value="">{$t('未填写')}</option>{#each Object.entries(natureLabels) as [value, label]}<option {value}>{$t(label)}</option>{/each}</select></label>
        </div>
        <fieldset class="manage-options"><legend>{$t('项目需要')}</legend>{#each needOptions as [value, label]}<label><input type="checkbox" bind:group={editForm.projectNeeds} value={value} />{$t(label)}</label>{/each}</fieldset>
        <fieldset class="manage-options"><legend>{$t('发起人可提供')}</legend>{#each offerOptions as [value, label]}<label><input type="checkbox" bind:group={editForm.providerOffers} value={value} />{$t(label)}</label>{/each}</fieldset>
        <div class="manage-actions"><button class="primary" type="submit" disabled={manageBusy}>{manageBusy ? $t('正在保存…') : $t('保存修改')}</button><button type="button" class="secondary" on:click={closeManage}>{$t('取消')}</button></div>
      </form>
      <section class="version-history">
        <h3>{$t('版本历史')}</h3>
        {#if versions.length}
          <ol>{#each versions as version}<li><strong>{$t('版本')} {version.version}</strong><span>{new Date(version.created).toLocaleString()}{#if version.editorName} · {version.editorName}{/if}</span></li>{/each}</ol>
        {:else}
          <p class="empty">{$t('暂无版本记录。')}</p>
        {/if}
      </section>
    </div>
  </div>
{/if}

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
  .version { color: var(--ink-subtle); font-family: var(--font-mono); font-size: 11.5px; }
  .card-actions { display: flex; gap: 8px; margin-top: 12px; }
  .manage-button { padding: 5px 12px; border: 1px solid var(--border-default); border-radius: 999px; background: transparent; color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: border-color var(--t-fast) var(--ease-out), background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .manage-button:hover { border-color: var(--accent); color: var(--accent-deep); background: var(--accent-soft); }
  .manage-backdrop { position: fixed; inset: 0; z-index: 60; display: grid; place-items: center; padding: 20px; background: #1c302266; }
  .manage-modal { width: min(720px, 100%); max-height: calc(100dvh - 40px); overflow-y: auto; padding: 24px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-md); }
  .manage-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
  .manage-head h2 { margin: 0; font-size: 19px; }
  .manage-close { border: 0; background: none; color: var(--ink-secondary); font-size: 26px; line-height: 1; cursor: pointer; }
  .manage-modal form { display: grid; gap: 14px; }
  .manage-modal label { display: grid; gap: 6px; color: var(--ink-secondary); font-size: 13px; }
  .manage-modal input:not([type="checkbox"]), .manage-modal select, .manage-modal textarea { width: 100%; padding: 9px 11px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .manage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 14px; }
  .manage-options { display: flex; flex-wrap: wrap; gap: 8px; margin: 0; padding: 10px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); }
  .manage-options legend { padding: 0 5px; color: var(--ink-secondary); font-size: 13px; }
  .manage-options label { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; padding: 4px 10px; border: 1px solid var(--border-default); border-radius: 999px; font-size: 12px; cursor: pointer; }
  .manage-options label:has(input:checked) { border-color: var(--accent); background: var(--accent-soft); color: var(--accent-deep); }
  .manage-actions { display: flex; gap: 10px; }
  .manage-actions .secondary { width: auto; min-height: 44px; padding: 0 18px; justify-content: center; }
  .version-history { margin-top: 20px; border-top: 1px solid var(--border-subtle); padding-top: 14px; }
  .version-history h3 { margin: 0 0 8px; font-size: 15px; }
  .version-history ol { margin: 0; padding: 0; list-style: none; display: grid; gap: 6px; }
  .version-history li { display: flex; justify-content: space-between; gap: 12px; padding: 6px 10px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-soft); font-size: 12px; }
  .version-history li span { color: var(--ink-subtle); }
</style>
