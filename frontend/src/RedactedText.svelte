<script>
  import { t } from './i18n.js';
  import { isCipherText, redactCipher, shortenCipher } from './redact.js';
  export let text = '';
  export let hidden = false;
  $: cipher = hidden ? (isCipherText(text) ? shortenCipher(text) : redactCipher(text)) : '';
</script>
{#if hidden}<span class="redaction" title={$t('数据已隐藏')} aria-label={$t('数据已隐藏')}><span class="redaction-code">[INTERCEPTED 0x5F]</span> <span class="redaction-cipher">{cipher}</span> <span class="redaction-note">// {$t('数据已隐藏')}</span></span>{:else}{text}{/if}
<style>
  .redaction { display:inline; font-family:var(--font-mono),monospace; font-size:.92em; overflow-wrap:anywhere; }
  .redaction-code, .redaction-cipher { color:var(--danger); }
  .redaction-note { color:var(--accent-deep); }
</style>
