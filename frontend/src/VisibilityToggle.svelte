<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from './i18n.js';
  export let hidden = false;
  export let label = '';
  const dispatch = createEventDispatcher();
</script>
<label class="visibility-toggle"><input type="checkbox" checked={hidden} on:change={() => dispatch('toggle')} aria-label={`${$t('隐藏此项')}：${$t(label)}`} /> {$t('隐藏此项')}</label>
<style>
  .visibility-toggle { display:inline-flex; align-items:center; gap:7px; flex:none; color:var(--ink-secondary); cursor:pointer; font-size:12px; white-space:nowrap; }
  input { accent-color:var(--accent-bright); }
</style>
