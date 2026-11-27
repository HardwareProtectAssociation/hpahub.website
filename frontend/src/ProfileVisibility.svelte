<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  export let token;
  const sections = [ ['avatar','显示头像'], ['bio','显示简介'], ['status','显示当前状态'], ['cards','显示展示卡片'], ['identities','显示职业'], ['projects','显示项目'], ['career','显示职业等级'], ['skills','显示技能'] ];
  let profilePublic = false, hiddenSections = [], loaded = false, busy = false, error = '', notice = '';
  async function request(options = {}) {
    const response = await fetch('/api/hpahub/me/profile-visibility', { ...options, headers: { Authorization: `Bearer ${token}`, ...(options.headers || {}) }, cache: 'no-store' });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.message || `请求失败（${response.status}）`);
    return data;
  }
  onMount(async () => { try { const data = await request(); profilePublic = data.profilePublic; hiddenSections = data.hiddenSections || []; loaded = true; } catch (err) { error = err.message; } });
  function toggle(key) { hiddenSections = hiddenSections.includes(key) ? hiddenSections.filter(item => item !== key) : [...hiddenSections, key]; }
  async function save() {
    busy = true; error = ''; notice = '';
    try { await request({ method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ profilePublic, hiddenSections }) }); notice = '资料已保存。'; }
    catch (err) { error = err.message; } finally { busy = false; }
  }
</script>
<main class="dashboard">
  <h1>{$t('修改个人名片')}</h1>
  {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
  {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}
  {#if loaded}
    <form class="visibility-form" on:submit|preventDefault={save}>
      <label><input type="checkbox" bind:checked={profilePublic} /> {$t('公开我的资料和卡片')}</label>
      {#each sections as [key, label]}
        <label><input type="checkbox" checked={!hiddenSections.includes(key)} on:change={() => toggle(key)} /> {$t(label)}</label>
      {/each}
      <button class="primary" disabled={busy}>{$t('保存')}</button>
      <a href="/me">{$t('编辑资料')}</a>
    </form>
  {/if}
</main>
<style>
  .visibility-form { display: grid; gap: 14px; max-width: 480px; padding: 24px; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--surface); }
  .visibility-form label { display: flex; align-items: center; gap: 10px; }
  .visibility-form button { justify-self: start; }
  .visibility-form a { color: var(--accent-deep); }
</style>
