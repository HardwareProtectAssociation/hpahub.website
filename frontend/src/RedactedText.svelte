<script>
  import { t } from './i18n.js';
  export let text = '';
  export let hidden = false;
  $: mask = '█'.repeat(Array.from(String(text || '')).length);
</script>
{#if hidden}<span class="redaction" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}>{mask}</span>{:else}{text}{/if}
<style>
  .redaction { color: var(--ink-primary); background: var(--ink-primary); display:inline; white-space:pre-wrap; overflow-wrap:anywhere; }
</style>
