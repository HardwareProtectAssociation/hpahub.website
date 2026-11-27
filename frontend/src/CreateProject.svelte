<script>
  import { t } from './i18n.js';
  export let token;
  let title = '', summary = '', description = '', difficulty = '', nature = '';
  let collaboration = 'collaborative', joinMode = 'open', sponsorship = false;
  let startDate = '', duration = '', scheduleNote = '', error = '', creating = false;
  let executionStatus = 'not_started', allowOthersManagement = false, confidentialityRequested = false, confidentialityTerms = '';
  let projectNeeds = [];
  let providerOffers = [];
  const needOptions = [['funding', '资金支持'], ['technical', '技术支持'], ['equipment', '设备支持'], ['venue', '场地支持'], ['manpower', '人手支持'], ['promotion', '宣传支持'], ['emotional', '情绪支持']];
  const offerOptions = [['technical', '技术'], ['funding', '资金'], ['equipment', '设备'], ['venue', '场地'], ['manpower', '人手'], ['promotion', '宣传']];

  async function createProject() {
    if (!title.trim() || creating) return;
    creating = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/projects', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: title.trim(), summary: summary.trim(), description: description.trim(), proposedDifficulty: difficulty, projectNature: nature, projectNeeds, providerOffers, collaborationMode: collaboration, joinMode, acceptsSponsorship: sponsorship, allowOthersManagement, confidentialityRequested, confidentialityTerms: confidentialityRequested ? confidentialityTerms.trim() : '', plannedStartDate: executionStatus === 'completed' ? '' : startDate, estimatedDurationDays: !['completed', 'long_term'].includes(executionStatus) && duration ? Number(duration) : 0, longTerm: executionStatus === 'long_term', executionStatus, scheduleNote: scheduleNote.trim() }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '创建失败，请重试。');
      location.assign('/showcase/' + encodeURIComponent(result.id));
    } catch (err) {
      error = err.message || String(err);
      creating = false;
    }
  }
</script>

<main class="dashboard create-project">
  <div class="page-heading"><div><h1>{$t('发起项目')}<span class="heading-dot">.</span></h1></div></div>
  <form on:submit|preventDefault={createProject}>
    <label>{$t('项目名称')}<input bind:value={title} maxlength="160" required /></label>
    <label>{$t('项目简介')}<textarea bind:value={summary} maxlength="2000" rows="3"></textarea></label>
    <label>{$t('项目详情')}<textarea bind:value={description} maxlength="10000" rows="7"></textarea></label>

    <div class="fields">
      <label>{$t('项目状态')}<select bind:value={executionStatus}><option value="not_started">{$t('未开始')}</option><option value="ongoing">{$t('已启动')}</option><option value="long_term">{$t('长期项目')}</option><option value="paused">{$t('暂停中')}</option><option value="completed">{$t('已完成')}</option></select></label>
      <label>{$t('发起人自评难度')}<select bind:value={difficulty}><option value="">{$t('暂不设置')}</option>{#each ['D', 'C', 'B', 'A', 'S'] as level}<option value={level}>{level}</option>{/each}</select></label>
      <label>{$t('项目性质')}<select bind:value={nature} required><option value="" disabled>{$t('请选择')}</option><option value="commercial">{$t('商业盈利')}</option><option value="learning">{$t('学习交流')}</option><option value="showcase">{$t('技术展示')}</option><option value="research">{$t('科学研究')}</option></select></label>
      <label>{$t('协作方式')}<select bind:value={collaboration}><option value="collaborative">{$t('协作项目')}</option><option value="independent">{$t('独立项目')}</option></select></label>
      {#if collaboration === 'collaborative'}<label>{$t('加入方式')}<select bind:value={joinMode}><option value="open">{$t('自由加入')}</option><option value="approval">{$t('申请加入')}</option></select></label>{/if}
      {#if executionStatus !== 'completed'}<label>{$t('计划开始日期')}<input type="date" bind:value={startDate} /></label>{/if}
      {#if !['completed', 'long_term'].includes(executionStatus)}<label>{$t('预计周期（天）')}<input type="number" min="1" max="36500" step="1" bind:value={duration} /></label>{/if}
    </div>
    <label>{$t('时间安排')}<textarea bind:value={scheduleNote} maxlength="2000" rows="3"></textarea></label>
    <fieldset class="arrangements"><legend>{$t('项目安排')}</legend>
      <div class="arrangement-row"><span>{$t('接受赞助')}</span><span class="choices" role="group" aria-label={$t('接受赞助')}><label><input type="radio" bind:group={sponsorship} value={true} />{$t('是')}</label><label><input type="radio" bind:group={sponsorship} value={false} />{$t('否')}</label></span></div>
      <div class="arrangement-row"><span>{$t('允许他人管理')}</span><span class="choices" role="group" aria-label={$t('允许他人管理')}><label><input type="radio" bind:group={allowOthersManagement} value={true} />{$t('是')}</label><label><input type="radio" bind:group={allowOthersManagement} value={false} />{$t('否')}</label></span></div>
      <div class="arrangement-row"><span>{$t('需要保密')}</span><span class="choices" role="group" aria-label={$t('需要保密')}><label><input type="radio" bind:group={confidentialityRequested} value={true} />{$t('是')}</label><label><input type="radio" bind:group={confidentialityRequested} value={false} />{$t('否')}</label></span></div>
    </fieldset>
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
    {#if confidentialityRequested}<label>{$t('保密协议')}<textarea bind:value={confidentialityTerms} maxlength="10000" rows="6" required></textarea></label>{/if}
    {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
    <div class="actions"><button class="primary" type="submit" disabled={creating}>{$t(creating ? '正在创建…' : '创建项目')}</button><a class="secondary" href="/my-projects">{$t('取消')}</a></div>
  </form>
</main>

<style>
  .create-project { width: min(880px, 88%); }
  form { display: grid; gap: 20px; padding: 28px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); }
  label { display: grid; gap: 8px; color: var(--ink-secondary); font-size: 14px; }
  input:not([type="checkbox"]), select, textarea { width: 100%; padding: 11px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  input:focus, select:focus, textarea:focus { border-color: var(--accent); outline: 2px solid var(--accent-glow); }
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
