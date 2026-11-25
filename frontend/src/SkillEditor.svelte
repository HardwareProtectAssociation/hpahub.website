<script>
  import { onDestroy, onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;

  const proficiencyOptions = ['初学者', '略懂一二', '比较熟练', '非常熟练', '你可以永远信任我', '我是这个领域的神！'];

  let loading = true;
  let error = '';
  let notice = '';
  let busy = false;
  let rows = [];

  function toRow(skill) {
    return {
      id: skill.userSkillId,
      name: skill.name,
      proficiency: proficiencyOptions.includes(skill.proficiency) ? skill.proficiency : '初学者',
      abilityNote: skill.abilityNote || '',
      linksText: (skill.links || []).join('\n'),
      images: skill.images || [],
      files: [],
      previews: [],
      error: '',
    };
  }

  function revokePreviews(row) {
    row.previews.forEach((url) => URL.revokeObjectURL(url));
    row.previews = [];
    row.files = [];
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/center', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || `请求失败（${response.status}）`);
      rows.forEach(revokePreviews);
      rows = (body.skills || []).map(toRow);
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  }

  function addRow() {
    rows = [...rows, {
      id: '',
      name: '',
      proficiency: '初学者',
      abilityNote: '',
      linksText: '',
      images: [],
      files: [],
      previews: [],
      error: '',
    }];
  }

  function selectFiles(row, event) {
    const selected = Array.from(event.currentTarget.files || []);
    event.currentTarget.value = '';
    if (!selected.length) return;
    if (row.images.length + row.files.length + selected.length > 4) {
      row.error = '最多 4 张图片。';
      return;
    }
    row.error = '';
    row.files = [...row.files, ...selected];
    row.previews = [...row.previews, ...selected.map((file) => URL.createObjectURL(file))];
  }

  function removePreview(row, index) {
    URL.revokeObjectURL(row.previews[index]);
    row.previews = row.previews.filter((_, itemIndex) => itemIndex !== index);
    row.files = row.files.filter((_, itemIndex) => itemIndex !== index);
  }

  async function removeImage(row, url) {
    if (!row.id) return;
    const name = decodeURIComponent(url.split('/').pop());
    busy = true;
    row.error = '';
    try {
      const response = await fetch(`/api/hpahub/me/skills/${row.id}/images/${encodeURIComponent(name)}`, {
        method: 'DELETE',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error('删除图片失败。');
      await load();
      notice = '技能已保存。';
    } catch (err) {
      row.error = err.message || String(err);
    } finally {
      busy = false;
    }
  }

  async function saveRow(row) {
    const name = row.name.trim();
    if (!name) {
      row.error = '请填写技能名称。';
      return;
    }
    const links = row.linksText.split('\n').map((line) => line.trim()).filter(Boolean);
    busy = true;
    row.error = '';
    notice = '';
    try {
      const response = await fetch(row.id ? `/api/hpahub/me/skills/${row.id}` : '/api/hpahub/me/skills', {
        method: row.id ? 'PATCH' : 'POST',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({ name, proficiency: row.proficiency, abilityNote: row.abilityNote, links }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '保存失败，请重试。');
      const id = row.id || result.userSkillId;
      if (row.files.length) {
        const body = new FormData();
        row.files.forEach((file) => body.append('images', file));
        const upload = await fetch(`/api/hpahub/me/skills/${id}/images`, {
          method: 'POST',
          headers: token ? { Authorization: `Bearer ${token}` } : {},
          body,
        });
        if (!upload.ok) throw new Error('图片上传失败。');
      }
      notice = '技能已保存。';
      await load();
    } catch (err) {
      row.error = err.message || String(err);
    } finally {
      busy = false;
    }
  }

  async function deleteRow(row) {
    if (!row.id) {
      rows = rows.filter((item) => item !== row);
      return;
    }
    if (!confirm($t('确定删除这个技能吗？'))) return;
    busy = true;
    row.error = '';
    try {
      const response = await fetch(`/api/hpahub/me/skills/${row.id}`, {
        method: 'DELETE',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error('删除失败，请重试。');
      notice = '技能已删除。';
      await load();
    } catch (err) {
      row.error = err.message || String(err);
    } finally {
      busy = false;
    }
  }

  onMount(load);
  onDestroy(() => rows.forEach(revokePreviews));
</script>

<main class="dashboard skill-editor">
  <a href="/me">{$t('返回个人中心')}</a>
  <div class="page-heading">
    <div>
      <h1>{$t('技能管理')}</h1>
      <p>{$t('编辑技能名称、熟练度、说明、图片和网页链接。')}</p>
    </div>
    <button class="primary" type="button" on:click={addRow} disabled={busy}>{$t('新增技能')}</button>
  </div>

  {#if loading}
    <p role="status">{$t('正在加载…')}</p>
  {:else if error}
    <div class="message error" role="alert">{$t(error)}</div>
    <button class="secondary" on:click={load}>{$t('重试')}</button>
  {:else}
    {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}
    {#if !rows.length}<p>{$t('还没有添加技能')}</p>{/if}
    <div class="skill-forms">
      {#each rows as row}
        <form class="skill-form" on:submit|preventDefault={() => saveRow(row)}>
          {#if row.error}<p class="message error" role="alert">{$t(row.error)}</p>{/if}
          <div class="form-grid">
            <label>{$t('技能名称')}<input bind:value={row.name} maxlength="80" required placeholder={$t('输入技能名称')} /></label>
            <label>{$t('熟练程度')}<select bind:value={row.proficiency}>{#each proficiencyOptions as option}<option value={option}>{$t(option)}</option>{/each}</select></label>
          </div>
          <label>{$t('能力说明')}<textarea bind:value={row.abilityNote} maxlength="500" rows="3" placeholder={$t('简单说明你的经验或作品')}></textarea></label>
          <label>{$t('网页链接（每行一个）')}<textarea bind:value={row.linksText} rows="2" placeholder="https://example.com"></textarea></label>
          {#if row.images.length || row.previews.length}
            <div class="image-grid">
              {#each row.images as url}
                <div class="image-item">
                  <img src={url} alt="" loading="lazy" />
                  <button type="button" class="small danger" on:click={() => removeImage(row, url)} disabled={busy}>{$t('删除图片')}</button>
                </div>
              {/each}
              {#each row.previews as preview, index}
                <div class="image-item">
                  <img src={preview} alt="" />
                  <button type="button" class="small" on:click={() => removePreview(row, index)}>{$t('取消选择')}</button>
                </div>
              {/each}
            </div>
          {/if}
          <label class="file-label">{$t('添加图片（最多 4 张）')}<input type="file" accept="image/jpeg,image/png,image/webp,image/gif" multiple on:change={(event) => selectFiles(row, event)} /></label>
          <div class="form-actions">
            <button class="primary" type="submit" disabled={busy}>{busy ? $t('正在保存…') : $t('保存技能')}</button>
            <button type="button" on:click={() => deleteRow(row)} disabled={busy}>{$t('删除技能')}</button>
          </div>
        </form>
      {/each}
    </div>
  {/if}
</main>

<style>
  .skill-editor { width: min(1000px, 88%); }
  .skill-editor > a { color: #345d3d; text-underline-offset: 3px; font-size: 13px; }
  .skill-forms { display: grid; gap: 18px; margin-top: 24px; }
  .skill-form { display: grid; gap: 14px; padding: 22px; border: 1px solid #dcded4; background: #fafbf7; }
  .skill-form .message { margin: 0; }
  .form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .skill-form label { display: grid; gap: 6px; color: #52695a; font-size: 13px; }
  .skill-form input:not([type=file]), .skill-form select, .skill-form textarea { width: 100%; padding: 9px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .skill-form textarea { resize: vertical; }
  .file-label { display: inline-flex; align-items: center; gap: 8px; width: fit-content; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .image-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 10px; }
  .image-item { display: grid; gap: 6px; }
  .image-item img { width: 100%; height: 100px; object-fit: cover; border: 1px solid #dcded4; background: #fff; }
  .image-item button { justify-self: start; }
  .form-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  @media (max-width: 700px) {
    .form-grid { grid-template-columns: 1fr; }
    .skill-editor { width: 90%; }
  }
</style>
