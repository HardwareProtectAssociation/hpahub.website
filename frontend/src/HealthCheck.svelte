<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;

  const moods = [
    { value: 'great', label: '很好', icon: '☀' },
    { value: 'good', label: '不错', icon: '🙂' },
    { value: 'okay', label: '一般', icon: '•' },
    { value: 'low', label: '低落', icon: '☁' },
    { value: 'bad', label: '很差', icon: '☂' },
  ];

  let loading = true;
  let open = false;
  let saving = false;
  let error = '';
  let data = null;
  let mood = 'good';
  let rested = false;
  let exercised = false;
  let ateOnTime = false;
  let note = '';

  async function load() {
    loading = true;
    try {
      const response = await fetch('/api/hpahub/me/health', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        cache: 'no-store',
      });
      if (!response.ok) return;
      data = await response.json();
      if (!data.enabled || data.checkedInToday) return;
      const dismissed = localStorage.getItem(`health-dismissed:${data.date}:${data.userId}`);
      if (dismissed) return;
      open = true;
    } finally {
      loading = false;
    }
  }

  function dismissToday() {
    if (data) localStorage.setItem(`health-dismissed:${data.date}:${data.userId}`, '1');
    open = false;
  }

  async function disableDaily() {
    try {
      const response = await fetch('/api/hpahub/me/health/settings', {
        method: 'PATCH',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: false }),
      });
      if (!response.ok) throw new Error('保存失败，请重试。');
      if (data) data.enabled = false;
      open = false;
      window.dispatchEvent(new Event('hpahub:health-updated'));
    } catch (err) {
      error = err.message || String(err);
    }
  }

  async function submit() {
    saving = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/health/checkin', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ mood, rested, exercised, ateOnTime, note }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || '保存失败，请重试。');
      if (data) {
        data.checkedInToday = true;
        data.today = body;
      }
      open = false;
      window.dispatchEvent(new Event('hpahub:health-updated'));
    } catch (err) {
      error = err.message || String(err);
    } finally {
      saving = false;
    }
  }

  onMount(load);
</script>

{#if open}
  <div class="health-backdrop" role="presentation">
    <section class="health-modal" role="dialog" aria-modal="true" aria-labelledby="health-title">
      <div class="health-head">
        <div><h2 id="health-title">{$t('每日健康评估')}</h2><p>{$t('每天花一分钟看看自己的状态。')}</p></div>
        <button type="button" class="health-close" aria-label={$t('今天稍后')} on:click={dismissToday}>×</button>
      </div>
      {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
      <p class="health-label">{$t('今天感觉怎么样？')}</p>
      <div class="mood-options">
        {#each moods as option}
          <button type="button" class:active={mood === option.value} class="mood-option" on:click={() => mood = option.value}>
            <span aria-hidden="true">{option.icon}</span>
            {$t(option.label)}
          </button>
        {/each}
      </div>
      <div class="health-checks">
        <label><input type="checkbox" bind:checked={rested} /> {$t('休息够了')}</label>
        <label><input type="checkbox" bind:checked={exercised} /> {$t('今天锻炼了')}</label>
        <label><input type="checkbox" bind:checked={ateOnTime} /> {$t('按时吃饭')}</label>
      </div>
      <label class="health-note">{$t('想补充点什么（可选）')}<textarea bind:value={note} maxlength="300" rows="2"></textarea></label>
      <div class="health-actions">
        <button type="button" class="primary" on:click={submit} disabled={saving}>{saving ? $t('正在保存…') : $t('提交评估')}</button>
        <button type="button" on:click={dismissToday} disabled={saving}>{$t('今天稍后')}</button>
      </div>
      <button type="button" class="health-disable" on:click={disableDaily} disabled={saving}>{$t('关闭每日询问')}</button>
    </section>
  </div>
{/if}

<style>
  .health-backdrop { position: fixed; inset: 0; z-index: 60; display: grid; place-items: center; padding: 20px; background: #1c302266; }
  .health-modal { width: min(560px, 100%); max-height: calc(100dvh - 40px); overflow-y: auto; padding: 24px; border: 1px solid #dcded4; border-radius: 8px; background: #fbfcf8; box-shadow: 0 20px 50px #1c302233; }
  .health-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
  .health-head h2 { margin: 0; font-size: 22px; }
  .health-head p { margin: 6px 0 0; color: #6f7b6d; font-size: 13px; }
  .health-close { border: 0; background: none; color: #52695a; font-size: 26px; line-height: 1; padding: 0 4px; }
  .health-label { margin: 22px 0 9px; color: #52695a; font-size: 13px; font-weight: 600; }
  .mood-options { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 6px; }
  .mood-option { display: grid; gap: 4px; justify-items: center; padding: 10px 4px; border: 1px solid #dce3d6; background: #fff; color: #52695a; font-size: 12px; }
  .mood-option span { font-size: 19px; }
  .mood-option.active { border-color: #6f9661; background: #eef3e8; color: #274732; }
  .health-checks { display: grid; gap: 8px; margin: 18px 0; }
  .health-checks label { display: flex; align-items: center; gap: 8px; color: #354839; font-size: 14px; }
  .health-note { display: grid; gap: 6px; color: #52695a; font-size: 13px; }
  .health-note textarea { width: 100%; padding: 9px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; resize: vertical; }
  .health-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 18px; }
  .health-actions .primary { min-height: 44px; }
  .health-disable { margin-top: 14px; border: 0; background: none; color: #8b9587; font-size: 12px; text-decoration: underline; text-underline-offset: 3px; }
  .health-modal .message { margin: 12px 0 0; }
  @media (max-width: 520px) { .mood-options { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
