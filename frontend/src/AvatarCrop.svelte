<script>
  import { createEventDispatcher, onDestroy, tick } from 'svelte';
  import { t } from './i18n.js';
  import { cropRect } from './avatarCrop.js';
  const dispatch = createEventDispatcher();
  let image, url = '', canvas, fileInput, cropDialog, x = 50, y = 50, zoom = 1, error = '', busy = false, dragOrigin;
  let width = 0, height = 0;
  $: if (canvas && image && width && height) { x; y; zoom; drawPreview(canvas, 360); }

  function drawCrop(target, size) {
    const { side, left, top } = cropRect(width, height, zoom, x, y);
    target.width = target.height = size;
    target.getContext('2d').drawImage(image, left, top, side, side, 0, 0, size, size);
  }
  function drawPreview(target, size) {
    const { side, left, top } = cropRect(width, height, zoom, x, y);
    const scale = size / side;
    target.width = target.height = size;
    target.getContext('2d').drawImage(image, -left * scale, -top * scale, width * scale, height * scale);
  }
  function startDrag(event) {
    if (busy || event.button !== 0) return;
    dragOrigin = { pointerId: event.pointerId, x: event.clientX, y: event.clientY, cropX: Number(x), cropY: Number(y) };
    event.currentTarget.setPointerCapture(event.pointerId);
  }
  function moveCrop(event) {
    if (!dragOrigin || dragOrigin.pointerId !== event.pointerId) return;
    const side = Math.min(width, height) / Number(zoom);
    const scale = event.currentTarget.getBoundingClientRect().width / side;
    const maxX = Math.max(1, width - side);
    const maxY = Math.max(1, height - side);
    x = Math.max(0, Math.min(100, dragOrigin.cropX - ((event.clientX - dragOrigin.x) / scale / maxX) * 100));
    y = Math.max(0, Math.min(100, dragOrigin.cropY - ((event.clientY - dragOrigin.y) / scale / maxY) * 100));
  }
  function stopDrag(event) {
    if (dragOrigin?.pointerId === event.pointerId) dragOrigin = null;
  }
  function moveWithKeys(event) {
    const step = event.shiftKey ? 5 : 1;
    const side = Math.min(width, height) / Number(zoom);
    const maxX = Math.max(1, width - side);
    const maxY = Math.max(1, height - side);
    if (event.key === 'ArrowLeft') x = Math.min(100, Number(x) + (step / maxX) * 100);
    else if (event.key === 'ArrowRight') x = Math.max(0, Number(x) - (step / maxX) * 100);
    else if (event.key === 'ArrowUp') y = Math.min(100, Number(y) + (step / maxY) * 100);
    else if (event.key === 'ArrowDown') y = Math.max(0, Number(y) - (step / maxY) * 100);
    else return;
    event.preventDefault();
  }
  function openPicker() { if (!busy) fileInput?.click(); }
  function choose(event) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = '';
    if (!file) return;
    dispatch('pending', true);
    if (url) URL.revokeObjectURL(url);
    url = URL.createObjectURL(file); image = null; width = height = 0; error = '';
    x = y = 50; zoom = 1;
    const next = new Image();
    next.onload = async () => {
      if (next.src !== url) return;
      image = next; width = next.naturalWidth; height = next.naturalHeight;
      await tick();
      cropDialog?.showModal();
    };
    next.onerror = () => { if (next.src === url) { cancel(); error = '无法读取图片。'; } };
    next.src = url;
  }
  async function confirmCrop() {
    busy = true; error = '';
    try {
      const output = document.createElement('canvas');
      const side = Math.min(512, Math.max(1, Math.round(Math.min(width, height) / Number(zoom))));
      drawCrop(output, side);
      const full = await new Promise(resolve => output.toBlob(resolve, 'image/png'));
      drawCrop(output, 144);
      const thumb = await new Promise(resolve => output.toBlob(resolve, 'image/webp', 0.85));
      if (!full || !thumb || thumb.type !== 'image/webp') throw new Error();
      dispatch('crop', { avatar: new File([full], 'avatar.png', { type: 'image/png' }), thumb: new File([thumb], 'avatar.webp', { type: 'image/webp' }) });
      cancel();
    } catch { error = '无法处理图片，请重试。'; }
    finally { busy = false; }
  }
  function cancel() {
    if (cropDialog?.open) cropDialog.close();
    if (url) URL.revokeObjectURL(url);
    url = ''; image = null; width = height = 0; dispatch('pending', false);
  }
  onDestroy(cancel);
</script>

<button type="button" class="file-label" on:click={openPicker}>{$t('更换头像')}</button>
<input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" bind:this={fileInput} on:change={choose} />
{#if error}<p role="alert">{$t(error)}</p>{/if}
{#if image}
  <dialog class="crop-dialog" aria-label={$t('裁切头像')} bind:this={cropDialog} on:cancel|preventDefault={cancel}>
    <div class="crop">
      <div class="crop-heading"><strong>{$t('裁切头像')}</strong><button type="button" class="crop-close" aria-label={$t('取消')} on:click={cancel}>×</button></div>
      <button type="button" class="crop-stage" aria-label={$t('头像裁切预览')} aria-keyshortcuts="ArrowLeft ArrowRight ArrowUp ArrowDown" on:keydown={moveWithKeys} on:pointerdown={startDrag} on:pointermove={moveCrop} on:pointerup={stopDrag} on:pointercancel={stopDrag}>
        <canvas bind:this={canvas} aria-hidden="true"></canvas>
        <span class="crop-mask" aria-hidden="true"></span>
      </button>
      <label>{$t('缩放')}<input type="range" min="1" max="4" step="0.01" bind:value={zoom} /></label>
      {#if error}<p role="alert">{$t(error)}</p>{/if}
      <div class="crop-actions"><button type="button" class="crop-confirm" on:click={confirmCrop} disabled={busy}>{$t('确认裁切')}</button><button type="button" class="crop-cancel" on:click={cancel} disabled={busy}>{$t('取消')}</button></div>
    </div>
  </dialog>
{/if}

<style>
  .file-label { padding: 0; border: 0; background: none; color: var(--accent-deep); font: inherit; font-size: 13px; font-weight: 600; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .crop-dialog { width: min(460px, calc(100% - 32px)); max-height: min(90dvh, 720px); overflow: auto; padding: 0; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--surface-elevated); color: var(--ink-primary); box-shadow: var(--shadow-lg, var(--shadow-md)); }
  .crop-dialog::backdrop { background: #000a; backdrop-filter: blur(3px); }
  .crop { display: grid; gap: 16px; width: 100%; padding: 20px; }
  .crop-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .crop-heading strong { font-size: 18px; }
  .crop-close { width: 40px; height: 40px; border: 0; border-radius: 50%; background: var(--surface-tint); color: var(--ink-secondary); font-size: 25px; line-height: 1; cursor: pointer; }
  .crop-close:hover { background: var(--border-subtle); }
  .crop-stage { position: relative; display: block; width: 100%; aspect-ratio: 1; overflow: hidden; padding: 0; border: 0; border-radius: var(--radius-sm); background: var(--surface-soft); color: inherit; text-align: left; cursor: grab; touch-action: none; }
  .crop-stage:active { cursor: grabbing; }
  .crop-stage:focus-visible { outline: 2px solid var(--accent-bright); outline-offset: 3px; }
  canvas { display: block; width: 100%; height: 100%; }
  .crop-mask { position: absolute; inset: 0; pointer-events: none; }
  .crop-mask::before { content: ''; position: absolute; inset: 0; border: 1px solid rgba(235, 255, 241, .92); border-radius: 50%; box-shadow: 0 0 0 999px rgba(3, 10, 6, .58); }
  .crop label { display: grid; gap: 6px; font-size: 12.5px; color: var(--ink-secondary); }
  .crop input { width: 100%; accent-color: var(--accent); }
  .crop-actions { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 8px; }
  .crop-actions button { min-height: 40px; padding: 8px 14px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); font-size: 13px; font-weight: 600; cursor: pointer; }
  .crop-confirm { background: var(--accent); color: var(--ink-on-accent, #06210f); }
  .crop-confirm:hover:not(:disabled) { filter: brightness(1.06); }
  .crop-cancel { background: var(--surface-elevated); color: var(--ink-secondary); }
  .crop-actions button:disabled { opacity: .6; cursor: wait; }
  @media (max-width: 520px) { .crop { padding: 16px; } }
</style>
