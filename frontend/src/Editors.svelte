<script>
  import { createEventDispatcher, onMount, onDestroy, tick } from 'svelte';
  import { get } from 'svelte/store';
  import { t } from './i18n.js';
  import AvatarCrop from './AvatarCrop.svelte';
  import Select from './Select.svelte';
  import MarkdownContent from './MarkdownContent.svelte';
  import CardCanvas from './CardCanvas.svelte';
  import { difficultyRank } from './tiers.js';
  export let token;
  export let manage = false;
  export let cardsOnly = false;
  const dispatch = createEventDispatcher();
  let profile = { name: '', bio: '', profilePublic: false, avatarUrl: '', currentStatus: '' };
  let publicProfileId = '';
  let records = [];
  let current = null;
  let showEditor = false;
  let editorDialog;
  let title = '';
  let text = '';
  let published = false;
  let keepImages = [];
  let files = [];
  let avatar = null;
  let avatarThumb = null;
  let cropPending = false;
  let imagesInput;
  let removeAvatar = false;
  let previews = {};
  let uploadPreviews = [];
  let layout = { version: 1, items: [] };
  let canvasMode = true;
  let addingBackground = false;
  $: canvasImages = [...keepImages.map(name => previews[current?.imageUrls?.[current.images.indexOf(name)]]), ...uploadPreviews];
  function addCanvasFiles(input, asBackground = false) {
    const selected = Array.from(input);
    if (!selected.length) return;
    const replacesBackground = asBackground && layout.backgroundImage != null;
    if ((asBackground && selected.length !== 1) || selected.some(file => !['image/jpeg','image/png','image/webp','image/gif'].includes(file.type) || file.size > 2 * 1024 * 1024) || canvasImages.length - (replacesBackground ? 1 : 0) + selected.length > 5 || layout.items.length + (asBackground ? 0 : selected.length) > 24) {
      error = '图片格式不支持、超过 2 MiB 或超出数量限制。'; return;
    }
    error = '';
    const start = canvasImages.length - (replacesBackground ? 1 : 0);
    if (replacesBackground) removeCanvasImage(layout.backgroundImage);
    files = [...files, ...selected];
    uploadPreviews = [...uploadPreviews, ...selected.map(file => URL.createObjectURL(file))];
    layout = asBackground
      ? { ...layout, backgroundImage: start }
      : { ...layout, items: [...layout.items, ...selected.map((_, i) => ({ type: 'image', image: start + i, x: 10, y: 10, width: 35, rotation: 0 }))] };
  }
  function convertToCanvas() {
    layout = { version: 1, backgroundImage: null, items: [
      ...(text ? [{ type: 'text', text, x: 5, y: 5, width: 80, rotation: 0, font: 'sans-serif', size: 24 }] : []),
      ...canvasImages.map((_, i) => ({ type: 'image', image: i, x: 5 + i * 20, y: 40, width: 25, rotation: 0 }))
    ] };
    canvasMode = true;
  }
  function removeCanvasImage(index) {
    if (index < keepImages.length) keepImages = keepImages.filter((_, i) => i !== index);
    else { const i = index - keepImages.length; URL.revokeObjectURL(uploadPreviews[i]); files = files.filter((_, n) => n !== i); uploadPreviews = uploadPreviews.filter((_, n) => n !== i); }
    layout = { ...layout, backgroundImage: layout.backgroundImage == null ? null : layout.backgroundImage === index ? null : layout.backgroundImage > index ? layout.backgroundImage - 1 : layout.backgroundImage, items: layout.items.filter(entry => entry.type !== 'image' || entry.image !== index).map(entry => entry.type === 'image' && entry.image > index ? { ...entry, image: entry.image - 1 } : entry) };
  }
  function pasteCanvas(event) {
    if (manage || !showEditor || !canvasMode) return;
    const pasted = Array.from(event.clipboardData?.files || []);
    if (pasted.length) { event.preventDefault(); addCanvasFiles(pasted); return; }
    if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement || event.target.isContentEditable) return;
    const value = event.clipboardData?.getData('text/plain');
    if (value && layout.items.length < 24) {
      event.preventDefault();
      layout = { ...layout, items: [...layout.items, { type: 'text', text: value, x: 10, y: 10, width: 40, rotation: 0, font: 'sans-serif', size: 24 }] };
    }
  }
  let avatarPreview = '';
  function selectFiles(event) {
    const selected = Array.from(event.currentTarget.files);
    if (selected.some(file => file.size > 2 * 1024 * 1024)) {
      error = '图片不能超过 2 MiB。';
      event.currentTarget.value = '';
      return;
    }
    error = '';
    uploadPreviews.forEach(URL.revokeObjectURL);
    files = selected;
    uploadPreviews = files.map(file => URL.createObjectURL(file));
  }
  function selectAvatar(event) {
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatar = event.detail.avatar;
    avatarThumb = event.detail.thumb;
    avatarPreview = URL.createObjectURL(avatarThumb);
    removeAvatar = false;
  }

  let error = '';
  let notice = '';
  let errorParams = {};
  let busy = false;
  let projects = [];
  let difficultyError = '';
  let difficultyNotice = '';
  let savingDifficulty = '';
  const difficultyLevels = ['D', 'C', 'B', 'A', 'S'];
  const projectStatusLabels = { not_started: '未开始', ongoing: '已启动', long_term: '长期项目', paused: '暂停中', terminated: '已中止', completed: '已完成' };
  let voteDifficulties = {};
  let voteStances = {};
  let voteComments = {};
  let ownerConfirming = {};
  const base = manage ? '/api/hpahub/manage/showcase' : '/api/hpahub/me/cards';
  const headers = () => ({ Authorization: `Bearer ${token}` });
  async function request(url, options = {}) {
    const response = await fetch(url, { ...options, headers: { ...headers(), ...options.headers }, cache: 'no-store' });
    if (!response.ok) { const result = await response.json().catch(() => ({})); const failure = new Error(result.message || '请求失败（{status}）'); if (!result.message) failure.params = { status: response.status }; throw failure; }
    return response.status === 204 ? null : response.json();
  }
  async function preview(url) {
    if (!url || previews[url]) return;
    const response = await fetch(url, { headers: headers() });
    if (response.ok) {
      const blob = await response.blob();
      previews = { ...previews, [url]: URL.createObjectURL(blob) };
    }
  }
  function clearPreviews() { Object.values(previews).forEach(URL.revokeObjectURL); previews = {}; }
  async function load() {
    clearPreviews();
    const [list, nextProfile, managed] = await Promise.all([
      request(base),
      manage || cardsOnly ? null : request('/api/hpahub/me/center'),
      manage ? request('/api/hpahub/manage/projects') : null,
    ]);
    records = list.items;
    if (nextProfile) {
      profile = nextProfile.profile;
      publicProfileId = profile.profilePublic ? profile.id : '';
    }
    if (managed) {
      projects = managed.items;
      voteDifficulties = Object.fromEntries(projects.map((project) => [project.id, project.difficulty || 'D']));
      voteStances = Object.fromEntries(projects.map((project) => [project.id, 'approve']));
    }
    await Promise.all([...(profile.avatarUrl && !manage ? [preview(profile.avatarUrl)] : []), ...records.flatMap(record => (record.imageUrls || []).map(preview))]);
  }
  onMount(async () => { try { await load(); } catch (e) { error = e.message; errorParams = e.params || {}; } });
  onDestroy(() => { clearPreviews(); uploadPreviews.forEach(URL.revokeObjectURL); if (avatarPreview) URL.revokeObjectURL(avatarPreview); });
  async function edit(record = null) {
    current = record;
    showEditor = true;
    title = record?.title || '';
    text = record?.text || '';
    published = record?.published || false;
    keepImages = [...(record?.images || [])];
    canvasMode = !record || record.layout?.version === 1;
    layout = record?.layout?.version === 1 ? structuredClone(record.layout) : { version: 1, backgroundImage: null, items: [
      ...(record?.text ? [{ type: 'text', text: record.text, x: 5, y: 5, width: 80, rotation: 0, font: 'sans-serif', size: 24 }] : []),
      ...(record?.images || []).map((_, i) => ({ type: 'image', image: i, x: 5 + i * 20, y: 40, width: 25, rotation: 0 }))
    ] };
    files = [];
    if (imagesInput) imagesInput.value = '';
    uploadPreviews.forEach(URL.revokeObjectURL); uploadPreviews = [];
    error = ''; notice = '';
    showEditor = true;
    await tick();
    editorDialog?.showModal();
  }
  function closeEditor() {
    showEditor = false;
    current = null;
    editorDialog?.close();
    uploadPreviews.forEach(URL.revokeObjectURL);
    uploadPreviews = [];
    files = [];
  }
  async function saveProfile() {
    if (cropPending) return;
    busy = true; error = ''; notice = '';
    try {
      const body = new FormData();
      body.set('name', profile.name); body.set('bio', profile.bio); body.set('profilePublic', String(profile.profilePublic)); body.set('currentStatus', profile.currentStatus || '');
      if (avatar && !removeAvatar) { body.set('avatar', avatar); body.set('avatarThumb', avatarThumb); }
      if (removeAvatar) body.set('removeAvatar', 'true');
      const result = await request('/api/hpahub/me/center/profile', { method: 'PATCH', body });
      dispatch('profileSaved', { name: result.profile?.name });
      avatar = null; avatarThumb = null; if (avatarPreview) URL.revokeObjectURL(avatarPreview); avatarPreview = ''; removeAvatar = false; await load(); notice = '资料已保存。';
    } catch (e) { error = e.message; errorParams = e.params || {}; } finally { busy = false; }
  }
  async function saveRecord() {
    busy = true; error = ''; notice = '';
    try {
      const body = new FormData(); body.set('title', title);
      if (!manage && canvasMode) {
        text = layout.items.filter(item => item.type === 'text').map(item => item.text).join('\n');
        if ([...text].length > 280) throw new Error('画布文字总长度不能超过 280 字。');
        body.set('layout', JSON.stringify(layout));
      }
      body.set('text', text);
      if (manage) body.set('published', String(published));
      keepImages.forEach(name => body.append('keepImages', name));
      Array.from(files).forEach(file => body.append('images', file));
      await request(base + (current ? '/' + encodeURIComponent(current.id) : ''), { method: current ? 'PATCH' : 'POST', body });
      closeEditor(); await load(); notice = '保存成功。';
    } catch (e) { error = e.message; errorParams = e.params || {}; } finally { busy = false; }
  }
  async function saveDifficulty(project) {
    savingDifficulty = project.id;
    difficultyError = '';
    difficultyNotice = '';
    try {
      const diff = voteDifficulties[project.id] || project.difficulty;
      const stance = voteStances[project.id] || 'approve';
      const comment = voteComments[project.id] || '';
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(project.id)}/difficulty/vote`, {
        method: 'POST',
        headers: { ...headers(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ difficulty: diff, stance, comment }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '投票失败，请重试。');
      project.approveCount = result.approveCounts?.[diff] || 0;
      project.opposeCount = result.opposeCounts?.[diff] || 0;
      difficultyNotice = result.consensusReached
        ? `项目级别 ${result.consensus} 已确认。`
        : `投票已提交，${result.approveCounts?.[diff] || 0} 票同意 ${diff}。`;
    } catch (e) {
      difficultyError = e.message || String(e);
    } finally {
      savingDifficulty = '';
    }
  }

  async function confirmOwner(project, confirmed) {
    savingDifficulty = project.id;
    difficultyError = '';
    difficultyNotice = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(project.id)}/difficulty/confirm`, {
        method: 'POST',
        headers: { ...headers(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ confirmed, proposedDifficulty: project.proposedDifficulty || '' }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '确认失败，请重试。');
      project.ownerConfirmed = confirmed;
      difficultyNotice = confirmed ? '发起人已确认项目级别。' : '发起人已取消确认。';
    } catch (e) {
      difficultyError = e.message || String(e);
    } finally {
      savingDifficulty = '';
    }
  }
  async function remove(record) {
    if (!confirm(get(t)('确定删除“{title}”吗？', { title: record.title }))) return;
    busy = true; error = ''; notice = '';
    try { await request(base + '/' + encodeURIComponent(record.id), { method: 'DELETE' }); await load(); notice = '已删除。'; }
    catch (e) { error = e.message; errorParams = e.params || {}; } finally { busy = false; }
  }
</script>

<svelte:window on:paste={pasteCanvas} />
<div class:dashboard={!cardsOnly} class="public-page editor-page">
  {#if !cardsOnly}<h1>{manage ? $t('项目大厅管理') : $t('修改个人名片')}</h1>{/if}
  {#if error}<p class="message error" role="alert">{$t(error, errorParams)}</p>{/if}{#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}
  {#if manage}
    <section class="difficulty-section">
      <h2>{$t('项目级别')}</h2>
      <p>{$t('由管理员判定，等级从 D（最低）到 S（最高）。')}</p>
      {#if difficultyError}<p class="message error" role="alert">{$t(difficultyError)}</p>{/if}
      {#if difficultyNotice}<p class="message success" role="status">{$t(difficultyNotice)}</p>{/if}
      {#if projects.length}
        <div class="difficulty-list">
          {#each projects as project}
            <div class="difficulty-row">
              <div class="difficulty-info">
                <strong>{project.title}</strong>{#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
                <small>{$t(projectStatusLabels[project.status] || project.status)} · {project.ownerName || '—'}</small>
                <small>
                  {#if project.proposedDifficulty}<span>{$t('发起人评级')}：<span class={`rank rank-text rank-${difficultyRank(project.proposedDifficulty)}`}>{project.proposedDifficulty}</span></span>{:else}<span>{$t('发起人评级')}：{$t('待定')}</span>{/if}
                  <span>{$t('同意')}：{project.approveCount || 0}</span>
                  <span>{$t('反对')}：{project.opposeCount || 0}</span>
                  {#if project.ownerConfirmed}<span class="confirmed">✓ {$t('发起人已确认')}</span>{/if}
                  {#if project.reviewStatus === 'approved' && project.difficulty}<span class="consensus">共识：<span class={`rank rank-text rank-${difficultyRank(project.difficulty)}`}>{project.difficulty}</span></span>{:else}<span class="consensus">共识：{$t('待定')}</span>{/if}
                </small>
              </div>
              {#if project.status !== 'completed'}
                <div class="vote-controls">
                  <Select bind:value={voteDifficulties[project.id]} ariaLabel={`${project.title} ${$t('项目级别')}`} options={difficultyLevels.map((value) => ({ value, label: value }))} />
                  <Select bind:value={voteStances[project.id]} ariaLabel={`${project.title} ${$t('立场')}`} options={[{ value: 'approve', label: $t('同意') }, { value: 'oppose', label: $t('反对') }]} />
                  <button type="button" class="primary small" on:click={() => saveDifficulty(project)} disabled={savingDifficulty === project.id}>{$t('投票')}</button>
                  <button type="button" class="small" on:click={() => confirmOwner(project, true)} disabled={savingDifficulty === project.id}>{$t('发起人确认')}</button>
                </div>
              {:else}
                <span class="frozen">{$t('已完成，项目级别锁定')}</span>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <p>{$t('暂无项目。')}</p>
      {/if}
    </section>
  {/if}
  {#if !manage && !cardsOnly}
    <section><h2>{$t('公开资料')}</h2><form class="editor-form" on:submit|preventDefault={saveProfile}>
      {#if profile.canEditProfile === false}<p class="message error" role="status">{$t('个人资料每7天只能修改一次')} · {new Date(profile.nextProfileEditAt).toLocaleString()}</p>{/if}
      {#if avatarPreview && !removeAvatar}<img class="avatar" src={avatarPreview} alt={$t('新头像预览')} />{:else if profile.avatarUrl && previews[profile.avatarUrl] && !removeAvatar}<img class="avatar" src={previews[profile.avatarUrl]} alt={$t('当前头像')} />{/if}
      <label>{$t('昵称')} <input bind:value={profile.name} maxlength="80" required /></label>
      <label>{$t('简介')} <textarea bind:value={profile.bio} maxlength="600" rows="4"></textarea></label>
      <div class="select-field"><span>{$t('当前状态')}</span><Select bind:value={profile.currentStatus} ariaLabel={$t('当前状态')} options={[{ value: '', label: $t('未设置') }, { value: 'busy', label: $t('忙碌') }, { value: 'free', label: $t('空闲') }]} /></div>
      <AvatarCrop on:crop={selectAvatar} on:pending={(event) => cropPending = event.detail} />
      {#if profile.avatarUrl}<label class="check-label"><input type="checkbox" bind:checked={removeAvatar} on:change={() => { if (removeAvatar) { avatar = null; avatarThumb = null; } }} /> {$t('删除头像')}</label>{/if}
      <label class="check-label"><input type="checkbox" bind:checked={profile.profilePublic} /> {$t('公开我的资料和卡片')}</label>
      <button class="primary" disabled={busy || cropPending || profile.canEditProfile === false}>{$t('保存资料')}</button>
      {#if publicProfileId}<a href={'/community/' + encodeURIComponent(publicProfileId)}>{$t('查看公开主页')}</a>{/if}
      <a href="/me/skills">{$t('技能管理')}</a>
    </form></section>
  {/if}
  <section><h2>{manage ? $t('项目') : $t('自定义卡片')}</h2>
    <div class="editor-toolbar"><button type="button" class="primary" on:click={() => edit()}>{manage ? $t('新建项目') : $t('新建卡片')}</button></div>
    {#if records.length}<div class="editor-list">{#each records as record}<article><h3>{record.title} {#if manage}<small>{record.published ? $t('已发布') : $t('草稿')}</small>{/if}</h3>{#if manage}<p>{record.text}</p>{:else if record.layout?.version === 1}<CardCanvas layout={record.layout} images={(record.imageUrls || []).map(url => previews[url])} />{:else}<MarkdownContent text={record.text} />{/if}{#if manage || record.layout?.version !== 1}<div class="image-grid">{#each record.imageUrls || [] as url}{#if previews[url]}<a href={previews[url]} target="_blank" rel="noreferrer"><img src={previews[url]} alt={record.title} /></a>{/if}{/each}</div>{/if}<div class="editor-actions"><button type="button" on:click={() => edit(record)}>{$t('编辑')}</button><button type="button" on:click={() => remove(record)} disabled={busy}>{$t('删除')}</button></div></article>{/each}</div>{:else}<p>{manage ? $t('暂无项目。') : $t('暂无卡片。')}</p>{/if}
  </section>
</div>

{#if showEditor}
  <dialog class="editor-modal" bind:this={editorDialog} aria-label={current ? (manage ? $t('编辑项目') : $t('编辑卡片')) : (manage ? $t('新建项目') : $t('新建卡片'))} on:cancel|preventDefault={closeEditor}>
      <div class="editor-modal-head">
        <h2>{current ? (manage ? $t('编辑项目') : $t('编辑卡片')) : (manage ? $t('新建项目') : $t('新建卡片'))}</h2>
        <button type="button" class="editor-modal-close" aria-label={$t('关闭')} on:click={closeEditor}>×</button>
      </div>
      <form class="editor-form" on:submit|preventDefault={saveRecord}>
        {#if error}<p class="message error" role="alert">{$t(error, errorParams)}</p>{/if}
        <label>{$t('标题')} <input bind:value={title} maxlength="80" required /></label>
        {#if manage || !canvasMode}
          {#if !manage}<button type="button" on:click={convertToCanvas}>{$t('转换为画布')}</button><p>{$t('转换后文字将按纯文本显示。')}</p>{/if}
          <label>{$t('文字')} <textarea bind:value={text} maxlength="280" rows="6"></textarea></label>
          {#if current?.images?.length}<fieldset><legend>{$t('保留图片')}</legend>{#each current.images as image, index}<label class="check-label"><input type="checkbox" value={image} bind:group={keepImages} /> {#if previews[current.imageUrls[index]]}<img class="editor-thumb" src={previews[current.imageUrls[index]]} alt="" />{/if} {image}</label>{/each}</fieldset>{/if}
          <label>{$t('添加图片（最多 4 张，每张最多 2 MiB）')} <input type="file" bind:this={imagesInput} accept="image/jpeg,image/png,image/webp,image/gif" multiple on:change={selectFiles} /></label>
          {#if uploadPreviews.length}<div class="image-grid">{#each uploadPreviews as url}<img src={url} alt={$t('待上传图片预览')} />{/each}</div>{/if}
        {:else}
          <input type="file" hidden bind:this={imagesInput} accept="image/jpeg,image/png,image/webp,image/gif" multiple on:change={(e) => { const asBackground = addingBackground; addingBackground = false; addCanvasFiles(e.currentTarget.files, asBackground); e.currentTarget.value = ''; }} />
          <CardCanvas bind:layout images={canvasImages} editable on:addimage={() => { addingBackground = false; imagesInput.click(); }} on:addbackgroundimage={() => { addingBackground = true; imagesInput.click(); }} on:removeimage={(e) => removeCanvasImage(e.detail)} />
        {/if}
        {#if manage}<label class="check-label"><input type="checkbox" bind:checked={published} /> {$t('发布项目')}</label>{/if}
        <div class="editor-actions"><button class="primary" disabled={busy}>{$t('保存')}</button><button type="button" on:click={closeEditor}>{$t('取消')}</button></div>
      </form>
  </dialog>
{/if}

<style>
  .difficulty-section { margin: 26px 0; }
  .difficulty-list { display: grid; gap: 10px; max-width: 900px; }
  .difficulty-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 16px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); }
  .difficulty-info { display: grid; gap: 3px; }
  .difficulty-info small { color: var(--ink-muted); }
  .difficulty-info small span { margin-right: 10px; }
  .difficulty-info .confirmed { color: var(--accent-deep); font-weight: 600; }
  .difficulty-info .consensus { color: #166a78; font-weight: 600; }
  .vote-controls { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .vote-controls :global(.select) { min-width: 90px; }
  .vote-controls .small { padding: 5px 11px; font-size: 12px; min-height: 32px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-secondary); }
  .vote-controls .small.primary { border-color: transparent; color: #fff; }
  .frozen { color: var(--ink-subtle); font-size: 12px; }
  .editor-toolbar { margin: 12px 0; }
  .editor-modal { width: min(680px, calc(100% - 32px)); max-height: min(90dvh, 900px); overflow-y: auto; padding: 26px; border: 1px solid var(--border-default); border-radius: var(--radius-lg); background: var(--surface-elevated); color: var(--ink-primary); box-shadow: var(--shadow-md); }
  .editor-modal::backdrop { background: #0009; }
  .editor-modal-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
  .editor-modal-head h2 { margin: 0; font-size: 20px; }
  .editor-modal-close { border: 0; background: none; color: var(--ink-muted); font-size: 26px; line-height: 1; padding: 0 4px; cursor: pointer; }
  @media (max-width: 600px) { .editor-modal { padding: 18px; } }
</style>
