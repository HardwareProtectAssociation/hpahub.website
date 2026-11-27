<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { t } from './i18n.js';
  import Select from './Select.svelte';

  export let token;

  const proficiencyOptions = ['初学者', '略懂一二', '比较熟练', '非常熟练', '你可以永远信任我', '我是这个领域的神！'];

  let loading = true;
  let error = '';
  let notice = '';
  let busy = false;
  let rows = [];
  let editingRow = null;
  let editDialog;
  let imagesInput;

  function toRow(skill) {
    return {
      id: skill.userSkillId,
      name: skill.name,
      proficiency: proficiencyOptions.includes(skill.proficiency) ? skill.proficiency : '初学者',
      abilityNote: skill.abilityNote || '',
      linksText: (skill.links || []).join('\n'),
      images: skill.images || [],
      featured: !!skill.featured,
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
    openRow({
      id: '',
      name: '',
      proficiency: '初学者',
      abilityNote: '',
      linksText: '',
      images: [],
      featured: false,
      files: [],
      previews: [],
      error: '',
    });
  }

  async function openRow(row) {
    editingRow = { ...row, images: [...row.images], files: [...row.files], previews: [...row.previews] };
    await tick();
    editDialog?.showModal();
  }

  function cancelEdit() {
    if (editingRow && !rows.includes(editingRow)) revokePreviews(editingRow);
    editingRow = null;
    editDialog?.close();
  }

  function selectFiles(row, event) {
    const selected = Array.from(event.currentTarget.files || []);
    event.currentTarget.value = '';
    if (!selected.length) return;
    if (selected.some((file) => file.size > 2 * 1024 * 1024)) {
      row.error = '图片不能超过 2 MiB。';
      return;
    }
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
      editingRow.images = editingRow.images.filter((image) => image !== url);
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
        body: JSON.stringify({ name, proficiency: row.proficiency, abilityNote: row.abilityNote, links, featured: row.featured }),
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
      cancelEdit();
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
        <p class="meta">{$t('勾选后显示在公开主页的技能展示中，数量不限。')}</p>
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
    <ul class="skill-list">
      {#each rows as row}
        <li class="skill-row">
          <div><strong>{row.name}</strong><span class="skill-level">{$t(row.proficiency)}</span>{#if row.featured}<span class="skill-featured">{$t('公开展示')}</span>{/if}{#if row.abilityNote}<p>{row.abilityNote}</p>{/if}</div>
          <div class="row-actions"><button type="button" class="secondary" on:click={() => openRow(row)}>{$t('编辑')}</button><button type="button" class="secondary" on:click={() => deleteRow(row)} disabled={busy}>{$t('删除技能')}</button></div>
        </li>
      {/each}
    </ul>
  {/if}
</main>

{#if editingRow}
  <dialog class="skill-dialog" bind:this={editDialog} on:cancel|preventDefault={cancelEdit}>
    <form class="skill-form" on:submit|preventDefault={() => saveRow(editingRow)}>
      <div class="dialog-heading"><h2>{editingRow.id ? $t('编辑技能') : $t('新增技能')}</h2><button type="button" class="dialog-close" aria-label={$t('关闭')} on:click={cancelEdit}>×</button></div>
      {#if editingRow.error}<p class="message error" role="alert">{$t(editingRow.error)}</p>{/if}
      <div class="form-grid">
        <label>{$t('技能名称')}<input bind:value={editingRow.name} maxlength="80" required placeholder={$t('输入技能名称')} /></label>
        <div class="skill-control"><span>{$t('熟练程度')}</span><Select bind:value={editingRow.proficiency} ariaLabel={$t('熟练程度')} options={proficiencyOptions.map((option) => ({ value: option, label: $t(option) }))} /></div>
      </div>
      <label>{$t('能力说明')}<textarea bind:value={editingRow.abilityNote} maxlength="500" rows="3" placeholder={$t('简单说明你的经验或作品')}></textarea></label>
      <label>{$t('网页链接（每行一个）')}<textarea bind:value={editingRow.linksText} rows="2" placeholder="https://example.com"></textarea></label>
      {#if editingRow.images.length || editingRow.previews.length}
        <div class="image-grid">
          {#each editingRow.images as url}
            <div class="image-item"><img src={url} alt="" loading="lazy" /><button type="button" class="small danger" on:click={() => removeImage(editingRow, url)} disabled={busy}>{$t('删除图片')}</button></div>
          {/each}
          {#each editingRow.previews as preview, index}
            <div class="image-item"><img src={preview} alt="" /><button type="button" class="small" on:click={() => removePreview(editingRow, index)}>{$t('取消选择')}</button></div>
          {/each}
        </div>
      {/if}
      <input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" multiple tabindex="-1" bind:this={imagesInput} on:change={(event) => selectFiles(editingRow, event)} />
      <button type="button" class="secondary image-picker" on:click={() => imagesInput?.click()}>{$t('添加图片（最多 4 张）')}</button>
      <label class="check-label"><input type="checkbox" bind:checked={editingRow.featured} /> {$t('在公开主页展示')}</label>
      <div class="form-actions"><button class="primary" type="submit" disabled={busy}>{busy ? $t('正在保存…') : $t('保存技能')}</button><button type="button" class="secondary" on:click={cancelEdit} disabled={busy}>{$t('取消')}</button></div>
    </form>
  </dialog>
{/if}

<style>
  .skill-editor { width: min(1000px, 88%); }
  .skill-editor > a { color: var(--accent-deep); text-underline-offset: 3px; font-size: 13px; font-weight: 600; }
  .skill-list { display: grid; gap: 0; margin: 24px 0 0; padding: 0; list-style: none; border-top: 1px solid var(--border-subtle); }
  .skill-row { display: flex; align-items: center; justify-content: space-between; gap: 18px; padding: 14px 4px; border-bottom: 1px solid var(--border-subtle); }
  .skill-row strong { font-size: 15px; }
  .skill-row p { margin: 5px 0 0; color: var(--ink-muted); font-size: 13px; white-space: pre-wrap; }
  .skill-level, .skill-featured { margin-left: 10px; color: var(--ink-muted); font-size: 12px; }
  .skill-featured { color: var(--accent-deep); }
  .row-actions { display: flex; gap: 8px; }
  .row-actions .secondary { width: auto; min-width: 82px; justify-content: center; }
  .skill-dialog { width: min(720px, calc(100% - 32px)); max-height: min(90dvh, 900px); padding: 0; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--surface-elevated); color: var(--ink-primary); box-shadow: var(--shadow-md); }
  .skill-dialog::backdrop { background: #0009; }
  .skill-form { display: grid; gap: 14px; padding: 24px; }
  .dialog-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .dialog-heading h2 { margin: 0; font-size: 20px; }
  .dialog-close { min-width: 44px; min-height: 44px; border: 0; background: none; color: var(--ink-muted); font-size: 26px; }
  .skill-form .message { margin: 0; }
  .form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .skill-form label { display: grid; gap: 6px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; }
  .skill-form label.check-label { display: flex; align-items: center; gap: 8px; }
  .meta { margin: 6px 0 0; color: var(--ink-subtle); font-size: 12px; }
  .skill-form input:not([type=checkbox]), .skill-form textarea { width: 100%; min-height: 44px; padding: 9px 11px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; transition: border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .skill-form input:not([type=checkbox]):focus, .skill-form textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  .skill-form input[type=checkbox] { width: auto; margin: 0; }
  .skill-form textarea { resize: vertical; }
  .image-picker { width: fit-content; justify-content: center; }
  .image-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 10px; }
  .image-item { display: grid; gap: 6px; }
  .image-item img { width: 100%; height: 100px; object-fit: cover; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-elevated); }
  .image-item button { justify-self: start; padding: 4px 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-secondary); font-size: 12px; }
  .form-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  .form-actions button:not(.primary) { width: auto; min-width: 92px; justify-content: center; }
  @media (max-width: 700px) {
    .form-grid { grid-template-columns: 1fr; }
    .skill-editor { width: 90%; }
    .skill-row { align-items: flex-start; flex-direction: column; }
    .row-actions { width: 100%; }
    .row-actions button { flex: 1; }
    .skill-form { padding: 18px; }
  }
</style>
