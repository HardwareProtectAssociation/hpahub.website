<script>
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { t } from './i18n.js';
  export let token;
  export let role;
  export let manage = false;
  let profile = { name: '', bio: '', profilePublic: false, avatarUrl: '' };
  let records = [];
  let current = null;
  let title = '';
  let text = '';
  let published = false;
  let keepImages = [];
  let files = [];
  let avatar = null;
  let imagesInput;
  let avatarInput;
  let removeAvatar = false;
  let previews = {};
  let uploadPreviews = [];
  let avatarPreview = '';
  function selectFiles(event) {
    uploadPreviews.forEach(URL.revokeObjectURL);
    files = Array.from(event.currentTarget.files);
    uploadPreviews = files.map(file => URL.createObjectURL(file));
  }
  function selectAvatar(event) {
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatar = event.currentTarget.files[0] || null;
    avatarPreview = avatar ? URL.createObjectURL(avatar) : '';
    if (avatar) removeAvatar = false;
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
  const projectStatusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };
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
    const response = await fetch(url, { headers: headers(), cache: 'no-store' });
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
      manage ? null : request('/api/hpahub/me/profile'),
      manage ? request('/api/hpahub/manage/projects') : null,
    ]);
    records = list.items;
    if (nextProfile) profile = nextProfile;
    if (managed) projects = managed.items;
    await Promise.all([...(profile.avatarUrl && !manage ? [preview(profile.avatarUrl)] : []), ...records.flatMap(record => (record.imageUrls || []).map(preview))]);
  }
  onMount(async () => { try { await load(); } catch (e) { error = e.message; errorParams = e.params || {}; } });
  onDestroy(() => { clearPreviews(); uploadPreviews.forEach(URL.revokeObjectURL); if (avatarPreview) URL.revokeObjectURL(avatarPreview); });
  function edit(record = null) {
    current = record;
    title = record?.title || '';
    text = record?.text || '';
    published = record?.published || false;
    keepImages = [...(record?.images || [])];
    files = [];
    imagesInput.value = '';
    uploadPreviews.forEach(URL.revokeObjectURL); uploadPreviews = [];
    error = ''; notice = '';
  }
  async function saveProfile() {
    busy = true; error = ''; notice = '';
    try {
      const body = new FormData();
      body.set('name', profile.name); body.set('bio', profile.bio); body.set('profilePublic', String(profile.profilePublic));
      if (avatar) body.set('avatar', avatar);
      if (removeAvatar) body.set('removeAvatar', 'true');
      await request('/api/hpahub/me/profile', { method: 'PATCH', body });
      avatar = null; avatarInput.value = ''; if (avatarPreview) URL.revokeObjectURL(avatarPreview); avatarPreview = ''; removeAvatar = false; await load(); notice = '资料已保存。';
    } catch (e) { error = e.message; errorParams = e.params || {}; } finally { busy = false; }
  }
  async function saveRecord() {
    busy = true; error = ''; notice = '';
    try {
      const body = new FormData(); body.set('title', title); body.set('text', text);
      if (manage) body.set('published', String(published));
      keepImages.forEach(name => body.append('keepImages', name));
      Array.from(files).forEach(file => body.append('images', file));
      await request(base + (current ? '/' + encodeURIComponent(current.id) : ''), { method: current ? 'PATCH' : 'POST', body });
      edit(); await load(); notice = '保存成功。';
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
        ? `共识难度 ${result.consensus} 已确认。`
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
      difficultyNotice = confirmed ? '发起人已确认难度。' : '发起人已取消确认。';
    } catch (e) {
      difficultyError = e.message || String(e);
    } finally {
      savingDifficulty = '';
    }
  }
  async function remove(record) {
    if (!confirm(get(t)('确定删除“{title}”吗？', { title: record.title }))) return;
    busy = true; error = ''; notice = '';
    try { await request(base + '/' + encodeURIComponent(record.id), { method: 'DELETE' }); edit(); await load(); notice = '已删除。'; }
    catch (e) { error = e.message; errorParams = e.params || {}; } finally { busy = false; }
  }
</script>

<main class="dashboard public-page editor-page">
  <h1>{manage ? $t('项目大厅管理') : $t('个人设置')}</h1>
  {#if error}<p class="message error" role="alert">{$t(error, errorParams)}</p>{/if}{#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}
  {#if manage}
    <section class="difficulty-section">
      <h2>{$t('项目难度')}</h2>
      <p>{$t('由管理员判定，等级从 D（最低）到 S（最高）。')}</p>
      {#if difficultyError}<p class="message error" role="alert">{$t(difficultyError)}</p>{/if}
      {#if difficultyNotice}<p class="message success" role="status">{$t(difficultyNotice)}</p>{/if}
      {#if projects.length}
        <div class="difficulty-list">
          {#each projects as project}
            <div class="difficulty-row">
              <div class="difficulty-info">
                <strong>{project.title}</strong>
                <small>{$t(projectStatusLabels[project.status] || project.status)} · {project.ownerName || '—'}</small>
                <small>
                  {#if project.proposedDifficulty}<span>{$t('发起人评级')}：{project.proposedDifficulty}</span>{/if}
                  <span>{$t('同意')}：{project.approveCount || 0}</span>
                  <span>{$t('反对')}：{project.opposeCount || 0}</span>
                  {#if project.ownerConfirmed}<span class="confirmed">✓ {$t('发起人已确认')}</span>{/if}
                  {#if project.difficulty}<span class="consensus">共识：{project.difficulty}</span>{/if}
                </small>
              </div>
              {#if project.status !== 'completed'}
                <div class="vote-controls">
                  <select bind:value={voteDifficulties[project.id]} aria-label={`${project.title} ${$t('难度')}`}>
                    {#each difficultyLevels as level}<option value={level}>{level}</option>{/each}
                  </select>
                  <select bind:value={voteStances[project.id]} aria-label={`${project.title} ${$t('立场')}`}>
                    <option value="approve">{$t('同意')}</option>
                    <option value="oppose">{$t('反对')}</option>
                  </select>
                  <button type="button" class="primary small" on:click={() => saveDifficulty(project)} disabled={savingDifficulty === project.id}>{$t('投票')}</button>
                  <button type="button" class="small" on:click={() => confirmOwner(project, true)} disabled={savingDifficulty === project.id}>{$t('发起人确认')}</button>
                </div>
              {:else}
                <span class="frozen">{$t('已完成，难度锁定')}</span>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <p>{$t('暂无项目。')}</p>
      {/if}
    </section>
  {/if}
  {#if !manage}
    <section><h2>{$t('成员资料')}</h2><form class="editor-form" on:submit|preventDefault={saveProfile}>
      {#if avatarPreview && !removeAvatar}<img class="avatar" src={avatarPreview} alt={$t('新头像预览')} />{:else if profile.avatarUrl && previews[profile.avatarUrl] && !removeAvatar}<img class="avatar" src={previews[profile.avatarUrl]} alt={$t('当前头像')} />{/if}
      <label>{$t('昵称')} <input bind:value={profile.name} maxlength="80" required={profile.profilePublic} /></label>
      <label>{$t('简介')} <textarea bind:value={profile.bio} maxlength="600" rows="4"></textarea></label>
      <label>{$t('头像（JPEG、PNG、WebP 或 GIF，最多 5 MiB）')} <input type="file" bind:this={avatarInput} accept="image/jpeg,image/png,image/webp,image/gif" on:change={selectAvatar} /></label>
      {#if profile.avatarUrl}<label class="check-label"><input type="checkbox" bind:checked={removeAvatar} on:change={() => { if (removeAvatar) { avatar = null; avatarInput.value = ''; } }} /> {$t('删除头像')}</label>{/if}
      <label class="check-label"><input type="checkbox" bind:checked={profile.profilePublic} /> {$t('公开我的资料和卡片')}</label>
      <button class="primary" disabled={busy}>{$t('保存资料')}</button>
    </form></section>
  {/if}
  <section><h2>{manage ? $t('项目') : $t('我的卡片')}</h2>
    {#if records.length}<div class="editor-list">{#each records as record}<article><h3>{record.title} {#if manage}<small>{record.published ? $t('已发布') : $t('草稿')}</small>{/if}</h3><p>{record.text}</p><div class="image-grid">{#each record.imageUrls || [] as url}{#if previews[url]}<a href={previews[url]} target="_blank" rel="noreferrer"><img src={previews[url]} alt={record.title} /></a>{/if}{/each}</div><div class="editor-actions"><button type="button" on:click={() => edit(record)}>{$t('编辑')}</button><button type="button" on:click={() => remove(record)} disabled={busy}>{$t('删除')}</button></div></article>{/each}</div>{:else}<p>{manage ? $t('暂无项目。') : $t('暂无卡片。')}</p>{/if}
    <h2>{current ? (manage ? $t('编辑项目') : $t('编辑卡片')) : (manage ? $t('新建项目') : $t('新建卡片'))}</h2><form class="editor-form" on:submit|preventDefault={saveRecord}>
      <label>{$t('标题')} <input bind:value={title} maxlength="80" required /></label><label>{$t('文字')} <textarea bind:value={text} maxlength="280" rows="4"></textarea></label>
      {#if current?.images?.length}<fieldset><legend>{$t('保留图片')}</legend>{#each current.images as image, index}<label class="check-label"><input type="checkbox" value={image} bind:group={keepImages} /> {#if previews[current.imageUrls[index]]}<img class="editor-thumb" src={previews[current.imageUrls[index]]} alt="" />{/if} {image}</label>{/each}</fieldset>{/if}
      <label>{$t('添加图片（最多 4 张，每张最多 5 MiB）')} <input type="file" bind:this={imagesInput} accept="image/jpeg,image/png,image/webp,image/gif" multiple on:change={selectFiles} /></label>
      {#if uploadPreviews.length}<div class="image-grid">{#each uploadPreviews as url}<img src={url} alt={$t('待上传图片预览')} />{/each}</div>{/if}
      {#if manage}<label class="check-label"><input type="checkbox" bind:checked={published} /> {$t('发布项目')}</label>{/if}
      <div class="editor-actions"><button class="primary" disabled={busy}>{$t('保存')}</button>{#if current}<button type="button" on:click={() => edit()}>{$t('取消编辑')}</button>{/if}</div>
    </form>
  </section>
</main>

<style>
  .difficulty-section { margin: 26px 0; }
  .difficulty-list { display: grid; gap: 8px; max-width: 900px; }
  .difficulty-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 10px 12px; border: 1px solid #dcded4; background: #fafbf7; }
  .difficulty-info { display: grid; gap: 2px; }
  .difficulty-info small { color: #818b7f; }
  .difficulty-info small span { margin-right: 10px; }
  .difficulty-info .confirmed { color: #4d6448; font-weight: 600; }
  .difficulty-info .consensus { color: #285a78; font-weight: 600; }
  .vote-controls { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .vote-controls select { min-width: 60px; padding: 5px 6px; border: 1px solid #b9c3b3; background: #fff; }
  .vote-controls .small { padding: 4px 10px; font-size: 12px; }
  .frozen { color: #8b9587; font-size: 12px; }
</style>
