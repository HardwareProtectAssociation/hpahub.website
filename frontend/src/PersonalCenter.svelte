<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;

  let data = null;
  let loading = true;
  let error = '';

  const rewardLabels = { money: '资金', product: '实物', title: '头衔', credit: '署名', learning: '学习', none: '无', other: '其他' };
  const statusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await fetch('/api/hpahub/me/center', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || `请求失败（${response.status}）`);
      data = body;
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  }

  function badgeProgress(badge) {
    const span = badge.nextLevelXp - badge.levelMinXp;
    if (!span || span <= 0) return 100;
    return Math.max(0, Math.min(100, Math.round(((badge.totalXp - badge.levelMinXp) / span) * 100)));
  }

  onMount(load);
</script>

<main class="dashboard personal-center">
  {#if loading}
    <p role="status">{$t('正在加载个人中心…')}</p>
  {:else if error}
    <div class="message error" role="alert">{$t(error)}</div>
    <button class="secondary" on:click={load}>{$t('重试')}</button>
  {:else if data}
    <div class="breadcrumb">{$t('个人中心')} <span>/</span> {$t('个人资料')}</div>

    <section class="profile-heading center-profile" aria-labelledby="center-name">
      {#if data.profile.avatarUrl}
        <img class="avatar" src={data.profile.avatarUrl} alt="" />
      {:else}
        <span class="avatar avatar-default" aria-hidden="true"></span>
      {/if}
      <div class="profile-copy">
        <h1 id="center-name">{data.profile.name || data.profile.email}</h1>
        {#if data.profile.name && data.profile.email && data.profile.email !== data.profile.name}
          <p class="center-email">{data.profile.email}</p>
        {/if}
        {#if data.profile.bio}<p class="center-bio">{data.profile.bio}</p>{/if}
        {#if data.identities.length}
          <div class="chips">
            {#each data.identities as identity}
              <span class="chip">{$t(identity.category === 'main' ? '主力职业' : '辅助职业')} · {identity.name}</span>
            {/each}
          </div>
        {/if}
        {#if data.profile.member}
          <div class="profile-actions">
            <a class="text-link" href="/settings">{$t('个人设置')}</a>
            {#if data.profile.profilePublic}
              <a class="text-link" href={`/community/${data.profile.id}`}>{$t('查看公开主页')}</a>
            {/if}
          </div>
        {/if}
      </div>
    </section>

    {#if !data.profile.member}
      <p class="message">{$t('当前为平台超级管理员账号，个人中心不包含会员资料；请使用会员账号查看完整信息。')}</p>
    {/if}

    <div class="stat-grid">
      <article class="stat"><span>{data.projects.participating.length}</span><p>{$t('参与的项目')}</p></article>
      <article class="stat"><span>{data.projects.completed.length}</span><p>{$t('自己完成的项目')}</p></article>
      <article class="stat"><span>{data.badges.length}</span><p>{$t('职业徽章')}</p></article>
      <article class="stat"><span>{data.skills.length}</span><p>{$t('技能展示')}</p></article>
    </div>

    <div class="content-grid center-grid">
      <section class="center-section" aria-labelledby="center-participating">
        <div class="section-head">
          <div><h2 id="center-participating">{$t('参与的项目')}</h2><p>{$t('你正在参与或发起的项目。')}</p></div>
        </div>
        {#if data.projects.participating.length}
          <ul class="center-list">
            {#each data.projects.participating as project (project.id)}
              <li>
                <div>
                  <strong>{project.title}</strong>
                  {#if project.summary}<p>{project.summary}</p>{/if}
                  <span class="meta">{$t(project.relation === 'owner' ? '发起人' : '协作成员')}{#if project.duties} · {project.duties}{/if}</span>
                </div>
                <span class:done={project.status === 'completed'} class="status-pill">{$t(statusLabels[project.status] || project.status)}</span>
              </li>
            {/each}
          </ul>
        {:else}
          <div class="center-empty"><p>{$t('还没有参与项目')}</p></div>
        {/if}
      </section>

      <section class="center-section" aria-labelledby="center-completed">
        <div class="section-head">
          <div><h2 id="center-completed">{$t('自己完成的项目')}</h2><p>{$t('由你发起并已完成的项目。')}</p></div>
        </div>
        {#if data.projects.completed.length}
          <ul class="center-list">
            {#each data.projects.completed as project (project.id)}
              <li>
                <div>
                  <strong>{project.title}</strong>
                  {#if project.summary}<p>{project.summary}</p>{/if}
                  <span class="meta">{$t('发起人')} · {$t('已完成')}</span>
                </div>
                <span class="status-pill done">{$t('已完成')}</span>
              </li>
            {/each}
          </ul>
        {:else}
          <div class="center-empty"><p>{$t('还没有完成项目')}</p></div>
        {/if}
      </section>

      <section class="center-section" aria-labelledby="center-badges">
        <div class="section-head">
          <div><h2 id="center-badges">{$t('职业徽章')}</h2><p>{$t('按实际职业贡献累计的经验与等级。')}</p></div>
        </div>
        {#if data.badges.length}
          <ul class="badge-list">
            {#each data.badges as badge (badge.careerId)}
              <li class="badge-card">
                <div class="badge-icon" aria-hidden="true">✦</div>
                <div class="badge-body">
                  <strong>{badge.careerName}</strong>
                  <p>{badge.levelTitle || `Lv.${badge.level}`} · {$t('累计经验')} {badge.totalXp}</p>
                  {#if badge.nextLevelXp}
                    <div class="progress" role="img" aria-label={`${badge.totalXp} / ${badge.nextLevelXp} XP`}><span style={`width:${badgeProgress(badge)}%`}></span></div>
                    <small>{badge.totalXp} / {badge.nextLevelXp} XP · {$t('下一等级')} {badge.nextLevelTitle}</small>
                  {:else}
                    <small>{$t('已达到当前最高等级')}</small>
                  {/if}
                </div>
              </li>
            {/each}
          </ul>
        {:else}
          <div class="center-empty"><p>{$t('还没有职业徽章')}</p></div>
        {/if}
      </section>

      <section class="center-section" aria-labelledby="center-skills">
        <div class="section-head">
          <div><h2 id="center-skills">{$t('技能展示')}</h2><p>{$t('技能为本人自述，作品与贡献作为依据。')}</p></div>
        </div>
        {#if data.skills.length}
          <ul class="skill-list">
            {#each data.skills as skill (skill.id)}
              <li class="skill-card">
                <div class="skill-head"><strong>{skill.name}</strong>{#if skill.category}<span class="chip small">{skill.category}</span>{/if}</div>
                {#if skill.proficiency}<span class="meta">{$t('熟练程度')}：{skill.proficiency}</span>{/if}
                {#if skill.abilityNote}<p>{skill.abilityNote}</p>{/if}
              </li>
            {/each}
          </ul>
        {:else}
          <div class="center-empty"><p>{$t('还没有添加技能')}</p></div>
        {/if}
      </section>

      <section class="center-section wide" aria-labelledby="center-rewards">
        <div class="section-head">
          <div><h2 id="center-rewards">{$t('希望得到什么')}</h2><p>{$t('你在合作中期望获得的回报。')}</p></div>
        </div>
        {#if data.preferences && (data.preferences.expectedRewardTypes.length || data.preferences.expectedRewardNote)}
          {#if data.preferences.expectedRewardTypes.length}
            <div class="chips">
              {#each data.preferences.expectedRewardTypes as type}
                <span class="chip">{$t(rewardLabels[type] || type)}</span>
              {/each}
            </div>
          {/if}
          {#if data.preferences.expectedRewardNote}<p class="reward-note">{data.preferences.expectedRewardNote}</p>{/if}
        {:else}
          <div class="center-empty">
            <p>{$t('还没有填写期望回报')}</p>
            {#if data.profile.member}<a href="/settings">{$t('去个人设置填写')}</a>{/if}
          </div>
        {/if}
      </section>
    </div>
  {/if}
</main>

<style>
  .personal-center { width: min(1200px, 88%); }
  .center-profile { align-items: flex-start; border-bottom: 1px solid #dcded4; padding-bottom: 28px; }
  .center-profile h1 { margin: 0 0 6px; font-size: clamp(28px, 3vw, 40px); line-height: 1.2; letter-spacing: -0.035em; font-weight: 600; }
  .center-email { margin: 0 0 12px; color: #818b7f; font-size: 14px; }
  .center-bio { margin: 0 0 14px; max-width: 70ch; white-space: pre-wrap; overflow-wrap: anywhere; }
  .chips { display: flex; flex-wrap: wrap; gap: 8px; }
  .chip { display: inline-block; padding: 4px 10px; border: 1px solid #c8d4c0; border-radius: 999px; background: #eef3e8; color: #3f5d3a; font-size: 12px; }
  .chip.small { padding: 2px 8px; font-size: 11px; }
  .profile-actions { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 16px; font-size: 13px; }
  .text-link { color: #345d3d; text-underline-offset: 3px; }
  .stat-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin: 26px 0; }
  .stat { border: 1px solid #dcded4; background: #fafbf7; padding: 18px 20px; }
  .stat span { font-size: 30px; font-weight: 600; letter-spacing: -0.03em; }
  .stat p { margin: 4px 0 0; color: #818b7f; font-size: 13px; }
  .center-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; }
  .center-section { min-width: 0; padding: 24px; border: 1px solid #dcded4; background: #fafbf7; }
  .center-section .section-head { margin-bottom: 16px; }
  .center-section h2 { margin: 0; font-size: 18px; }
  .center-list, .badge-list, .skill-list { display: grid; gap: 12px; margin: 0; padding: 0; list-style: none; }
  .center-list > li { display: flex; justify-content: space-between; gap: 16px; padding-top: 12px; border-top: 1px solid #e6e9df; }
  .center-list > li:first-child { padding-top: 0; border-top: 0; }
  .center-list p { margin: 4px 0; color: #6f7b6d; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .meta { color: #818b7f; font-size: 12px; }
  .status-pill { flex: none; height: fit-content; padding: 3px 9px; border: 1px solid #c8d4c0; border-radius: 999px; color: #3f5d3a; font-size: 12px; white-space: nowrap; }
  .status-pill.done { background: #e3eddd; }
  .badge-card { display: flex; gap: 14px; padding-top: 12px; border-top: 1px solid #e6e9df; }
  .badge-card:first-child { padding-top: 0; border-top: 0; }
  .badge-icon { display: grid; flex: none; width: 40px; height: 40px; place-items: center; border-radius: 50%; background: #e3eddd; color: #3f5d3a; }
  .badge-body { flex: 1; min-width: 0; }
  .badge-body p { margin: 2px 0 6px; color: #6f7b6d; font-size: 13px; }
  .badge-body small { color: #818b7f; font-size: 12px; }
  .progress { height: 6px; margin: 8px 0 4px; background: #e6e9df; }
  .progress span { display: block; height: 100%; background: #6f9661; }
  .skill-card { padding-top: 12px; border-top: 1px solid #e6e9df; }
  .skill-card:first-child { padding-top: 0; border-top: 0; }
  .skill-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
  .skill-card p { margin: 6px 0 0; color: #6f7b6d; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .reward-note { margin: 12px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
  .center-empty { color: #818b7f; font-size: 13px; }
  .center-empty p { margin: 0 0 6px; }
  .center-empty a { color: #345d3d; }
  .wide { grid-column: 1 / -1; }
  @media (max-width: 900px) {
    .stat-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
  @media (max-width: 800px) {
    .center-grid { grid-template-columns: 1fr; }
    .center-profile { flex-direction: column; gap: 18px; }
  }
</style>
