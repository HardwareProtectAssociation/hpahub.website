<script>
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { t } from './i18n.js';
  import { cropRect } from './avatarCrop.js';
  const dispatch = createEventDispatcher();
  let image, url = '', canvas, x = 50, y = 50, zoom = 1, error = '', busy = false, dragOrigin;
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
  function choose(event) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = '';
    if (!file) return;
    dispatch('pending', true);
    if (url) URL.revokeObjectURL(url);
    url = URL.createObjectURL(file); image = null; width = height = 0; error = '';
    x = y = 50; zoom = 1;
    const next = new Image();
    next.onload = () => { if (next.src !== url) return; image = next; width = next.naturalWidth; height = next.naturalHeight; };
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
  function cancel() { if (url) URL.revokeObjectURL(url); url = ''; image = null; width = height = 0; dispatch('pending', false); }
  onDestroy(cancel);
</script>

<label class="file-label">{$t('更换头像')}<input type="file" accept="image/jpeg,image/png,image/webp,image/gif" on:change={choose} /></label>
{#if error}<p role="alert">{$t(error)}</p>{/if}
{#if image}
  <div class="crop" aria-label={$t('裁切头像')}>
    <strong>{$t('裁切头像')}</strong>
    <div class="crop-stage" aria-label={$t('头像裁切预览')} on:pointerdown={startDrag} on:pointermove={moveCrop} on:pointerup={stopDrag} on:pointercancel={stopDrag}>
      <canvas bind:this={canvas} aria-hidden="true"></canvas>
      <span class="crop-mask" aria-hidden="true"></span>
    </div>
    <label>{$t('缩放')}<input type="range" min="1" max="4" step="0.01" bind:value={zoom} /></label>
    <label>{$t('水平位置')}<input type="range" min="0" max="100" bind:value={x} aria-label={$t('水平位置')} /></label>
    <label>{$t('垂直位置')}<input type="range" min="0" max="100" bind:value={y} aria-label={$t('垂直位置')} /></label>
    <div><button type="button" on:click={confirmCrop} disabled={busy}>{$t('确认裁切')}</button> <button type="button" on:click={cancel} disabled={busy}>{$t('取消')}</button></div>
  </div>
{/if}

<style>
  .crop { display: grid; gap: 12px; width: min(360px, 100%); margin-top: 12px; padding: 16px; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--surface-elevated); box-shadow: var(--shadow-md); }
  .crop-stage { position: relative; width: 100%; aspect-ratio: 1; overflow: hidden; border-radius: var(--radius-sm); background: var(--surface-soft); cursor: grab; touch-action: none; }
  .crop-stage:active { cursor: grabbing; }
  canvas { display: block; width: 100%; height: 100%; }
  .crop-mask { position: absolute; inset: 0; pointer-events: none; }
  .crop-mask::before { content: ''; position: absolute; inset: 8%; border: 1px solid rgba(235, 255, 241, .92); border-radius: 50%; box-shadow: 0 0 0 999px rgba(3, 10, 6, .56); }
  .crop-stage:focus-visible { outline: 2px solid var(--accent-bright); outline-offset: 3px; }
  .crop label { display: grid; gap: 6px; font-size: 12.5px; color: var(--ink-secondary); }
  .crop input { width: 100%; accent-color: var(--accent); }
  .crop button { padding: 7px 14px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--accent-deep); font-size: 13px; font-weight: 600; }
  .crop button:hover:not(:disabled) { background: var(--surface-tint); border-color: var(--border-strong); }
  @media (max-width: 520px) { .crop { width: min(100%, 320px); padding: 12px; } }
</style>
