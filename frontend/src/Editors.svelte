<script>
  import { onMount, onDestroy } from 'svelte';
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
  let busy = false;
  const base = manage ? '/api/hpahub/manage/showcase' : '/api/hpahub/me/cards';
  const headers = () => ({ Authorization: `Bearer ${token}` });
  async function request(url, options = {}) {
    const response = await fetch(url, { ...options, headers: { ...headers(), ...options.headers }, cache: 'no-store' });
    if (!response.ok) { const result = await response.json().catch(() => ({})); throw new Error(result.message || `请求失败（${response.status}）`); }
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
    const [list, nextProfile] = await Promise.all([request(base), manage ? null : request('/api/hpahub/me/profile')]);
    records = list.items;
    if (nextProfile) profile = nextProfile;
    await Promise.all([...(profile.avatarUrl && !manage ? [preview(profile.avatarUrl)] : []), ...records.flatMap(record => (record.imageUrls || []).map(preview))]);
  }
  onMount(async () => { try { await load(); } catch (e) { error = e.message; } });
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
    } catch (e) { error = e.message; } finally { busy = false; }
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
    } catch (e) { error = e.message; } finally { busy = false; }
  }
  async function remove(record) {
    if (!confirm(`确定删除“${record.title}”吗？`)) return;
    busy = true; error = ''; notice = '';
    try { await request(base + '/' + encodeURIComponent(record.id), { method: 'DELETE' }); edit(); await load(); notice = '已删除。'; }
    catch (e) { error = e.message; } finally { busy = false; }
  }
</script>

<main class="dashboard public-page editor-page">
  <h1>{manage ? '项目展示管理' : '个人设置'}</h1>
  {#if error}<p class="message error" role="alert">{error}</p>{/if}{#if notice}<p class="message success" role="status">{notice}</p>{/if}
  {#if !manage}
    <section><h2>成员资料</h2><form class="editor-form" on:submit|preventDefault={saveProfile}>
      {#if avatarPreview && !removeAvatar}<img class="avatar" src={avatarPreview} alt="新头像预览" />{:else if profile.avatarUrl && previews[profile.avatarUrl] && !removeAvatar}<img class="avatar" src={previews[profile.avatarUrl]} alt="当前头像" />{/if}
      <label>昵称 <input bind:value={profile.name} maxlength="80" required={profile.profilePublic} /></label>
      <label>简介 <textarea bind:value={profile.bio} maxlength="600" rows="4"></textarea></label>
      <label>头像（JPEG、PNG、WebP 或 GIF，最多 5 MiB） <input type="file" bind:this={avatarInput} accept="image/jpeg,image/png,image/webp,image/gif" on:change={selectAvatar} /></label>
      {#if profile.avatarUrl}<label class="check-label"><input type="checkbox" bind:checked={removeAvatar} on:change={() => { if (removeAvatar) { avatar = null; avatarInput.value = ''; } }} /> 删除头像</label>{/if}
      <label class="check-label"><input type="checkbox" bind:checked={profile.profilePublic} /> 公开我的资料和卡片</label>
      <button class="primary" disabled={busy}>保存资料</button>
    </form></section>
  {/if}
  <section><h2>{manage ? '项目' : '我的卡片'}</h2>
    {#if records.length}<div class="editor-list">{#each records as record}<article><h3>{record.title} {#if manage}<small>{record.published ? '已发布' : '草稿'}</small>{/if}</h3><p>{record.text}</p><div class="image-grid">{#each record.imageUrls || [] as url}{#if previews[url]}<a href={previews[url]} target="_blank" rel="noreferrer"><img src={previews[url]} alt={record.title} /></a>{/if}{/each}</div><div class="editor-actions"><button type="button" on:click={() => edit(record)}>编辑</button><button type="button" on:click={() => remove(record)} disabled={busy}>删除</button></div></article>{/each}</div>{:else}<p>暂无{manage ? '项目' : '卡片'}。</p>{/if}
    <h2>{current ? '编辑' : '新建'}{manage ? '项目' : '卡片'}</h2><form class="editor-form" on:submit|preventDefault={saveRecord}>
      <label>标题 <input bind:value={title} maxlength="80" required /></label><label>文字 <textarea bind:value={text} maxlength="280" rows="4"></textarea></label>
      {#if current?.images?.length}<fieldset><legend>保留图片</legend>{#each current.images as image, index}<label class="check-label"><input type="checkbox" value={image} bind:group={keepImages} /> {#if previews[current.imageUrls[index]]}<img class="editor-thumb" src={previews[current.imageUrls[index]]} alt="" />{/if} {image}</label>{/each}</fieldset>{/if}
      <label>添加图片（最多 4 张，每张最多 5 MiB） <input type="file" bind:this={imagesInput} accept="image/jpeg,image/png,image/webp,image/gif" multiple on:change={selectFiles} /></label>
      {#if uploadPreviews.length}<div class="image-grid">{#each uploadPreviews as url}<img src={url} alt="待上传图片预览" />{/each}</div>{/if}
      {#if manage}<label class="check-label"><input type="checkbox" bind:checked={published} /> 发布项目</label>{/if}
      <div class="editor-actions"><button class="primary" disabled={busy}>保存</button>{#if current}<button type="button" on:click={() => edit()}>取消编辑</button>{/if}</div>
    </form>
  </section>
</main>
