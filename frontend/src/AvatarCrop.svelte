<script>
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { t } from './i18n.js';
  import { cropRect } from './avatarCrop.js';
  const dispatch = createEventDispatcher();
  let image, url = '', canvas, x = 50, y = 50, zoom = 1, error = '', busy = false;
  let width = 0, height = 0;
  $: if (canvas && image && width && height) { x; y; zoom; draw(canvas, 288); }

  function draw(target, size) {
    const { side, left, top } = cropRect(width, height, zoom, x, y);
    target.width = target.height = size;
    target.getContext('2d').drawImage(image, left, top, side, side, 0, 0, size, size);
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
      draw(output, side);
      const full = await new Promise(resolve => output.toBlob(resolve, 'image/png'));
      draw(output, 144);
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
    <canvas bind:this={canvas} aria-label={$t('头像裁切预览')}></canvas>
    <label>{$t('缩放')}<input type="range" min="1" max="4" step="0.01" bind:value={zoom} /></label>
    <label>{$t('水平位置')}<input type="range" min="0" max="100" bind:value={x} /></label>
    <label>{$t('垂直位置')}<input type="range" min="0" max="100" bind:value={y} /></label>
    <div><button type="button" on:click={confirmCrop} disabled={busy}>{$t('确认裁切')}</button> <button type="button" on:click={cancel} disabled={busy}>{$t('取消')}</button></div>
  </div>
{/if}

<style>
  .crop { display: grid; gap: 10px; width: min(288px, 100%); margin-top: 12px; }
  canvas { display: block; width: 100%; aspect-ratio: 1; background: #eef1eb; }
  .crop label { display: grid; gap: 4px; }
  .crop input { width: 100%; }
</style>
