<script>
  import { onDestroy } from 'svelte';
  import { t } from './i18n.js';
  import Select from './Select.svelte';
  export let token;
  let title = '', summary = '', description = '', difficulty = '', nature = '';
  let collaboration = 'collaborative', joinMode = 'open', sponsorship = false;
  let startDate = '', duration = '', scheduleNote = '', error = '', creating = false;
  let executionStatus = 'not_started', allowOthersManagement = false, confidentialityRequested = false, confidentialityTerms = '';
  let statusReason = '';  let cover = null, coverPreview = '', createdId = '';
  let projectNeeds = [];
  let providerOffers = [];
  const needOptions = [['funding', '资金支持'], ['technical', '技术支持'], ['equipment', '设备支持'], ['venue', '场地支持'], ['manpower', '人手支持'], ['promotion', '宣传支持'], ['emotional', '情绪支持']];
  const offerOptions = [['technical', '技术'], ['funding', '资金'], ['equipment', '设备'], ['venue', '场地'], ['manpower', '人手'], ['promotion', '宣传']];

  async function createProject() {
    if (!title.trim() || creating) return;
    if (!nature) { error = '请选择项目性质。'; return; }
    creating = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/projects', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: title.trim(), summary: summary.trim(), description: description.trim(), proposedDifficulty: difficulty, projectNature: nature, projectNeeds, providerOffers, collaborationMode: collaboration, joinMode, acceptsSponsorship: sponsorship, allowOthersManagement, confidentialityRequested, confidentialityTerms: confidentialityRequested ? confidentialityTerms.trim() : '', plannedStartDate: executionStatus === 'completed' ? '' : startDate, estimatedDurationDays: !['completed', 'long_term'].includes(executionStatus) && duration ? Number(duration) : 0, longTerm: executionStatus === 'long_term', executionStatus, statusReason: statusReason.trim(), scheduleNote: scheduleNote.trim() }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '创建失败，请重试。');
      if (cover) {
        const body = new FormData();
        body.set('cover', cover);
        const upload = await fetch(`/api/hpahub/me/projects/${encodeURIComponent(result.id)}/cover`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
          body,
        });
        if (!upload.ok) {
          createdId = result.id;
          const failure = await upload.json().catch(() => ({}));
          throw new Error(failure.message || '项目已创建，但封面上传失败。');
        }
      }
      location.assign('/showcase/' + encodeURIComponent(result.id));
    } catch (err) {
      error = err.message || String(err);
      creating = false;
    }
  }
  function selectCover(event) {
    const file = event.currentTarget.files?.[0] || null;
    if (file && file.size > 2 * 1024 * 1024) {
      error = '封面不能超过 2 MiB。';
      event.currentTarget.value = '';
      return;
    }
    error = '';
    if (coverPreview) URL.revokeObjectURL(coverPreview);
    cover = file;
    coverPreview = file ? URL.createObjectURL(file) : '';
  }
  onDestroy(() => { if (coverPreview) URL.revokeObjectURL(coverPreview); });
</script>

<main class="dashboard create-project">
  <div class="page-heading"><div><h1>{$t('发起项目')}<span class="heading-dot">.</span></h1></div></div>
  <form on:submit|preventDefault={createProject}>
    <fieldset class="needs"><legend>{$t('项目需要')}</legend>
      {#each needOptions as [value, label]}
        <label><input type="checkbox" bind:group={projectNeeds} value={value} />{$t(label)}</label>
      {/each}
    </fieldset>
    <fieldset class="needs"><legend>{$t('发起人可提供')}</legend>
      {#each offerOptions as [value, label]}
        <label><input type="checkbox" bind:group={providerOffers} value={value} />{$t(label)}</label>
      {/each}
    </fieldset>
    <label>{$t('项目名称')}<input bind:value={title} maxlength="160" required /></label>
    <label>{$t('项目简介')}<textarea bind:value={summary} maxlength="2000" rows="3"></textarea></label>
    <label>{$t('项目详情')}<textarea bind:value={description} maxlength="10000" rows="7"></textarea></label>
    <label>{$t('项目封面（可选，最多 2 MiB）')}<input type="file" accept="image/jpeg,image/png,image/webp,image/gif" on:change={selectCover} /></label>
    {#if coverPreview}<img class="cover-preview" src={coverPreview} alt={$t('封面预览')} />{/if}

    <div class="fields">
      <div class="select-field"><span>{$t('项目状态')}</span><Select bind:value={executionStatus} ariaLabel={$t('项目状态')} options={[['not_started', '未开始'], ['ongoing', '已启动'], ['long_term', '长期项目'], ['paused', '暂停中'], ['completed', '已完成'], ['terminated', '已中止']].map(([value, label]) => ({ value, label: $t(label) }))} /></div>
      <div class="select-field"><span>{$t('发起人自评难度')}</span><Select bind:value={difficulty} ariaLabel={$t('发起人自评难度')} options={[{ value: '', label: $t('暂不设置') }, ...['D', 'C', 'B', 'A', 'S'].map((value) => ({ value, label: value }))]} /></div>
      <div class="select-field"><span>{$t('项目性质')}</span><Select bind:value={nature} ariaLabel={$t('项目性质')} options={[{ value: '', label: $t('请选择') }, ...[['commercial', '商业盈利'], ['learning', '学习交流'], ['showcase', '技术展示'], ['research', '科学研究']].map(([value, label]) => ({ value, label: $t(label) }))]} /></div>
      <div class="select-field"><span>{$t('协作方式')}</span><Select bind:value={collaboration} ariaLabel={$t('协作方式')} options={[{ value: 'collaborative', label: $t('协作项目') }, { value: 'independent', label: $t('独立项目') }]} /></div>
      {#if collaboration === 'collaborative'}<div class="select-field"><span>{$t('加入方式')}</span><Select bind:value={joinMode} ariaLabel={$t('加入方式')} options={[{ value: 'open', label: $t('自由加入') }, { value: 'approval', label: $t('申请加入') }]} /></div>{/if}
      {#if executionStatus !== 'completed'}<label>{$t('计划开始日期')}<input type="date" bind:value={startDate} /></label>{/if}
      {#if !['completed', 'long_term'].includes(executionStatus)}<label>{$t('预计周期（天）')}<input type="number" min="1" max="36500" step="1" bind:value={duration} /></label>{/if}
    </div>
    {#if ['paused', 'terminated'].includes(executionStatus)}<label>{$t('暂停或中止原因')}<textarea bind:value={statusReason} maxlength="2000" rows="3" required></textarea></label>{/if}
    <label>{$t('时间安排')}<textarea bind:value={scheduleNote} maxlength="2000" rows="3"></textarea></label>
    <fieldset class="arrangements"><legend>{$t('项目安排')}</legend>
      <div class="arrangement-row"><span>{$t('接受赞助')}</span><span class="choices" role="group" aria-label={$t('接受赞助')}><label><input type="radio" bind:group={sponsorship} value={true} />{$t('是')}</label><label><input type="radio" bind:group={sponsorship} value={false} />{$t('否')}</label></span></div>
      <div class="arrangement-row"><span>{$t('允许他人管理')}</span><span class="choices" role="group" aria-label={$t('允许他人管理')}><label><input type="radio" bind:group={allowOthersManagement} value={true} />{$t('是')}</label><label><input type="radio" bind:group={allowOthersManagement} value={false} />{$t('否')}</label></span></div>
      <div class="arrangement-row"><span>{$t('需要保密')}</span><span class="choices" role="group" aria-label={$t('需要保密')}><label><input type="radio" bind:group={confidentialityRequested} value={true} />{$t('是')}</label><label><input type="radio" bind:group={confidentialityRequested} value={false} />{$t('否')}</label></span></div>
    </fieldset>
    {#if confidentialityRequested}<label>{$t('保密协议')}<textarea bind:value={confidentialityTerms} maxlength="10000" rows="6" required></textarea></label>{/if}
    {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
    <div class="actions"><button class="primary" type="submit" disabled={creating}>{$t(creating ? '正在创建…' : '创建项目')}</button>{#if createdId}<a class="secondary" href={'/showcase/' + encodeURIComponent(createdId)}>{$t('前往项目')}</a>{/if}<a class="secondary" href="/my-projects">{$t('取消')}</a></div>
  </form>
</main>

<style>
  .create-project { width: min(880px, 88%); }
  .cover-preview { display: block; max-width: 100%; max-height: 260px; object-fit: cover; border: 1px solid var(--border-default); border-radius: var(--radius-sm); }
  form { display: grid; gap: 20px; padding: 28px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); }
  label { display: grid; gap: 8px; color: var(--ink-secondary); font-size: 14px; }
  input:not([type="checkbox"]), textarea { width: 100%; padding: 11px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  input:focus, textarea:focus { border-color: var(--accent); outline: 2px solid var(--accent-glow); }
  textarea { resize: vertical; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 20px; }
  .arrangements { display: grid; gap: 10px; margin: 0; padding: 14px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); }
  .arrangements legend { padding: 0 5px; color: var(--ink-secondary); font-size: 14px; }
  .arrangement-row { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px 20px; }
  .arrangement-row > span:first-child { color: var(--ink-secondary); font-size: 14px; }
  .choices { display: inline-flex; gap: 8px; }
  .choices label { display: inline-flex; flex-direction: row; align-items: center; gap: 6px; padding: 4px 12px; border: 1px solid var(--border-default); border-radius: 999px; color: var(--ink-secondary); font-size: 13px; cursor: pointer; transition: border-color var(--t-fast) var(--ease-out), background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .choices label:has(input:checked) { border-color: var(--accent); background: var(--accent-soft); color: var(--accent-deep); }
  .choices input { accent-color: var(--accent); margin: 0; }
  .needs { display: flex; flex-wrap: wrap; gap: 10px; margin: 0; padding: 14px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); }
  .needs legend { padding: 0 5px; color: var(--ink-secondary); font-size: 14px; }
  .needs label { display: inline-flex; align-items: center; gap: 7px; padding: 5px 12px; border: 1px solid var(--border-default); border-radius: 999px; color: var(--ink-secondary); font-size: 13px; cursor: pointer; transition: border-color var(--t-fast) var(--ease-out), background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .needs label:has(input:checked) { border-color: var(--accent); background: var(--accent-soft); color: var(--accent-deep); }
  .needs input { accent-color: var(--accent); margin: 0; }
  .actions { display: flex; align-items: center; gap: 12px; }
  .actions .secondary { width: auto; min-height: 44px; padding: 0 18px; justify-content: center; }
  @media (max-width: 600px) { form { padding: 18px; } }
</style>
