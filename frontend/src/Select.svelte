<script>
  import { createEventDispatcher, onDestroy, tick } from 'svelte';

  export let value = '';
  export let options = [];
  export let ariaLabel = '';
  export let disabled = false;

  const dispatch = createEventDispatcher();
  let open = false;
  let root, trigger;
  let list;
  let activeIndex = 0;
  const id = `select-${Math.random().toString(36).slice(2)}`;
  $: selected = options.find((option) => option.value === value);

  function close() {
    try { list?.hidePopover?.(); } catch { /* The list may not be in the top layer. */ }
    open = false;
    activeIndex = Math.max(0, options.findIndex((option) => option.value === value));
  }

  async function show(index = options.findIndex((option) => option.value === value)) {
    if (disabled) return;
    activeIndex = Math.max(0, index);
    open = true;
    await tick();
    if (!list) return;
    try { list.showPopover?.(); } catch { /* Older browsers still get the fixed-position fallback. */ }
    positionList();
    list?.querySelector(`[data-index="${activeIndex}"]`)?.focus();
  }

  function choose(option) {
    value = option.value;
    dispatch('value', option.value);
    close();
    trigger?.focus();
  }

  function positionList() {
    if (!open || !root || !list) return;
    const rect = root.getBoundingClientRect();
    const viewportPadding = 8;
    const gap = 6;
    const desiredHeight = Math.min(list.scrollHeight, 280, window.innerHeight * 0.4);
    const roomBelow = window.innerHeight - rect.bottom - viewportPadding - gap;
    const roomAbove = rect.top - viewportPadding - gap;
    const placeAbove = roomBelow < desiredHeight && roomAbove > roomBelow;
    const availableHeight = Math.max(96, Math.min(280, window.innerHeight * 0.4, placeAbove ? roomAbove : roomBelow));
    const height = Math.min(desiredHeight, availableHeight);
    const width = Math.min(rect.width, window.innerWidth - viewportPadding * 2);
    const left = Math.max(viewportPadding, Math.min(rect.left, window.innerWidth - width - viewportPadding));
    const top = placeAbove ? Math.max(viewportPadding, rect.top - height - gap) : Math.min(rect.bottom + gap, window.innerHeight - height - viewportPadding);
    list.style.left = `${left}px`;
    list.style.top = `${top}px`;
    list.style.width = `${width}px`;
    list.style.maxHeight = `${height}px`;
  }

  function onKeydown(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const direction = event.key === 'ArrowDown' ? 1 : -1;
      if (!open) show(direction > 0 ? 0 : options.length - 1);
      else {
        activeIndex = (activeIndex + direction + options.length) % options.length;
        tick().then(() => list?.querySelector(`[data-index="${activeIndex}"]`)?.focus());
      }
    } else if (event.key === 'Home' && open) {
      event.preventDefault(); activeIndex = 0; list?.querySelector('[data-index="0"]')?.focus();
    } else if (event.key === 'End' && open) {
      event.preventDefault(); activeIndex = options.length - 1; list?.querySelector(`[data-index="${activeIndex}"]`)?.focus();
    } else if (event.key === 'Escape' && open) {
      event.preventDefault(); close(); root?.querySelector('button')?.focus();
    } else if (event.key === 'Tab' && open) close();
  }

  function onOutside(event) {
    if (open && root && !root.contains(event.target) && !list?.contains(event.target)) close();
  }
  function onViewportChange() { positionList(); }
  if (typeof document !== 'undefined') document.addEventListener('pointerdown', onOutside);
  if (typeof window !== 'undefined') {
    window.addEventListener('resize', onViewportChange);
    window.addEventListener('scroll', onViewportChange, true);
  }
  onDestroy(() => {
    document.removeEventListener('pointerdown', onOutside);
    window.removeEventListener('resize', onViewportChange);
    window.removeEventListener('scroll', onViewportChange, true);
  });
</script>

<div class="select" class:open role="group" aria-label={ariaLabel || undefined} bind:this={root}>
  <button bind:this={trigger} type="button" aria-haspopup="listbox" aria-expanded={open} aria-controls={id} aria-label={ariaLabel || undefined} {disabled} on:keydown={onKeydown} on:click={() => open ? close() : show()}>
    <span>{selected?.label ?? value}</span><svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5 7.5 5 5 5-5" /></svg>
  </button>
  {#if open}
    <div class="select-options" popover="manual" role="listbox" id={id} aria-label={ariaLabel || undefined} tabindex="-1" bind:this={list} on:keydown={onKeydown}>
      {#each options as option, index}
        <button type="button" role="option" aria-selected={option.value === value} data-index={index} on:click={() => choose(option)}>{option.label}</button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .select { position: relative; min-width: 0; }
  .select > button { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; min-height: 44px; padding: 8px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); text-align: left; cursor: pointer; }
  .select > button:hover:not(:disabled), .select.open > button { border-color: var(--border-strong); }
  .select > button:focus-visible { outline: 2px solid var(--accent-bright); outline-offset: 2px; }
  .select > button svg { flex: none; width: 16px; height: 16px; fill: none; stroke: currentColor; stroke-width: 1.7; transition: transform var(--t-fast) var(--ease-out); }
  .select.open > button svg { transform: rotate(180deg); }
  .select-options { position: fixed; inset: auto; z-index: 100; margin: 0; max-height: min(280px, 40vh); overflow-x: hidden; overflow-y: auto; overscroll-behavior: contain; scrollbar-width: none; padding: 4px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); box-shadow: var(--shadow-md); }
  .select-options::-webkit-scrollbar { display: none; }
  .select-options button { display: block; width: 100%; min-height: 40px; padding: 8px 10px; border: 0; border-radius: 4px; background: transparent; color: var(--ink-secondary); text-align: left; white-space: normal; overflow-wrap: anywhere; cursor: pointer; }
  .select-options button:hover, .select-options button:focus-visible, .select-options button[aria-selected="true"] { outline: none; background: var(--surface-tint); color: var(--ink-primary); }
</style>
