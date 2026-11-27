<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import VisibilityToggle from './VisibilityToggle.svelte';
  import RedactedText from './RedactedText.svelte';
  import { t } from './i18n.js';
  import { difficultyRank, levelRank } from './tiers.js';
  export let token;
  let profilePublic = false, hiddenSections = [], loaded = false, busy = false, error = '', notice = '';
  let center = null, cards = [], images = {}, activeTab = 'completed';
  let alive = true;
  const statusLabels = { busy: '忙碌', free: '空闲' };
  $: profile = center?.profile || {};
  $: identities = (center?.identities || []).filter(item => item.featured);
  $: featured = (center?.skills || []).filter(item => item.featured);
  $: skills = featured.length ? featured : (center?.skills || []);
  $: allProjects = [...(center?.projects?.initiated || []), ...(center?.projects?.participated || [])].filter(item => item.reviewStatus === 'approved');
  $: completed = allProjects.filter(item => item.status === 'completed');
  $: ongoing = allProjects.filter(item => item.status === 'ongoing');
  $: badges = center?.badges || [];
  async function request(url, options = {}) {
    const response = await fetch(url, { ...options, headers: { Authorization: `Bearer ${token}`, ...(options.headers || {}) }, cache: 'no-store' });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.message || `请求失败（${response.status}）`);
    return data;
  }
  async function loadImage(url) {
    if (!url || images[url]) return;
    const response = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok) return;
    const objectUrl = URL.createObjectURL(await response.blob());
    if (!alive) { URL.revokeObjectURL(objectUrl); return; }
    images = { ...images, [url]: objectUrl };
  }
  onMount(async () => {
    try {
      const [visibility, data, cardData] = await Promise.all([
        request('/api/hpahub/me/profile-visibility'), request('/api/hpahub/me/center'), request('/api/hpahub/me/cards'),
      ]);
      profilePublic = visibility.profilePublic;
      hiddenSections = visibility.hiddenSections || [];
      center = data;
      cards = cardData.items || [];
      await tick();
      const options = [['completed', completed.length], ['ongoing', ongoing.length], ['career', badges.length], ['skills', skills.length]];
      activeTab = options.find(([, count]) => count > 0)?.[0] || 'completed';
      loaded = true;
      const urls = [profile.avatarUrl, ...cards.flatMap(card => card.imageUrls || []), ...skills.flatMap(skill => skill.images || [])].filter(Boolean);
      await Promise.all([...new Set(urls)].map(loadImage));
    } catch (err) { error = err.message; }
  });
  onDestroy(() => { alive = false; Object.values(images).forEach(URL.revokeObjectURL); });
  function toggle(key) { hiddenSections = hiddenSections.includes(key) ? hiddenSections.filter(item => item !== key) : [...hiddenSections, key]; notice = ''; }
  async function save() {
    busy = true; error = ''; notice = '';
    try { await request('/api/hpahub/me/profile-visibility', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ profilePublic, hiddenSections }) }); notice = '资料已保存。'; }
    catch (err) { error = err.message; } finally { busy = false; }
  }
</script>

<main class="dashboard preview-page">
  <div class="page-head"><h1>{$t('修改个人名片')}</h1><a href="/me">{$t('编辑资料')}</a></div>
  {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
  {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}
  {#if loaded}
    <form on:submit|preventDefault={save}>
      <div class="settings-bar"><label><input type="checkbox" bind:checked={profilePublic} /> {$t('公开我的资料和卡片')}</label><button class="primary" disabled={busy}>{$t('保存')}</button></div>
      <div class="preview-surface">
        <div class="profile-heading">
          <div class="avatar-part" class:section-hidden={hiddenSections.includes('avatar')}><div class="avatar-wrap">{#if hiddenSections.includes('avatar')}<span class="avatar black-avatar" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{:else if images[profile.avatarUrl]}<img class="avatar" src={images[profile.avatarUrl]} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}</div><VisibilityToggle hidden={hiddenSections.includes('avatar')} label={'头像'} on:toggle={() => toggle('avatar')} /></div>
          <div class="profile-copy"><h2>{profile.name || profile.email}</h2><div class="inline-part" class:section-hidden={hiddenSections.includes('bio')}><p><RedactedText text={profile.bio || '—'} hidden={hiddenSections.includes('bio')} /></p><VisibilityToggle hidden={hiddenSections.includes('bio')} label={'简介'} on:toggle={() => toggle('bio')} /></div><div class="inline-part" class:section-hidden={hiddenSections.includes('status')}><span class="profile-status">{$t('当前状态')}：<RedactedText text={$t(statusLabels[profile.currentStatus] || '未设置')} hidden={hiddenSections.includes('status')} /></span><VisibilityToggle hidden={hiddenSections.includes('status')} label={'当前状态'} on:toggle={() => toggle('status')} /></div></div>
        </div>
        <section class="preview-section" class:section-hidden={hiddenSections.includes('identities')}><div class="section-top"><h3>{$t('职业')}</h3><VisibilityToggle hidden={hiddenSections.includes('identities')} label={'职业'} on:toggle={() => toggle('identities')} /></div><div class="identities">{#each identities as item}<div class="item-preview"><span class={`rank rank-pill rank-${item.category === 'main' ? 4 : 3}`}><RedactedText text={item.name} hidden={hiddenSections.includes('identities') || hiddenSections.includes('identity:' + item.id)} /></span><VisibilityToggle hidden={hiddenSections.includes('identity:' + item.id)} label={item.name} on:toggle={() => toggle('identity:' + item.id)} /></div>{:else}<span class="empty">—</span>{/each}</div></section>
        {#if center.relations}<div class="relation-panel"><span>{$t('被关注')} <strong>{center.relations.admireCount}</strong></span><span>{$t('被监视')} <strong>{center.relations.watchCount}</strong></span><span>{$t('被调查')} <strong>{center.relations.investigateCount}</strong></span></div>{/if}
        <section class="preview-section" class:section-hidden={hiddenSections.includes('cards')}><div class="section-top"><h3>{$t('我的卡片')}</h3><VisibilityToggle hidden={hiddenSections.includes('cards')} label={'我的卡片'} on:toggle={() => toggle('cards')} /></div><div class="public-grid">{#each cards as card}<article class="public-card"><div class="section-top"><h3><RedactedText text={card.title} hidden={hiddenSections.includes('cards') || hiddenSections.includes('card:' + card.id)} /></h3><VisibilityToggle hidden={hiddenSections.includes('card:' + card.id)} label={card.title} on:toggle={() => toggle('card:' + card.id)} /></div><p><RedactedText text={card.text} hidden={hiddenSections.includes('cards') || hiddenSections.includes('card:' + card.id)} /></p><div class="image-grid">{#each card.imageUrls || [] as url}{#if hiddenSections.includes('cards') || hiddenSections.includes('card:' + card.id)}<span class="image-mask" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{:else if images[url]}<img src={images[url]} alt={card.title} />{/if}{/each}</div></article>{:else}<p class="empty">{$t('暂无公开卡片。')}</p>{/each}</div></section>
        <nav class="profile-tabs" aria-label={$t('公开资料')}><button type="button" class:active={activeTab === 'completed'} on:click={() => activeTab = 'completed'}>{$t('完成项目')} <span>{completed.length}</span></button><button type="button" class:active={activeTab === 'ongoing'} on:click={() => activeTab = 'ongoing'}>{$t('进行中项目')} <span>{ongoing.length}</span></button><button type="button" class:active={activeTab === 'career'} on:click={() => activeTab = 'career'}>{$t('职业等级')} <span>{badges.length}</span></button><button type="button" class:active={activeTab === 'skills'} on:click={() => activeTab = 'skills'}>{$t('技能展示')} <span>{skills.length}</span></button></nav>
        {#if activeTab === 'completed' || activeTab === 'ongoing'}
          <section class="preview-section" class:section-hidden={hiddenSections.includes('projects')}><div class="section-top"><h3>{$t(activeTab === 'completed' ? '完成项目' : '进行中项目')}</h3><VisibilityToggle hidden={hiddenSections.includes('projects')} label={'项目'} on:toggle={() => toggle('projects')} /></div><div class="public-grid">{#each activeTab === 'completed' ? completed : ongoing as project}<article class="public-card">{#if !(hiddenSections.includes('projects') || hiddenSections.includes('project:' + project.id))}<div class="project-meta"><span class="project-status">{$t(activeTab === 'completed' ? '已完成' : '进行中')}</span>{#if project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{/if}</div>{/if}<div class="section-top"><h3><RedactedText text={project.title} hidden={hiddenSections.includes('projects') || hiddenSections.includes('project:' + project.id)} /></h3><VisibilityToggle hidden={hiddenSections.includes('project:' + project.id)} label={project.title} on:toggle={() => toggle('project:' + project.id)} /></div><p><RedactedText text={project.summary} hidden={hiddenSections.includes('projects') || hiddenSections.includes('project:' + project.id)} /></p></article>{:else}<p class="empty">{$t(activeTab === 'completed' ? '暂无完成项目。' : '暂无进行中项目。')}</p>{/each}</div></section>
        {:else if activeTab === 'career'}
          <section class="preview-section" class:section-hidden={hiddenSections.includes('career')}><div class="section-top"><h3>{$t('职业等级')}</h3><VisibilityToggle hidden={hiddenSections.includes('career')} label={'职业等级'} on:toggle={() => toggle('career')} /></div><div class="public-grid">{#each badges as badge}<article class="public-card"><div class="section-top"><h3><RedactedText text={badge.careerName} hidden={hiddenSections.includes('career') || hiddenSections.includes('career:' + badge.careerId)} /></h3><VisibilityToggle hidden={hiddenSections.includes('career:' + badge.careerId)} label={badge.careerName} on:toggle={() => toggle('career:' + badge.careerId)} /></div>{#if hiddenSections.includes('career') || hiddenSections.includes('career:' + badge.careerId)}<RedactedText text={badge.levelTitle || `Lv.${badge.level}`} hidden={true} />{:else}<span class={`rank rank-text rank-${levelRank(badge.level)}`}>{badge.levelTitle || `Lv.${badge.level}`}</span><p>{badge.totalXp} XP · {$t('最高等级')} Lv.{badge.maxLevel}</p>{/if}</article>{:else}<p class="empty">{$t('暂无职业等级。')}</p>{/each}</div></section>
        {:else}
          <section class="preview-section" class:section-hidden={hiddenSections.includes('skills')}><div class="section-top"><h3>{$t('技能展示')}</h3><VisibilityToggle hidden={hiddenSections.includes('skills')} label={'技能展示'} on:toggle={() => toggle('skills')} /></div><div class="public-grid">{#each skills as skill}<article class="public-card"><div class="section-top"><h3><RedactedText text={skill.name} hidden={hiddenSections.includes('skills') || hiddenSections.includes('skill:' + skill.userSkillId)} /></h3><VisibilityToggle hidden={hiddenSections.includes('skill:' + skill.userSkillId)} label={skill.name} on:toggle={() => toggle('skill:' + skill.userSkillId)} /></div><p><RedactedText text={skill.abilityNote} hidden={hiddenSections.includes('skills') || hiddenSections.includes('skill:' + skill.userSkillId)} /></p>{#if !(hiddenSections.includes('skills') || hiddenSections.includes('skill:' + skill.userSkillId))}{#if skill.proficiency}<small>{$t('熟练程度')}：{$t(skill.proficiency)}</small>{/if}<div class="image-grid">{#each skill.images || [] as url}{#if images[url]}<img src={images[url]} alt={skill.name} />{/if}{/each}</div>{#each skill.links || [] as link}<p class="skill-link">{link}</p>{/each}{:else}<p><RedactedText text={$t(skill.proficiency)} hidden={true} /></p>{#each skill.images || [] as url}<span class="image-mask" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{/each}{#each skill.links || [] as link}<p><RedactedText text={link} hidden={true} /></p>{/each}{/if}</article>{:else}<p class="empty">{$t('暂无技能。')}</p>{/each}</div></section>
        {/if}
      </div>
      <div class="bottom-actions"><button class="primary" disabled={busy}>{$t('保存')}</button></div>
    </form>
  {/if}
</main>
<style>
  .page-head { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:24px; }
  .page-head h1 { margin:0; } .page-head a { color:var(--accent-deep); }
  .settings-bar { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:18px; padding:14px 18px; border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface); }
  .settings-bar label, .visibility-toggle { display:inline-flex; align-items:center; gap:7px; }
  .preview-surface { padding:28px; border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-soft); }
  .profile-heading { display:flex; align-items:flex-start; gap:20px; margin-bottom:24px; }.avatar-part { display:grid; justify-items:center; gap:10px; flex:none; }.avatar { display:block; width:96px; height:96px; border-radius:50%; object-fit:cover; background:var(--surface-tint); border:1px solid var(--border-default); }.profile-copy { flex:1; min-width:0; }.profile-copy h2 { font-size:28px; margin:0 0 8px; }.inline-part { display:flex; align-items:center; flex-wrap:wrap; gap:10px 20px; margin:8px 0; }.inline-part p { margin:0; color:var(--ink-muted); overflow-wrap:anywhere; }.profile-status,.project-status { display:inline-block; flex:none; max-width:100%; padding:3px 9px; border:1px solid var(--border-subtle); border-radius:999px; color:var(--accent-deep); background:var(--surface-tint); font-size:12px; overflow-wrap:anywhere; }

  .black-avatar, .image-mask { display:inline-block; background:#000; } .image-mask { width:80px; height:80px; border-radius:var(--radius-sm); } .item-preview { display:inline-flex; align-items:center; flex-wrap:wrap; gap:6px; }
  .section-hidden { outline:1px dashed var(--ink-subtle); outline-offset:4px; }
  .preview-section { margin:20px 0; padding:18px; border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--surface); }.section-top { display:flex; justify-content:space-between; align-items:baseline; gap:12px; margin-bottom:12px; }.section-top h3 { margin:0; font-size:15px; }.identities { display:flex; flex-wrap:wrap; gap:8px; }.relation-panel { display:flex; flex-wrap:wrap; gap:20px; padding:14px 16px; border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--surface); color:var(--ink-muted); font-size:12px; }.relation-panel strong { color:var(--accent-deep); }
  .public-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,230px),1fr)); gap:14px; }.public-card { min-width:0; padding:16px; border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--surface-elevated); }.public-card h3 { margin:0; font-size:15px; }.public-card p { color:var(--ink-muted); font-size:13px; white-space:pre-wrap; overflow-wrap:anywhere; }.public-card small { color:var(--ink-subtle); font-size:12px; font-weight:400; }.image-grid { display:flex; flex-wrap:wrap; gap:8px; }.image-grid img { width:80px; height:80px; object-fit:cover; border-radius:var(--radius-sm); }.project-meta { display:flex; gap:8px; margin-bottom:12px; }.empty { margin:0; color:var(--ink-muted); font-size:13px; }.profile-tabs { display:flex; flex-wrap:wrap; gap:6px; margin-top:24px; padding-bottom:10px; border-bottom:1px solid var(--border-subtle); }.profile-tabs button { padding:8px 14px; border:1px solid transparent; border-radius:var(--radius-sm); background:none; color:var(--ink-secondary); cursor:pointer; }.profile-tabs button.active { background:var(--accent); color:#fff; }.profile-tabs span { margin-left:5px; font-size:12px; }.bottom-actions { margin-top:18px; }
  @media (max-width:600px) { .preview-surface { padding:16px; }.profile-heading { gap:14px; }.avatar { width:64px; height:64px; }.profile-copy h2 { font-size:22px; }.settings-bar { align-items:flex-start; flex-direction:column; }.section-top { flex-wrap:wrap; } }
</style>
