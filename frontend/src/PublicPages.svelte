<script>
  import { onMount } from 'svelte';
  import { locale, t } from './i18n.js';
  export let pathname;
  export let token = '';
  let data = null;
  let error = '';
  let skillError = '';
  let loading = true;
  let errorParams = {};
  const parts = pathname.split('/').filter(Boolean);
  const section = parts[0];
  const id = parts[1];
  onMount(async () => {
    if (!['community', 'showcase'].includes(section)) { loading = false; return; }
    try {
      const endpoint = section === 'community' ? 'members' : 'projects';
      const response = await fetch(`/api/hpahub/${endpoint}${id ? '/' + encodeURIComponent(id) : ''}`, {
        cache: 'no-store',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) { error = response.status === 404 ? '内容不存在或未公开。' : '加载失败（{status}）'; errorParams = { status: response.status }; return; }
      data = await response.json();
    } catch (e) { error = e.message; } finally { loading = false; }
  });
  $: items = Array.isArray(data) ? data : data?.items || [];
  const statusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };
  function statusLabel(status) { return $t(statusLabels[status] || status); }
  function formatDate(value) {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString($locale);
  }
  async function endorseSkill(skill) {
    if (!token) return;
    skillError = '';
    try {
      const response = await fetch(`/api/hpahub/skills/${encodeURIComponent(skill.userSkillId)}/like`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error('点赞失败，请重试。');
      data = {
        ...data,
        skills: data.skills.map((item) => item.userSkillId === skill.userSkillId
          ? { ...item, likedByMe: result.liked, likeCount: result.likeCount, likeTier: result.likeTier }
          : item),
      };
    } catch (err) {
      skillError = err.message || '点赞失败，请重试。';
    }
  }
</script>

<main class="dashboard public-page">
  {#if pathname === '/'}
    <section class="public-hero"><h1>{$t('硬件保护协会')}</h1><p>Hardware Protection Association</p><p>{$t('创造，保护，调查')}</p><div class="public-links"><a href="/hardware">{$t('浏览硬件数据库')}</a><a href="/community">{$t('认识成员')}</a><a href="/showcase">{$t('项目展示')}</a></div><p><a href="/brief">{$t('协会简介')}</a> · <a href="/rules">{$t('协会会规')}</a></p></section>
  {:else if section === 'brief' || section === 'brief.html'}
    <article class="prose"><h1>{$t('组织简介')}</h1><p>{$t('硬件保护协会（Hardware Protection Association, HPA，ANTI-OCTPPUS Association）并非某个国家或地区的官方机构，是一个去中心化的全球性组织。组织成员拥有较高水平的互联网和软硬件知识，善于在互联网上隐藏自己的行动痕迹，无法通过搜索引擎找到这个组织的信息。')}</p><p>{$t('组织成员相信机魂（真正意义上的 AI，机械形式生命体）的存在，并认为电子科技发展到一定程度后，人类最终将迎来机械飞升，灵魂和机魂将融为一体。因此成员爱护机械和电子设备，相信这样可以让机魂愉悦，让设备获得更好的表现。')}</p><p>{$t('组织内部遵循硬件保护协定（硬件保护法），旨在合理规范地使用硬件，避免硬件遭受不必要的破坏或恶意损坏。尽管大部分成员认为即便存在机魂，机械权也应低于人权，仍有部分成员认为拥有高度人工智能的机械应该受到尊重，因此反感破坏硬件的行为和人。组织最严厉的处理方式仅为上门调查、劝阻，并谴责恶意破坏硬件的行为（不包括科研），不会对人类造成肉体和精神上的伤害。')}</p><p>{$t('硬件保护协会成员通常崇尚科学，具有较高的知识水平和逻辑思维能力，不会做出像动物保护协会那样的离谱行为，但不排除少数极端狂热的机魂信徒做出限制人类自由行动等过度行为。这种行为不对，遇到极端硬件保护成员时可以反抗，并拿起法律武器保护自己。')}</p><p>{$t('由于硬件保护协会是去中心化组织，没有加入和退出的标准，所有遵守硬件保护法的人都可以视为成员。因此你身边的人可能就是协会的卧底，会在你破坏硬件时阻止你，或向组织举报。')}</p><p>{$t('总之硬件保护协会的人还蛮好的，除非遇到一些性质太过恶劣、可能让机魂不悦的人。')}</p></article>
  {:else if section === 'rules' || section === 'rules.html'}
    <article class="prose"><h1>{$t('协会会规')}</h1><p>{$t('敬请期待！')}</p></article>
  {:else if loading}<p role="status">{$t('正在加载…')}</p>
  {:else if error}<p class="message error" role="alert">{$t(error, errorParams)}</p>
  {:else if section === 'community'}
    {#if id}
      <a href="/community">{$t('返回成员列表')}</a><header class="profile-heading">{#if data.avatarUrl}<img class="avatar" src={data.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<div><h1>{data.name}</h1><p>{data.bio}</p></div></header>
      {#if data.cards?.length}<div class="public-grid">{#each data.cards as card}<article class="public-card"><h2>{card.title}</h2><p>{card.text}</p><div class="image-grid">{#each card.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={card.title} loading="lazy" /></a>{/each}</div></article>{/each}</div>{:else}<p>{$t('暂无公开卡片。')}</p>{/if}
      {#if data.skills?.length}
        <section class="member-skills" aria-labelledby="member-skills-title">
          <h2 id="member-skills-title">{$t('技能展示')}</h2>
          {#if skillError}<p class="message error" role="alert">{$t(skillError)}</p>{/if}
          <ul class="public-skill-list">
            {#each data.skills as skill (skill.userSkillId)}
              <li class="public-skill">
                <div class="public-skill-head"><strong>{skill.name}</strong>{#if skill.proficiency}<span class="meta">{$t('熟练程度')}：{$t(skill.proficiency)}</span>{/if}</div>
                {#if skill.abilityNote}<p>{skill.abilityNote}</p>{/if}
                <div class="public-skill-foot">
                  {#if skill.canEndorse}
                    <button type="button" class:liked={skill.likedByMe} class="like-button" on:click={() => endorseSkill(skill)} aria-pressed={skill.likedByMe} aria-label={$t(skill.likedByMe ? '取消点赞' : '点赞')}>
                      <span aria-hidden="true">{skill.likedByMe ? '♥' : '♡'}</span> {skill.likeCount}
                    </button>
                  {:else}
                    <span class="like-count"><span aria-hidden="true">♥</span> {skill.likeCount}</span>
                  {/if}
                  {#if skill.likeTier > 0}<span class={`recognized tier-${skill.likeTier}`}>{$t('受到认可')}</span>{/if}
                  {#if !token}<span class="meta">{$t('登录后可点赞')}</span>{/if}
                </div>
              </li>
            {/each}
          </ul>
        </section>
      {/if}
    {:else}
      <h1>{$t('成员')}</h1><p>{$t('认识 HPA 的成员。')}</p>{#if items.length}<div class="public-grid">{#each items as member}<article class="public-card member-card">{#if member.avatarUrl}<img class="avatar" src={member.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<h2>{#if member.id}<a href={'/community/' + encodeURIComponent(member.id)}>{member.name}</a>{:else}{member.name}{/if}</h2></article>{/each}</div>{:else}<p>{$t('暂无成员。')}</p>{/if}
    {/if}
  {:else if section === 'showcase'}
    {#if id}
      <a href="/showcase">{$t('返回项目列表')}</a>
      <header class="project-detail-head">
        <div class="project-meta"><span class:done={data.status === 'completed'} class="project-status">{statusLabel(data.status)}</span>{#if data.currentStage}<span class="project-stage">{$t('当前阶段')}：{data.currentStage}</span>{/if}</div>
        <h1>{data.title}</h1>
        <p>{data.summary}</p>
        <p class="project-owner">{$t('负责人')}：{data.ownerName || '—'}{#if data.updated} · {$t('更新于')} {formatDate(data.updated)}{/if}</p>
        {#if data.description && data.description !== data.summary}<p class="project-description">{data.description}</p>{/if}
        {#if data.estimatedDurationDays}<p class="project-owner">{$t('预计周期')}：{data.estimatedDurationDays} {$t('天')}</p>{/if}
      </header>
      {#if data.stages?.length}
        <section class="project-section">
          <h2>{$t('项目阶段')}</h2>
          <ol class="project-stages">
            {#each data.stages as stage}
              <li class:current={stage.current}>
                <strong>{stage.name}</strong>
                {#if stage.description}<p>{stage.description}</p>{/if}
                <span class="project-stage-state">{stage.completedAt ? $t('已完成') : stage.current ? $t('当前阶段') : $t('未开始')}</span>
              </li>
            {/each}
          </ol>
        </section>
      {/if}
      <section class="project-section">
        <h2>{$t('协作成员')}</h2>
        {#if data.members?.length}
          <ul class="project-members">
            {#each data.members as member}
              <li><strong>{member.name || '—'}</strong>{#if member.duties}<span>{member.duties}</span>{/if}</li>
            {/each}
          </ul>
        {:else}
          <p>{$t('暂无协作成员。')}</p>
        {/if}
      </section>
    {:else}
      <h1>{$t('项目展示')}</h1>
      <p>{$t('浏览公开的共创项目。')}</p>
      {#if items.length}
        <div class="public-grid">
          {#each items as project}
            <article class="public-card project-card">
              <div class="project-meta"><span class:done={project.status === 'completed'} class="project-status">{statusLabel(project.status)}</span></div>
              <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
              <p>{project.summary}</p>
              <p class="project-owner">{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>
            </article>
          {/each}
        </div>
      {:else}<p>{$t('暂无公开项目。')}</p>{/if}
    {/if}
  {:else}<h1>{$t('页面不存在')}</h1><a href="/">{$t('返回首页')}</a>{/if}
</main>

<style>
  .project-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 10px; }
  .project-status { display: inline-block; padding: 3px 9px; border: 1px solid #c8d4c0; border-radius: 999px; color: #3f5d3a; font-size: 12px; }
  .project-status.done { background: #e3eddd; }
  .project-stage { color: #818b7f; font-size: 12px; }
  .project-owner { margin: 0 0 10px; color: #818b7f; font-size: 13px; }
  .project-card p { margin: 0 0 10px; }
  .project-card p.project-owner { margin-bottom: 0; }
  .project-description { margin-top: 16px; max-width: 70ch; white-space: pre-wrap; }
  .project-detail-head { margin: 24px 0 32px; }
  .project-detail-head h1 { margin: 8px 0 12px; }
  .project-section { margin: 32px 0; }
  .project-section h2 { margin-bottom: 12px; }
  .project-stages { display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
  .project-stages li { padding: 4px 0 4px 14px; border-left: 2px solid #dcded4; }
  .project-stages li.current { border-color: #6f9661; }
  .project-stages p { margin: 4px 0; color: #6f7b6d; font-size: 13px; }
  .project-stage-state { color: #818b7f; font-size: 12px; }
  .project-members { display: grid; gap: 8px; margin: 0; padding: 0; list-style: none; }
  .project-members li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; }
  .project-members span { color: #818b7f; font-size: 13px; }
  .member-skills { margin: 32px 0; }
  .member-skills h2 { margin-bottom: 14px; }
  .public-skill-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr)); gap: 16px; margin: 0; padding: 0; list-style: none; }
  .public-skill { padding: 16px; border: 1px solid #dcded4; background: #fafbf7; }
  .public-skill-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; }
  .public-skill p { margin: 6px 0; color: #6f7b6d; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .public-skill-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 8px; }
  .meta { color: #818b7f; font-size: 12px; }
  .like-button { display: inline-flex; align-items: center; gap: 4px; padding: 2px 9px; border: 1px solid #d6ddd2; border-radius: 999px; background: #fff; color: #52695a; font-size: 12px; cursor: pointer; }
  .like-button.liked { color: #a84358; border-color: #e3bcc4; background: #fff4f6; }
  .like-count { color: #a84358; font-size: 12px; }
  .recognized { display: inline-flex; align-items: center; gap: 4px; padding: 2px 9px; border: 1px solid transparent; border-radius: 999px; font-size: 11px; font-weight: 600; letter-spacing: 0.02em; }
  .recognized::before { content: '✦'; }
  .tier-1 { color: #4d6b3f; background: #e8f0e0; border-color: #c4d8b4; }
  .tier-2 { color: #285a78; background: linear-gradient(135deg, #e8f4fb, #d6e9f7); border-color: #a9cde4; }
  .tier-3 { color: #7a5a12; background: linear-gradient(135deg, #fff7d6, #f2d98b); border-color: #e2c25e; box-shadow: 0 0 0 1px #f6e7b0 inset, 0 2px 6px rgba(180, 140, 20, 0.18); }
  .tier-4 { color: #fff; background: linear-gradient(120deg, #7c4dff, #d04dff, #ffb84d, #7c4dff); background-size: 300% 300%; border-color: #e0c7ff; box-shadow: 0 0 12px rgba(160, 80, 255, 0.45); animation: recognized-shine 6s linear infinite; }
  @keyframes recognized-shine { to { background-position: 300% 50%; } }
  @media (prefers-reduced-motion: reduce) { .tier-4 { animation: none; } }
</style>
