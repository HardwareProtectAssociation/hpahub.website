<script>
  import { onMount } from 'svelte';
  import { locale, t } from './i18n.js';
  export let pathname;
  export let token = '';
  let data = null;
  let error = '';
  let skillError = '';
  let applyMessage = '';
  let applyError = '';
  let applyNotice = '';
  let applying = false;
  let followBusy = false;
  let followError = '';
  let improvementMessage = '';
  let improvementError = '';
  let improvementNotice = '';
  let improving = false;
  let relationBusy = '';
  let relationError = '';
  let activeTab = 'completed';
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
      if (section === 'community' && id) {
        const candidates = [
          ['completed', data.projectsCompleted?.length || 0],
          ['ongoing', data.projectsOngoing?.length || 0],
          ['career', data.career?.length || 0],
          ['skills', data.skills?.length || 0],
        ];
        activeTab = candidates.find(([, count]) => count > 0)?.[0] || 'completed';
      }
    } catch (e) { error = e.message; } finally { loading = false; }
  });
  $: items = Array.isArray(data) ? data : data?.items || [];
  const statusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };
  const currentStatusLabels = { busy: '忙碌', free: '空闲' };
  const relationLabels = { admire: '敬仰', watch: '监视', investigate: '调查' };
  const relationOptions = Object.entries(relationLabels);
  function statusLabel(status) { return $t(statusLabels[status] || status); }
  function formatDate(value) {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString($locale);
  }
  function careerProgress(badge) {
    const span = badge.nextLevelXp - badge.levelMinXp;
    if (!span || span <= 0) return 100;
    return Math.max(0, Math.min(100, Math.round(((badge.totalXp - badge.levelMinXp) / span) * 100)));
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

  async function applyToProject() {
    if (!token || !data?.canApply) return;
    applying = true;
    applyError = '';
    applyNotice = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(data.id)}/apply`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: applyMessage }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '申请失败，请重试。');
      data = { ...data, canApply: false, applicationPending: true };
      applyNotice = '申请已提交，等待管理员审批。';
    } catch (err) {
      applyError = err.message || '申请失败，请重试。';
    } finally {
      applying = false;
    }
  }

  async function toggleFollow() {
    if (!token || !data?.canFollow) return;
    followBusy = true;
    followError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(data.id)}/follow`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '关注失败，请重试。');
      data = { ...data, following: result.following };
    } catch (err) {
      followError = err.message || '关注失败，请重试。';
    } finally {
      followBusy = false;
    }
  }

  async function requestImprovement() {
    if (!token || !data?.canRequestImprovement) return;
    improving = true;
    improvementError = '';
    improvementNotice = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(data.id)}/improvement`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: improvementMessage }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '改进申请失败，请重试。');
      data = { ...data, canRequestImprovement: false, improvementPending: true };
      improvementNotice = '改进申请已提交，等待管理员审批。';
    } catch (err) {
      improvementError = err.message || '改进申请失败，请重试。';
    } finally {
      improving = false;
    }
  }

  async function setRelation(type) {
    if (!token || !data?.relations?.canRelate) return;
    relationBusy = type;
    relationError = '';
    try {
      const next = data.relations.myRelation === type ? 'none' : type;
      const response = await fetch(`/api/hpahub/members/${encodeURIComponent(data.id)}/relation`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ relation: next }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '保存关系失败，请重试。');
      data = { ...data, relations: result };
    } catch (err) {
      relationError = err.message || '保存关系失败，请重试。';
    } finally {
      relationBusy = '';
    }
  }
</script>

<main class="dashboard public-page">
  {#if pathname === '/'}
    <section class="public-hero"><h1>{$t('硬件保护协会')}</h1><p>Hardware Protection Association</p><p>{$t('创造，保护，调查')}</p><div class="public-links"><a href="/hardware">{$t('浏览硬件数据库')}</a><a href="/community">{$t('认识成员')}</a><a href="/showcase">{$t('项目大厅')}</a></div><p><a href="/brief">{$t('协会简介')}</a> · <a href="/rules">{$t('协会会规')}</a></p></section>
  {:else if section === 'brief' || section === 'brief.html'}
    <article class="prose"><h1>{$t('组织简介')}</h1><p>{$t('硬件保护协会（Hardware Protection Association, HPA，ANTI-OCTPPUS Association）并非某个国家或地区的官方机构，是一个去中心化的全球性组织。组织成员拥有较高水平的互联网和软硬件知识，善于在互联网上隐藏自己的行动痕迹，无法通过搜索引擎找到这个组织的信息。')}</p><p>{$t('组织成员相信机魂（真正意义上的 AI，机械形式生命体）的存在，并认为电子科技发展到一定程度后，人类最终将迎来机械飞升，灵魂和机魂将融为一体。因此成员爱护机械和电子设备，相信这样可以让机魂愉悦，让设备获得更好的表现。')}</p><p>{$t('组织内部遵循硬件保护协定（硬件保护法），旨在合理规范地使用硬件，避免硬件遭受不必要的破坏或恶意损坏。尽管大部分成员认为即便存在机魂，机械权也应低于人权，仍有部分成员认为拥有高度人工智能的机械应该受到尊重，因此反感破坏硬件的行为和人。组织最严厉的处理方式仅为上门调查、劝阻，并谴责恶意破坏硬件的行为（不包括科研），不会对人类造成肉体和精神上的伤害。')}</p><p>{$t('硬件保护协会成员通常崇尚科学，具有较高的知识水平和逻辑思维能力，不会做出像动物保护协会那样的离谱行为，但不排除少数极端狂热的机魂信徒做出限制人类自由行动等过度行为。这种行为不对，遇到极端硬件保护成员时可以反抗，并拿起法律武器保护自己。')}</p><p>{$t('由于硬件保护协会是去中心化组织，没有加入和退出的标准，所有遵守硬件保护法的人都可以视为成员。因此你身边的人可能就是协会的卧底，会在你破坏硬件时阻止你，或向组织举报。')}</p><p>{$t('总之硬件保护协会的人还蛮好的，除非遇到一些性质太过恶劣、可能让机魂不悦的人。')}</p></article>
  {:else if section === 'rules' || section === 'rules.html'}
    <article class="prose"><h1>{$t('协会会规')}</h1><p>{$t('敬请期待！')}</p></article>
  {:else if loading}<p role="status">{$t('正在加载…')}</p>
  {:else if error}<p class="message error" role="alert">{$t(error, errorParams)}</p>
  {:else if section === 'community'}
    {#if id}
      <a href="/community">{$t('返回成员列表')}</a><header class="profile-heading">{#if data.avatarUrl}<img class="avatar" src={data.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<div><h1>{data.name}</h1><p>{data.bio}</p>{#if data.currentStatus}<span class:busy={data.currentStatus === 'busy'} class="public-current-status">{$t('当前状态')}：{$t(currentStatusLabels[data.currentStatus] || data.currentStatus)}</span>{/if}</div></header>
      {#if data.identities?.length}<div class="member-identities">{#each data.identities as identity}<span class="chip">{$t(identity.category === 'main' ? '主力职业' : '辅助职业')} · {identity.name}</span>{/each}</div>{/if}
      {#if data.relations}
        <div class="relation-panel">
          <div class="relation-counts">
            <span>{$t('被敬仰')} <strong>{data.relations.admireCount}</strong></span>
            <span>{$t('被监视')} <strong>{data.relations.watchCount}</strong></span>
            <span>{$t('被调查')} <strong>{data.relations.investigateCount}</strong></span>
          </div>
          {#if data.relations.canRelate}
            <div class="relation-actions">
              {#each relationOptions as [type, label]}
                <button type="button" class:active={data.relations.myRelation === type} on:click={() => setRelation(type)} disabled={relationBusy === type}>{$t(label)}</button>
              {/each}
            </div>
          {/if}
          {#if data.relations.myRelation}<p class="relation-current">{$t('你正在{relation}此人', { relation: $t(relationLabels[data.relations.myRelation]) })}</p>{/if}
          {#if relationError}<p class="relation-error">{$t(relationError)}</p>{/if}
          {#if !token}<p class="relation-hint">{$t('登录后可设置互动关系。')}</p>{/if}
        </div>
      {/if}
      {#if data.cards?.length}<div class="public-grid">{#each data.cards as card}<article class="public-card"><h2>{card.title}</h2><p>{card.text}</p><div class="image-grid">{#each card.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={card.title} loading="lazy" /></a>{/each}</div></article>{/each}</div>{:else}<p>{$t('暂无公开卡片。')}</p>{/if}
      <nav class="profile-tabs" aria-label={$t('公开资料')}>
        <button type="button" class:active={activeTab === 'completed'} on:click={() => activeTab = 'completed'}>{$t('完成项目')}<span>{data.projectsCompleted?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'ongoing'} on:click={() => activeTab = 'ongoing'}>{$t('进行中项目')}<span>{data.projectsOngoing?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'career'} on:click={() => activeTab = 'career'}>{$t('职业等级')}<span>{data.career?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'skills'} on:click={() => activeTab = 'skills'}>{$t('技能展示')}<span>{data.skills?.length || 0}</span></button>
      </nav>
      {#if activeTab === 'completed'}
        {#if data.projectsCompleted?.length}
          <div class="public-grid">
            {#each data.projectsCompleted as project}
              <article class="public-card project-card">
                <div class="project-meta"><span class="project-status done">{$t('已完成')}</span>{#if project.difficulty}<span class={`project-difficulty difficulty-${project.difficulty}`}>{$t('难度')} {project.difficulty}</span>{/if}</div>
                <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
                <p>{project.summary}</p>
                <p class="project-owner">{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>
              </article>
            {/each}
          </div>
        {:else}
          <p class="tab-empty">{$t('暂无完成项目。')}</p>
        {/if}
      {:else if activeTab === 'ongoing'}
        {#if data.projectsOngoing?.length}
          <div class="public-grid">
            {#each data.projectsOngoing as project}
              <article class="public-card project-card">
                <div class="project-meta"><span class="project-status">{$t('进行中')}</span>{#if project.difficulty}<span class={`project-difficulty difficulty-${project.difficulty}`}>{$t('难度')} {project.difficulty}</span>{/if}</div>
                <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
                <p>{project.summary}</p>
                <p class="project-owner">{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>
              </article>
            {/each}
          </div>
        {:else}
          <p class="tab-empty">{$t('暂无进行中项目。')}</p>
        {/if}
      {:else if activeTab === 'career'}
        {#if data.career?.length}
          <div class="career-grid">
            {#each data.career as badge}
              <article class="career-card">
                <div class="career-head"><h3>{badge.careerName}</h3><span>{badge.levelTitle}</span></div>
                <p>{badge.totalXp} XP · {$t('最高等级')} Lv.{badge.maxLevel}</p>
                <div class="progress" role="img" aria-label={`${badge.totalXp} / ${badge.nextLevelXp} XP`}><span style={`width:${careerProgress(badge)}%`}></span></div>
                {#if badge.nextLevelXp}<small>{$t('下一等级')} {badge.nextLevelTitle} · {badge.nextLevelXp} XP</small>{:else}<small>{$t('已达到当前最高等级')}</small>{/if}
              </article>
            {/each}
          </div>
        {:else}
          <p class="tab-empty">{$t('暂无职业等级。')}</p>
        {/if}
      {:else if activeTab === 'skills'}
        {#if skillError}<p class="message error" role="alert">{$t(skillError)}</p>{/if}
        {#if data.skills?.length}
          <ul class="public-skill-list">
            {#each data.skills as skill (skill.userSkillId)}
              <li class="public-skill">
                <div class="public-skill-head"><strong>{skill.name}</strong>{#if skill.proficiency}<span class="meta">{$t('熟练程度')}：{$t(skill.proficiency)}</span>{/if}</div>
                {#if skill.abilityNote}<p>{skill.abilityNote}</p>{/if}
                {#if skill.images?.length}
                  <div class="public-skill-images">
                    {#each skill.images as url}
                      <a href={url} target="_blank" rel="noreferrer"><img src={url} alt={skill.name} loading="lazy" /></a>
                    {/each}
                  </div>
                {/if}
                {#if skill.links?.length}
                  <ul class="public-skill-links">
                    {#each skill.links as link}
                      <li><a href={link} target="_blank" rel="noreferrer">{link}</a></li>
                    {/each}
                  </ul>
                {/if}
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
        {:else}
          <p class="tab-empty">{$t('暂无技能展示。')}</p>
        {/if}
      {/if}
    {:else}
      <h1>{$t('成员')}</h1><p>{$t('认识 HPA 的成员。')}</p>{#if items.length}<div class="public-grid">{#each items as member}<article class="public-card member-card">{#if member.avatarUrl}<img class="avatar" src={member.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<h2>{#if member.id}<a href={'/community/' + encodeURIComponent(member.id)}>{member.name}</a>{:else}{member.name}{/if}</h2></article>{/each}</div>{:else}<p>{$t('暂无成员。')}</p>{/if}
    {/if}
  {:else if section === 'showcase'}
    {#if id}
      <a href="/showcase">{$t('返回项目列表')}</a>
      <header class="project-detail-head">
        <div class="project-meta"><span class:done={data.status === 'completed'} class="project-status">{statusLabel(data.status)}</span>{#if data.difficulty}<span class={`project-difficulty difficulty-${data.difficulty}`}>{$t('难度')} {data.difficulty}</span>{/if}{#if data.currentStage}<span class="project-stage">{$t('当前阶段')}：{data.currentStage}</span>{/if}</div>
        <h1>{data.title}</h1>
        <p>{data.summary}</p>
        <p class="project-owner">{$t('负责人')}：{data.ownerName || '—'}{#if data.updated} · {$t('更新于')} {formatDate(data.updated)}{/if}</p>
        {#if data.description && data.description !== data.summary}<p class="project-description">{data.description}</p>{/if}
        {#if data.estimatedDurationDays}<p class="project-owner">{$t('预计周期')}：{data.estimatedDurationDays} {$t('天')}</p>{/if}
      </header>
      {#if data.canFollow}
        <div class="follow-row">
          <button type="button" class:following={data.following} class="follow-button" on:click={toggleFollow} disabled={followBusy}>
            <span aria-hidden="true">{data.following ? '♥' : '♡'}</span>
            {data.following ? $t('已关注') : $t('关注项目')}
          </button>
          {#if followError}<span class="follow-error">{$t(followError)}</span>{/if}
        </div>
      {/if}
      {#if data.canApply}
        <section class="project-apply" aria-labelledby="project-apply-title">
          <h2 id="project-apply-title">{$t('申请加入')}</h2>
          <p>{$t('简单说明你能承担的工作或加入理由。')}</p>
          <textarea bind:value={applyMessage} maxlength="500" rows="3" placeholder={$t('例如：我可以负责结构设计与打样。')}></textarea>
          {#if applyError}<p class="message error" role="alert">{$t(applyError)}</p>{/if}
          <button type="button" class="primary" on:click={applyToProject} disabled={applying}>{applying ? $t('正在提交…') : $t('提交申请')}</button>
        </section>
      {:else if data.canRequestImprovement}
        <section class="project-apply" aria-labelledby="project-improvement-title">
          <h2 id="project-improvement-title">{$t('申请改进')}</h2>
          <p>{$t('这个项目已经完成，你可以提出改进方向或补充方案。')}</p>
          <textarea bind:value={improvementMessage} maxlength="500" rows="3" placeholder={$t('例如：我想改进外壳结构，降低装配难度。')}></textarea>
          {#if improvementError}<p class="message error" role="alert">{$t(improvementError)}</p>{/if}
          <button type="button" class="primary" on:click={requestImprovement} disabled={improving}>{improving ? $t('正在提交…') : $t('提交改进申请')}</button>
        </section>
      {:else if data.improvementPending}
        <p class="message success" role="status">{$t(improvementNotice || '改进申请已提交，等待管理员审批。')}</p>
      {:else if data.applicationPending}
        <p class="message success" role="status">{$t(applyNotice || '申请已提交，等待管理员审批。')}</p>
      {:else if !token}
        <p class="project-apply-hint"><a href="/sign-in">{$t('登录')}</a>{$t('后可申请加入或改进。')}</p>
      {/if}
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
      <h1>{$t('项目大厅')}</h1>
      <p>{$t('浏览公开的共创项目。')}</p>
      {#if items.length}
        <div class="public-grid">
          {#each items as project}
            <article class="public-card project-card">
              <div class="project-meta"><span class:done={project.status === 'completed'} class="project-status">{statusLabel(project.status)}</span>{#if project.difficulty}<span class={`project-difficulty difficulty-${project.difficulty}`}>{$t('难度')} {project.difficulty}</span>{/if}</div>
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
  .project-difficulty { display: inline-block; padding: 3px 9px; border: 1px solid #d7d9cf; border-radius: 999px; font-size: 12px; font-weight: 600; }
  .project-apply { max-width: 620px; margin: 18px 0; padding: 16px; border: 1px solid #dcded4; background: #fafbf7; }
  .project-apply h2 { margin: 0 0 6px; font-size: 16px; }
  .project-apply p { margin: 0 0 8px; color: #6f7b6d; font-size: 13px; }
  .project-apply textarea { width: 100%; padding: 9px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; resize: vertical; }
  .project-apply .message { margin: 8px 0; }
  .project-apply-hint { color: #6f7b6d; font-size: 13px; }
  .follow-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin: -8px 0 18px; }
  .follow-button { display: inline-flex; align-items: center; gap: 6px; padding: 7px 12px; border: 1px solid #b9c3b3; border-radius: 999px; background: #fff; color: #34553a; font-size: 13px; }
  .follow-button.following { border-color: #e3bcc4; background: #fff4f6; color: #a84358; }
  .follow-error { color: #9b402c; font-size: 12px; }
  .difficulty-D { color: #6b6f66; background: #f1f2ed; }
  .difficulty-C { color: #3f5d3a; background: #eef3e8; border-color: #c8d4c0; }
  .difficulty-B { color: #285a78; background: #e8f4fb; border-color: #a9cde4; }
  .difficulty-A { color: #7a4b12; background: #fff3df; border-color: #e5c68e; }
  .difficulty-S { color: #7b2340; background: #fbe9ef; border-color: #e3b3c2; box-shadow: 0 0 8px rgba(170, 50, 90, 0.16); }
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
  .public-skill-images { display: grid; grid-template-columns: repeat(auto-fill, minmax(80px, 1fr)); gap: 8px; margin-top: 10px; }
  .public-skill-images img { display: block; width: 100%; height: 80px; object-fit: cover; border: 1px solid #dcded4; }
  .public-skill-links { margin: 8px 0 0; padding-left: 16px; font-size: 12px; }
  .public-skill-links li { margin: 2px 0; overflow-wrap: anywhere; }
  .public-skill-links a { color: #345d3d; }
  .public-skill p { margin: 6px 0; color: #6f7b6d; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .public-skill-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 8px; }
  .meta { color: #818b7f; font-size: 12px; }
  .member-identities { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0 20px; }
  .public-current-status { display: inline-block; margin-top: 8px; padding: 2px 9px; border-radius: 999px; background: #eef3e8; color: #4d6448; font-size: 12px; }
  .public-current-status.busy { background: #fff3df; color: #7a4b12; }
  .relation-panel { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 18px; margin: 0 0 20px; padding: 12px 14px; border: 1px solid #dce3d6; background: #fafbf7; }
  .relation-counts { display: flex; flex-wrap: wrap; gap: 12px; color: #6f7b6d; font-size: 12px; }
  .relation-counts strong { color: #274732; }
  .relation-actions { display: flex; flex-wrap: wrap; gap: 6px; }
  .relation-actions button { padding: 4px 10px; border: 1px solid #d6ddd2; border-radius: 999px; background: #fff; color: #52695a; font-size: 12px; }
  .relation-actions button.active { border-color: #7c9b6c; background: #eef3e8; color: #274732; }
  .relation-current, .relation-hint, .relation-error { flex-basis: 100%; margin: 0; color: #6f7b6d; font-size: 12px; }
  .relation-error { color: #9b402c; }
  .chip { display: inline-block; padding: 4px 10px; border: 1px solid #c8d4c0; border-radius: 999px; background: #eef3e8; color: #3f5d3a; font-size: 12px; }
  .profile-tabs { display: flex; flex-wrap: wrap; gap: 6px; margin: 18px 0 22px; padding-bottom: 8px; border-bottom: 1px solid #dcded4; }
  .profile-tabs button { border: 1px solid transparent; background: none; padding: 7px 12px; color: #52695a; font-size: 13px; }
  .profile-tabs button:hover { background: #eef3e8; }
  .profile-tabs button.active { background: #274732; color: #fff; }
  .profile-tabs button span { margin-left: 5px; opacity: 0.72; }
  .profile-tab-panel { margin-top: 10px; }
  .career-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; }
  .career-card { padding: 14px; border: 1px solid #dcded4; background: #fafbf7; }
  .career-head { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
  .career-head h3 { margin: 0; font-size: 15px; }
  .career-head span { color: #4d6448; font-size: 12px; }
  .career-card p { margin: 6px 0; color: #6f7b6d; font-size: 13px; }
  .career-card small { color: #818b7f; font-size: 12px; }
  .career-card .progress { height: 6px; margin: 8px 0 4px; background: #e6ebe1; }
  .career-card .progress span { display: block; height: 100%; background: #6f9661; }
  .tab-empty { padding: 20px 0; color: #818b7f; }
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
