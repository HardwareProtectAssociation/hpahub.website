<script>
  import RedactedText from './RedactedText.svelte';
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { difficultyRank } from './tiers.js';
  import Select from './Select.svelte';

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
  const offerOptions = [['technical', '技术'], ['funding', '资金'], ['equipment', '设备'], ['venue', '场地'], ['manpower', '人手'], ['promotion', '宣传'], ['title', '头衔认证']];
  let manageProject = null;
  let editForm = null;
  let versions = [];
  let manageBusy = false;
  let manageError = '';
  let manageNotice = '';
  let canEditProject = true;
  let nextProjectEditAt = '';
  let coverFile = null;
  let coverPreview = '';
  let removeCover = false;
  let versionDetail = null;
  let versionBusy = '';
  let versionError = '';
  const needLabelMap = Object.fromEntries(needOptions);
  const offerLabelMap = Object.fromEntries(offerOptions);
  const versionFieldLabels = {
    title: '项目名称', summary: '项目简介', collaborationMode: '协作方式',
    joinMode: '加入方式', acceptsSponsorship: '接受赞助', allowOthersManagement: '允许他人管理',
    confidentialityRequested: '需要保密', confidentialityTerms: '保密协议内容', estimatedDurationDays: '预计周期（天）',
    longTerm: '长期项目', executionStatus: '项目状态', plannedStartDate: '计划开始日期', scheduleNote: '时间安排',
    repeatable: '重复项目', repeatLimit: '每位参与者最多完成次数', sponsor: '赞助者',
    proposedDifficulty: '项目级别', dangerLevel: '危险性', taskDifficulty: '难度', adventureStyle: '冒险风格', projectNature: '项目性质', projectNeeds: '项目需要',
    providerOffers: '发起人可提供', projectNeedDetails: '需求说明', providerOfferDetails: '提供说明', statusReason: '状态原因',
  };

  function formatVersionValue(field, value) {
    if (value === null || value === undefined || value === '') return $t('无');
    if (field === 'executionStatus') return $t(statusLabels[value] || value);
    if (field === 'projectNature') return $t(natureLabels[value] || value);
    if (field === 'projectNeeds' && Array.isArray(value)) return value.map((item) => $t(needLabelMap[item] || item)).join(' · ') || $t('无');
    if (field === 'providerOffers' && Array.isArray(value)) return value.map((item) => $t(offerLabelMap[item] || item)).join(' · ') || $t('无');
    if (field === 'projectNeeds') return $t(needLabelMap[value] || value);
    if (field === 'providerOffers') return $t(offerLabelMap[value] || value);
    if (typeof value === 'boolean') return $t(value ? '是' : '否');
    if (typeof value === 'object') {
      return Object.entries(value).map(([key, item]) => `${$t(needLabelMap[key] || offerLabelMap[key] || key)}：${item}`).join(' · ') || $t('无');
    }
    return String(value);
  }

  async function request(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: { Authorization: `Bearer ${token}`, ...(options.headers || {}) },
      cache: 'no-store',
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(body.message || `请求失败（${response.status}）`);
      error.status = response.status;
      error.body = body;
      throw error;
    }
    return body;
  }

  async function openManage(project) {
    manageError = '';
    manageNotice = '';
    versions = [];
    editForm = null;
    coverFile = null;
    removeCover = false;
    if (coverPreview) URL.revokeObjectURL(coverPreview);
    coverPreview = '';
    versionDetail = null;
    versionError = '';
    versionBusy = '';
    try {
      const [detail, versionList] = await Promise.all([
        request(`/api/hpahub/me/projects/${encodeURIComponent(project.id)}`),
        request(`/api/hpahub/me/projects/${encodeURIComponent(project.id)}/versions`),
      ]);
      editForm = {
        title: detail.title || '',
        sponsor: detail.sponsor || '',
        dangerLevel: detail.dangerLevel || '',
        taskDifficulty: detail.taskDifficulty || '',
        summary: detail.summary || '',
        description: detail.description || '',
        projectNature: detail.projectNature || '',
        executionStatus: detail.executionStatus || 'ongoing',
        repeatable: detail.repeatable || false,
        repeatLimit: detail.repeatLimit || 1,
        statusReason: detail.statusReason || '',
        projectNeeds: detail.projectNeeds || [],
        providerOffers: detail.providerOffers || [],
        projectNeedDetails: detail.projectNeedDetails || {},
        providerOfferDetails: detail.providerOfferDetails || {},
        coverUrl: detail.coverUrl || '',
      };
      versions = versionList.items || [];
      manageProject = project;
      canEditProject = detail.canEdit !== false;
      nextProjectEditAt = detail.nextEditAt || '';
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
    coverFile = null;
    removeCover = false;
    if (coverPreview) URL.revokeObjectURL(coverPreview);
    coverPreview = '';
    versionDetail = null;
    versionError = '';
    versionBusy = '';
    canEditProject = true;
    nextProjectEditAt = '';
  }

  function selectCover(event) {
    const file = event.currentTarget.files?.[0] || null;
    if (file && file.size > 2 * 1024 * 1024) {
      manageError = '封面不能超过 2 MiB。';
      event.currentTarget.value = '';
      return;
    }
    manageError = '';
    if (coverPreview) URL.revokeObjectURL(coverPreview);
    coverFile = file;
    coverPreview = file ? URL.createObjectURL(file) : '';
    if (file) removeCover = false;
  }

  function toggleRemoveCover() {
    if (removeCover) {
      coverFile = null;
      if (coverPreview) URL.revokeObjectURL(coverPreview);
      coverPreview = '';
    }
  }

  async function openVersion(version) {
    versionBusy = String(version.version);
    versionError = '';
    versionDetail = null;
    try {
      versionDetail = await request(`/api/hpahub/me/projects/${encodeURIComponent(manageProject.id)}/versions/${version.version}`);
    } catch (err) {
      versionError = err.message || String(err);
    } finally {
      versionBusy = '';
    }
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
      let coverError = '';
      if (removeCover) {
        await request(`/api/hpahub/me/projects/${encodeURIComponent(manageProject.id)}/cover`, { method: 'DELETE' });
        editForm = { ...editForm, coverUrl: '' };
      } else if (coverFile) {
        const body = new FormData();
        body.set('cover', coverFile);
        try {
          const cover = await request(`/api/hpahub/me/projects/${encodeURIComponent(manageProject.id)}/cover`, { method: 'POST', body });
          editForm = { ...editForm, coverUrl: cover.coverUrl || '' };
        } catch {
          coverError = '项目内容已保存，但封面更新失败。';
        }
        coverFile = null;
        if (coverPreview) URL.revokeObjectURL(coverPreview);
        coverPreview = '';
      }
      removeCover = false;
      manageNotice = coverError ? '' : $t('项目已保存为版本 {version}。', { version: result.version });
      manageError = coverError;
      canEditProject = false;
      nextProjectEditAt = result.nextProjectEditAt || '';
      manageProject = { ...manageProject, title: editForm.title, version: result.version };
      const versionList = await request(`/api/hpahub/me/projects/${encodeURIComponent(manageProject.id)}/versions`);
      versions = versionList.items || [];
      load();
    } catch (err) {
      if (err.status === 429) {
        canEditProject = false;
        nextProjectEditAt = err.body?.nextProjectEditAt || '';
        manageError = '项目信息每天只能修改一次';
      } else {
        manageError = err.message || String(err);
      }
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
            {#if project.difficulty}<span class={`difficulty-star star-${difficultyRank(project.difficulty)}`} title={`${$t('项目级别')} ${project.difficulty}`} aria-label={`${$t('项目级别')} ${project.difficulty}`}>{project.difficulty}</span>{:else}<span class="difficulty-star star-0" title={$t('项目级别')} aria-label={$t('项目级别')}>?</span>{/if}
            <div class="project-meta">
              <span class:done={project.status === 'completed'} class="project-status">{$t(statusLabels[project.status] || project.status)}</span>
              {#if activeTab === 'initiated'}<span class="review">{$t(reviewLabels[project.reviewStatus] || project.reviewStatus)}</span><span class="version">{$t('版本')} {project.version || 1}</span>{/if}
            </div>
            <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
            {#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
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
      {#if !canEditProject}<p class="message">{$t('项目信息每天只能修改一次')}{#if nextProjectEditAt} · {$t('下次可修改：{time}', { time: new Date(nextProjectEditAt).toLocaleString() })}{/if}</p>{/if}
      <form on:submit|preventDefault={saveManage}>
        <label>{$t('项目名称')}<input bind:value={editForm.title} maxlength="160" required /></label>
        <label>{$t('项目简介')}<textarea bind:value={editForm.summary} maxlength="2000" rows="4" placeholder={$t('介绍项目目标、内容或参与方式')} /></label>
        <label>{$t('赞助者')}<input bind:value={editForm.sponsor} maxlength="500" placeholder={$t('可填写赞助者姓名或组织名称')} /></label>
        <div class="manage-cover">
          <div class="cover-preview-wrap">
            {#if coverPreview && !removeCover}<img class="cover-image" src={coverPreview} alt={$t('封面预览')} />
            {:else if editForm.coverUrl && !removeCover}<img class="cover-image" src={editForm.coverUrl} alt={$t('当前封面')} />
            {:else}<span class="cover-empty">{$t('暂无封面')}</span>{/if}
          </div>
          <div class="cover-controls">
            <label class="cover-file">{$t('更换封面')}<input type="file" accept="image/jpeg,image/png,image/webp,image/gif" on:change={selectCover} /></label>
            {#if (editForm.coverUrl || coverPreview) && !removeCover}<label class="cover-remove"><input type="checkbox" bind:checked={removeCover} on:change={toggleRemoveCover} />{$t('删除封面')}</label>{/if}
          </div>
        </div>
        <div class="manage-grid">
          <div class="select-field"><span>{$t('项目状态')}</span><Select bind:value={editForm.executionStatus} ariaLabel={$t('项目状态')} options={Object.entries(statusLabels).filter(([value]) => !editForm.repeatable || value !== 'completed').map(([value, label]) => ({ value, label: $t(label) }))} /></div>
          <div class="select-field"><span>{$t('项目性质')}</span><Select bind:value={editForm.projectNature} ariaLabel={$t('项目性质')} options={[{ value: '', label: $t('未填写') }, ...Object.entries(natureLabels).map(([value, label]) => ({ value, label: $t(label) }))]} /></div>
          <div class="select-field"><span>{$t('危险性')}</span><Select bind:value={editForm.dangerLevel} ariaLabel={$t('危险性')} options={[{ value: '', label: $t('未填写') }, ...[['low', '低'], ['medium', '中'], ['high', '高'], ['extreme', '极高']].map(([value, label]) => ({ value, label: $t(label) }))]} /></div>
          <div class="select-field"><span>{$t('难度')}</span><Select bind:value={editForm.taskDifficulty} ariaLabel={$t('难度')} options={[{ value: '', label: $t('未填写') }, ...[['easy', '简单'], ['normal', '一般'], ['hard', '困难'], ['extreme', '极难']].map(([value, label]) => ({ value, label: $t(label) }))]} /></div>
        </div>
        <fieldset class="manage-options"><legend>{$t('重复项目')}</legend><label><input type="checkbox" bind:checked={editForm.repeatable} on:change={() => { if (editForm.repeatable && editForm.executionStatus === 'completed') editForm.executionStatus = 'ongoing'; }} />{$t('允许每位参与者重复完成任务')}</label>{#if editForm.repeatable}<label>{$t('每位参与者最多完成次数')}<input type="number" min="1" max="9999" step="1" bind:value={editForm.repeatLimit} required /></label><p class="muted">{$t('每次完成都需提交申请，经项目管理者通过后计入个人完成项目。项目本身保持进行中。')}</p>{/if}</fieldset>
        {#if ['paused', 'terminated'].includes(editForm.executionStatus)}<label>{$t('暂停或中止原因')}<textarea bind:value={editForm.statusReason} maxlength="2000" rows="3" required></textarea></label>{/if}
        <fieldset class="manage-options"><legend>{$t('项目需要')}</legend>{#each needOptions as [value, label]}<div class="manage-option"><label><input type="checkbox" bind:group={editForm.projectNeeds} value={value} />{$t(label)}</label>{#if editForm.projectNeeds.includes(value)}<label class="detail-input">{$t(label)} · {$t('具体补充（可选）')}<textarea rows="2" maxlength="500" bind:value={editForm.projectNeedDetails[value]}></textarea></label>{/if}</div>{/each}</fieldset>
        <fieldset class="manage-options"><legend>{$t('发起人可提供')}</legend>{#each offerOptions as [value, label]}<div class="manage-option"><label><input type="checkbox" bind:group={editForm.providerOffers} value={value} />{$t(label)}</label>{#if editForm.providerOffers.includes(value)}<label class="detail-input">{#if value === 'title'}{$t('头衔名称')}<input bind:value={editForm.providerOfferDetails[value]} maxlength="10" required placeholder="最多10个字" />{:else}{$t(label)} · {$t('具体补充（可选）')}<textarea rows="2" maxlength="500" bind:value={editForm.providerOfferDetails[value]}></textarea>{/if}</label>{/if}</div>{/each}</fieldset>
        <div class="manage-actions"><button class="primary" type="submit" disabled={manageBusy || !canEditProject}>{manageBusy ? $t('正在保存…') : $t('保存修改')}</button><button type="button" class="secondary" on:click={closeManage}>{$t('取消')}</button></div>
      </form>
      <section class="version-history">
        <h3>{$t('版本历史')}</h3>
        {#if versions.length}
          <ol>{#each versions as version}<li><strong>{$t('版本')} {version.version}</strong><span>{new Date(version.created).toLocaleString()}{#if version.editorName} · {version.editorName}{/if}</span><button type="button" class="version-view" on:click={() => openVersion(version)} disabled={versionBusy === String(version.version)}>{$t('查看')}</button></li>{/each}</ol>
        {:else}
          <p class="empty">{$t('暂无版本记录。')}</p>
        {/if}
        {#if versionError}<p class="message error" role="alert">{$t(versionError)}</p>{/if}
        {#if versionDetail?.changes}
          <div class="version-detail">
            <div class="version-detail-head"><strong>{$t('版本')} {versionDetail.version}</strong><span>{new Date(versionDetail.created).toLocaleString()}{#if versionDetail.editorName} · {versionDetail.editorName}{/if}</span></div>
            {#if versionDetail.isInitial}
              <p>{$t('初始版本：项目创建时的基础内容。')}</p>
            {:else if versionDetail.changes.length}
              <dl>{#each versionDetail.changes as change}
                <div><dt>{$t(versionFieldLabels[change.field] || change.field)}{#if change.detailKey} · {$t(needLabelMap[change.detailKey] || offerLabelMap[change.detailKey] || change.detailKey)}{/if}</dt><dd><span>{formatVersionValue(change.field, change.before)}</span><span aria-hidden="true"> → </span><strong>{formatVersionValue(change.field, change.after)}</strong></dd></div>
              {/each}</dl>
            {:else}
              <p>{$t('此版本没有可显示的内容差异。')}</p>
            {/if}
          </div>
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
  .manage-modal input:not([type="checkbox"]), .manage-modal textarea { width: 100%; padding: 9px 11px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .manage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 14px; }
  .manage-cover { display: grid; gap: 10px; }
  .cover-preview-wrap { display: flex; align-items: center; justify-content: center; min-height: 120px; border: 1px dashed var(--border-default); border-radius: var(--radius-sm); background: var(--surface-soft); overflow: hidden; }
  .cover-image { display: block; width: 100%; max-height: 220px; object-fit: cover; }
  .cover-empty { color: var(--ink-subtle); font-size: 13px; }
  .cover-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 14px; }
  .cover-file { display: inline-flex !important; flex-direction: row; align-items: center; gap: 8px; color: var(--accent-deep) !important; font-size: 13px; cursor: pointer; white-space: nowrap; }
  .cover-remove { display: inline-flex !important; flex-direction: row; align-items: center; gap: 6px; color: var(--ink-secondary) !important; font-size: 13px; white-space: nowrap; }
  .manage-options { display: flex; flex-wrap: wrap; gap: 8px; margin: 0; padding: 10px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); }
  .manage-options legend { padding: 0 5px; color: var(--ink-secondary); font-size: 13px; }
  .manage-options label { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; padding: 4px 10px; border: 1px solid var(--border-default); border-radius: 999px; font-size: 12px; cursor: pointer; }
  .manage-option:has(.detail-input) { flex: 1 1 100%; }
  .manage-options .detail-input { display: grid; margin-top: 8px; padding: 0; border: 0; border-radius: 0; cursor: text; }
  .manage-options label:has(input:checked) { border-color: var(--accent); background: var(--accent-soft); color: var(--accent-deep); }
  .manage-actions { display: flex; gap: 10px; }
  .manage-actions .secondary { width: auto; min-height: 44px; padding: 0 18px; justify-content: center; }
  .version-history { margin-top: 20px; border-top: 1px solid var(--border-subtle); padding-top: 14px; }
  .version-history h3 { margin: 0 0 8px; font-size: 15px; }
  .version-history ol { margin: 0; padding: 0; list-style: none; display: grid; gap: 6px; }
  .version-history li { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 10px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-soft); font-size: 12px; }
  .version-history li span { color: var(--ink-subtle); }
  .version-view { flex: none; padding: 2px 10px; border: 1px solid var(--border-default); border-radius: 999px; background: transparent; color: var(--ink-secondary); font-size: 12px; cursor: pointer; }
  .version-view:hover { border-color: var(--accent); color: var(--accent-deep); background: var(--accent-soft); }
  .version-detail { margin-top: 12px; padding: 12px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-soft); }
  .version-detail-head { display: flex; justify-content: space-between; gap: 10px; margin-bottom: 8px; color: var(--ink-subtle); font-size: 12px; }
  .version-detail dl { margin: 0; display: grid; gap: 6px; }
  .version-detail dl > div { display: grid; grid-template-columns: 90px minmax(0, 1fr); gap: 10px; font-size: 12px; }
  .version-detail dt { color: var(--ink-subtle); }
  .version-detail dd { margin: 0; color: var(--ink-secondary); white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
