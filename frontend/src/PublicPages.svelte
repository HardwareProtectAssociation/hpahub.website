<script>
  import { onMount } from 'svelte';
  import { locale, t } from './i18n.js';
  import { difficultyRank, levelRank } from './tiers.js';
  export let pathname;
  export let token = '';
  let data = null;
  let home = null;
  let error = '';
  let skillError = '';
  let applyMessage = '';
  let applyProfession = '';
  let applyTime = '';
  let applyAbility = '';
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
    if (pathname === '/') {
      try {
        const response = await fetch('/api/hpahub/home', { cache: 'no-store', headers: token ? { Authorization: `Bearer ${token}` } : {} });
        if (response.ok) home = await response.json();
      } catch { /* 热门栏非关键路径，失败时仅展示 hero */ }
      loading = false;
      return;
    }
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
        const visible = candidates.filter(([key]) => !(data.hiddenSections || []).includes(key === 'completed' || key === 'ongoing' ? 'projects' : key));
        activeTab = visible.find(([, count]) => count > 0)?.[0] || visible[0]?.[0] || '';
      }
    } catch (e) { error = e.message; } finally { loading = false; }
  });
  $: items = Array.isArray(data) ? data : data?.items || [];
  // shortcut: 演示卡片在 /api/hpahub/home 无数据时展示版式，真实内容稳定后删除
  const demoProjects = [
    { id: '', title: '智能家居中控系统', summary: '开发一套基于树莓派的智能家居中控系统，包含硬件设计、固件和前端界面。', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 12, updated: '', difficulty: 'S', adventureStyle: 'project' },
    { id: '', title: '开源键盘固件', summary: '可配置的分体键盘固件，支持多层键位、宏、OLED 状态显示和无线切换。', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 8, updated: '', difficulty: 'A', adventureStyle: 'dungeon' },
    { id: '', title: '桌面灯带控制器', summary: '为桌面工作区设计一套可调色温的灯带控制器，包含固件、外壳和简单上位机。', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 5, updated: '', difficulty: 'B', adventureStyle: 'boss' },
    { id: '', title: '桌面电源管理器', summary: '为桌面设备设计一个集中供电和理线的小型电源管理器。', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 3, updated: '', difficulty: 'C', adventureStyle: 'project' },
  ];
  const demoEngineers = [
    { id: '', name: '示例工程师 A', avatarUrl: '', identities: [{ name: '电子工程师' }], admireCount: 24 },
    { id: '', name: '示例工程师 B', avatarUrl: '', identities: [{ name: '机械设计师' }], admireCount: 12 },
    { id: '', name: '示例工程师 C', avatarUrl: '', identities: [{ name: '固件工程师' }], admireCount: 6 },
  ];
  const demoHardware = [
    { id: '', name: 'GeForce RTX 4080 SUPER', brand: 'NVIDIA', subtitle: '16GB GDDR6X', likeCount: 32 },
    { id: '', name: 'Ryzen 7 9800X3D', brand: 'AMD', subtitle: '8 核 16 线程', likeCount: 18 },
    { id: '', name: 'HHKB Professional', brand: 'PFU', subtitle: '静电容键盘', likeCount: 9 },
  ];
  $: shownProjects = home?.popularProjects?.length ? home.popularProjects : demoProjects;
  $: shownEngineers = home?.popularEngineers?.length ? home.popularEngineers : demoEngineers;
  $: shownHardware = home?.popularHardware?.length ? home.popularHardware : demoHardware;
  $: featuredSkills = (data?.skills || []).filter((skill) => skill.featured);
  $: shownSkills = featuredSkills.length ? featuredSkills : (data?.skills || []);
  const statusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };
  const currentStatusLabels = { busy: '忙碌', free: '空闲' };
  const relationLabels = { admire: '关注', watch: '监视', investigate: '调查' };
  const recognizedLabels = { 1: '有点东西', 2: '他不对劲', 3: '他真会啊', 4: '他太强了！', 5: '他是超人！' };
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
  function medalLevel(level) {
    return Math.floor((level || 0) / 10) * 10;
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
        body: JSON.stringify({
          message: applyMessage,
          profession: applyProfession,
          time: applyTime,
          ability: applyAbility,
        }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '申请失败，请重试。');
      data = { ...data, canApply: false, applicationPending: !result.autoAccepted, alreadyMember: result.autoAccepted };
      applyNotice = result.autoAccepted ? '已自动加入项目！' : '申请已提交，等待管理员审批。';
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
    <section class="public-hero">
      <div class="hero-orb" aria-hidden="true"><span></span><span></span></div>
      <div class="hero-badge" aria-hidden="true"><i></i> HPA · TELEMETRY &amp; LAB</div>
      <h1>{$t('硬件保护协会')}</h1>
      <p class="hero-en">Hardware Protection Association</p>
      <p class="hero-motto">{$t('创造，保护，调查')}</p>
      <p class="hero-actions"><a href="/brief">{$t('协会简介')}</a><a href="/rules">{$t('协会会规')}</a></p>
    </section>
    <section class="home-shelf">
      <div class="shelf-head"><h2>{$t('热门项目')}</h2><a href="/showcase">{$t('项目大厅')} →</a></div>
      <div class="home-grid">
        {#each shownProjects as project (project.id || project.title)}
          <a class="yt-card" href={project.id ? '/showcase/' + encodeURIComponent(project.id) : '/showcase'}>
            <span class={`yt-thumb yt-${project.adventureStyle}`}>{#if project.difficulty}<span class={`yt-badge rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{:else}<span class="yt-badge rank-pending">{$t('难度')} {$t('待定')}</span>{/if}</span>
            <div class="yt-body">
              {#if project.ownerAvatarUrl}<img class="yt-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="yt-avatar" aria-hidden="true"></span>{/if}
              <div>
                <h3>{project.title}</h3>
                <p class="yt-meta">{project.ownerName || '—'}</p>
                <p class="yt-meta">{$t('关注')} {project.followCount}{#if project.updated} · {formatDate(project.updated)}{/if}</p>
              </div>
            </div>
          </a>
        {/each}
      </div>
    </section>
    <section class="home-shelf">
      <div class="shelf-head"><h2>{$t('热门工程师')}</h2><a href="/community">{$t('成员')} →</a></div>
      <div class="home-grid engineer-grid">
        {#each shownEngineers as engineer (engineer.id || engineer.name)}
          <a class="engineer-card" href={engineer.id ? '/community/' + encodeURIComponent(engineer.id) : '/community'}>
            {#if engineer.avatarUrl}<img class="engineer-avatar" src={engineer.avatarUrl} alt="" loading="lazy" />{:else}<span class="engineer-avatar" aria-hidden="true"></span>{/if}
            <h3>{engineer.name}</h3>
            <p class="yt-meta">{#if engineer.identities.length}{engineer.identities.map((identity) => identity.name).join(' · ')} · {/if}{$t('关注')} {engineer.admireCount}</p>
          </a>
        {/each}
      </div>
    </section>
    <section class="home-shelf">
      <div class="shelf-head"><h2>{$t('热门硬件')}</h2><a href="/hardware">{$t('硬件数据库')} →</a></div>
      <div class="home-grid">
        {#each shownHardware as hardware (hardware.id || hardware.name)}
          <a class="yt-card" href="/hardware">
            <span class="yt-thumb yt-hardware"></span>
            <div class="yt-body">
              <div>
                <h3>{hardware.name}</h3>
                <p class="yt-meta">{[hardware.brand, hardware.subtitle].filter(Boolean).join(' · ')}</p>
                <p class="yt-meta">♥ {hardware.likeCount}</p>
              </div>
            </div>
          </a>
        {/each}
      </div>
    </section>
  {:else if section === 'brief' || section === 'brief.html'}
    <article class="prose"><h1>{$t('组织简介')}</h1><p>{$t('待定')}</p></article>
  {:else if section === 'rules' || section === 'rules.html'}
    <article class="prose"><h1>{$t('协会会规')}</h1><p>{$t('敬请期待！')}</p></article>
  {:else if loading}<p role="status">{$t('正在加载…')}</p>
  {:else if error}<p class="message error" role="alert">{$t(error, errorParams)}</p>
  {:else if section === 'community'}
    {#if id}
      <a href="/community">{$t('返回成员列表')}</a><header class="profile-heading">{#if data.avatarUrl}<img class="avatar" src={data.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<div><h1>{data.name}</h1><p>{data.bio}</p>{#if data.currentStatus}<span class:busy={data.currentStatus === 'busy'} class="public-current-status">{$t('当前状态')}：{$t(currentStatusLabels[data.currentStatus] || data.currentStatus)}</span>{/if}</div></header>
      {#if data.identities?.length}<div class="member-identities">{#each data.identities as identity}<span class={`rank rank-pill rank-${identity.category === 'main' ? 4 : 3}`}>{$t(identity.category === 'main' ? '主力职业' : '辅助职业')} · {identity.name}</span>{/each}</div>{/if}
      {#if data.relations}
        <div class="relation-panel">
          <div class="relation-counts">
            <span>{$t('被关注')} <strong>{data.relations.admireCount}</strong></span>
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
      {#if !(data.hiddenSections || []).includes('cards')}{#if data.cards?.length}<div class="public-grid">{#each data.cards as card}<article class="public-card"><h2>{card.title}</h2><p>{card.text}</p><div class="image-grid">{#each card.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={card.title} loading="lazy" /></a>{/each}</div></article>{/each}</div>{:else}<p>{$t('暂无公开卡片。')}</p>{/if}{/if}
      <nav class="profile-tabs" aria-label={$t('公开资料')}>
        {#if !(data.hiddenSections || []).includes('projects')}<button type="button" class:active={activeTab === 'completed'} on:click={() => activeTab = 'completed'}>{$t('完成项目')}<span>{data.projectsCompleted?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'ongoing'} on:click={() => activeTab = 'ongoing'}>{$t('进行中项目')}<span>{data.projectsOngoing?.length || 0}</span></button>{/if}
        {#if !(data.hiddenSections || []).includes('career')}<button type="button" class:active={activeTab === 'career'} on:click={() => activeTab = 'career'}>{$t('职业等级')}<span>{data.career?.length || 0}</span></button>{/if}
        {#if !(data.hiddenSections || []).includes('skills')}<button type="button" class:active={activeTab === 'skills'} on:click={() => activeTab = 'skills'}>{$t('技能展示')}<span>{shownSkills.length}</span></button>{/if}
      </nav>
      {#if activeTab === 'completed'}
        {#if data.projectsCompleted?.length}
          <div class="public-grid">
            {#each data.projectsCompleted as project}
              <article class="public-card project-card">
                <div class="project-meta"><span class="project-status done">{$t('已完成')}</span>{#if project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{:else}<span class="rank-pending">{$t('难度')} {$t('待定')}</span>{/if}</div>
                <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
                <p>{project.summary}</p>
                <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>
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
                <div class="project-meta"><span class="project-status">{$t('进行中')}</span>{#if project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{:else}<span class="rank-pending">{$t('难度')} {$t('待定')}</span>{/if}</div>
                <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
                <p>{project.summary}</p>
                <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>
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
                <div class="career-head"><h3>{badge.careerName}</h3><span class={`rank rank-text rank-${levelRank(badge.level)}`}>{badge.levelTitle || `Lv.${badge.level}`}</span></div>
                <p>{badge.totalXp} XP · {$t('最高等级')} Lv.{badge.maxLevel}</p>
                <div class="progress" role="img" aria-label={`${badge.totalXp} / ${badge.nextLevelXp} XP`}><span style={`width:${careerProgress(badge)}%`}></span></div>
                {#if badge.nextLevelXp}<small>{$t('下一等级')} {badge.nextLevelTitle} · {badge.nextLevelXp} XP</small>{:else}<small>{$t('已达到当前最高等级')}</small>{/if}
                {#if medalLevel(badge.level) >= 10}
                  <div class="milestones" aria-label={$t('徽章占位')}>
                    <span class={`rank rank-disc rank-${levelRank(badge.level)}`} title={`Lv.${badge.level} ${$t('徽章占位')}`}><span class="flash-layer" aria-hidden="true"></span>{medalLevel(badge.level)}</span>
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        {:else}
          <p class="tab-empty">{$t('暂无职业等级。')}</p>
        {/if}
      {:else if activeTab === 'skills'}
        {#if skillError}<p class="message error" role="alert">{$t(skillError)}</p>{/if}
        {#if shownSkills.length}
          <ul class="public-skill-list">
            {#each shownSkills as skill (skill.userSkillId)}
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
                    <button type="button" class:liked={skill.likedByMe} class="like-button" on:click={() => endorseSkill(skill)} aria-pressed={skill.likedByMe} aria-label={$t(skill.likedByMe ? '取消认可' : '认可')}>
                      <svg class="thumb" viewBox="0 0 24 24" width="14" height="14" fill={skill.likedByMe ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                        <path d="M7 10v12" />
                        <path d="M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h2.76a2 2 0 0 0 1.79-1.11L12 2a3.13 3.13 0 0 1 3 3.88Z" />
                      </svg>
                      <span>{skill.likedByMe ? $t('你已认可') : $t('认可')}</span>
                      <span class="like-sep" aria-hidden="true">·</span>
                      <span class="like-number">{skill.likeCount}</span>
                    </button>
                  {:else}
                    <span class="like-count"><svg class="thumb" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 10v12" /><path d="M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h2.76a2 2 0 0 0 1.79-1.11L12 2a3.13 3.13 0 0 1 3 3.88Z" /></svg> {skill.likeCount}</span>
                  {/if}
                  {#if skill.likeTier > 0}<span class={`recognized rank rank-pill rank-${skill.likeTier}`}><span class="flash-layer" aria-hidden="true"></span>{$t(recognizedLabels[skill.likeTier])}</span>{/if}
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
        <div class="project-meta"><span class:done={data.status === 'completed'} class="project-status">{statusLabel(data.status)}</span>{#if data.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(data.difficulty)}`}>{$t('难度')} {data.difficulty}</span>{:else}<span class="rank-pending">{$t('难度')} {$t('待定')}</span>{/if}{#if data.currentStage}<span class="project-stage">{$t('当前阶段')}：{data.currentStage}</span>{/if}</div>
        <h1>{data.title}</h1>
        <p>{data.summary}</p>
        <p class="project-owner">{$t('负责人')}：{data.ownerName || '—'}{#if data.updated} · {$t('更新于')} {formatDate(data.updated)}{/if}</p>
        {#if data.description && data.description !== data.summary}<p class="project-description">{data.description}</p>{/if}
        {#if data.estimatedDurationDays}<p class="project-owner">{$t('预计周期')}：{data.estimatedDurationDays} {$t('天')}</p>{/if}
        {#if data.proposedDifficulty}<p class="project-owner">{$t('发起人自评')}：<span class={`rank rank-text rank-${difficultyRank(data.proposedDifficulty)}`}>{data.proposedDifficulty}</span></p>{:else}<p class="project-owner">{$t('发起人自评')}：{$t('待定')}</p>{/if}
        {#if data.difficulty}<p class="project-owner">{$t('共识')}：<span class={`rank rank-text rank-${difficultyRank(data.difficulty)}`}>{data.difficulty}</span></p>{:else}<p class="project-owner">{$t('共识')}：{$t('待定')}</p>{/if}
        {#if data.ownerConfirmed}<span class="confirmed">✓ {$t('发起人已确认')}</span>{/if}
        {#if data.approveCount}<span>{$t('同意')}：{data.approveCount}</span>{/if}
        {#if data.opposeCount}<span>{$t('反对')}：{data.opposeCount}</span>{/if}
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
          <h2 id="project-apply-title">{$t(data.joinMode === 'open' ? '加入项目' : '申请加入')}</h2>
          <p>{data.joinMode === 'open' ? $t('此项目允许自由加入，点击下方按钮直接成为成员。') : $t('此项目需要申请，填写你的技能和可投入时间。')}</p>
          {#if data.joinMode !== 'open'}
            <label>{$t('你的职业')}<input bind:value={applyProfession} maxlength="100" placeholder={$t('例如：电子工程师、机械设计师')} /></label>
            <label>{$t('可投入时间')}<input bind:value={applyTime} maxlength="100" placeholder={$t('例如：每周 10 小时、周末')} /></label>
            <label>{$t('能力上限')}<textarea bind:value={applyAbility} maxlength="200" rows="2" placeholder={$t('简单描述你的技能水平和能承担的工作范围')}></textarea></label>
          {/if}
          <label>{$t('补充说明')}<textarea bind:value={applyMessage} maxlength="500" rows="3" placeholder={$t('例如：我可以负责结构设计与打样。')}></textarea></label>
          {#if applyError}<p class="message error" role="alert">{$t(applyError)}</p>{/if}
          <button type="button" class="primary" on:click={applyToProject} disabled={applying}>{applying ? $t('正在提交…') : $t(data.joinMode === 'open' ? '直接加入' : '提交申请')}</button>
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
              <div class="project-meta"><span class:done={project.status === 'completed'} class="project-status">{statusLabel(project.status)}</span>{#if project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('难度')} {project.difficulty}</span>{:else}<span class="rank-pending">{$t('难度')} {$t('待定')}</span>{/if}</div>
              <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
              <p>{project.summary}</p>
              <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：{project.ownerName || '—'}{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>
            </article>
          {/each}
        </div>
      {:else}<p>{$t('暂无公开项目。')}</p>{/if}
    {/if}
  {:else}<h1>{$t('页面不存在')}</h1><a href="/">{$t('返回首页')}</a>{/if}
</main>

<style>
  .public-hero {
    position: relative;
    overflow: hidden;
    padding: 48px 44px;
    margin: 8px 0 42px;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    background:
      linear-gradient(to right, rgba(15, 138, 95, 0.055) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(15, 138, 95, 0.055) 1px, transparent 1px),
      linear-gradient(135deg, rgba(255, 255, 255, 0.94) 0%, rgba(226, 245, 234, 0.86) 100%);
    background-size: 26px 26px, 26px 26px, auto;
    box-shadow: var(--shadow-md);
  }
  .hero-orb {
    position: absolute;
    right: -46px;
    top: -46px;
    width: 280px;
    height: 280px;
    border-radius: 50%;
    border: 1px dashed rgba(15, 138, 95, 0.28);
    display: grid;
    place-items: center;
    pointer-events: none;
    animation: mint-spin 56s linear infinite;
  }
  .hero-orb span {
    position: absolute;
    width: 72%;
    height: 72%;
    border-radius: 50%;
    border: 1px solid rgba(15, 138, 95, 0.22);
  }
  .hero-orb span:nth-child(2) {
    width: 40%;
    height: 40%;
    background: radial-gradient(circle, rgba(24, 181, 125, 0.26), rgba(15, 138, 95, 0.04) 70%);
    border-style: dashed;
  }
  .hero-badge {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 5px 12px;
    margin-bottom: 16px;
    border-radius: 999px;
    border: 1px solid var(--border-strong);
    background: rgba(255, 255, 255, 0.78);
    color: var(--accent-deep);
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.1em;
  }
  .hero-badge i {
    display: inline-block;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--accent-bright);
    animation: mint-pulse 2.4s ease-in-out infinite;
  }
  .public-hero h1 {
    position: relative;
    font-size: clamp(34px, 5.2vw, 56px);
    line-height: 1.12;
    letter-spacing: -0.035em;
    margin: 0 0 12px;
    background: linear-gradient(125deg, var(--ink-primary) 20%, var(--accent-deep) 100%);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }
  .hero-en {
    position: relative;
    margin: 0 0 8px;
    font-family: var(--font-mono);
    font-size: 13px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--accent);
    font-weight: 600;
  }
  .hero-motto {
    position: relative;
    margin: 0;
    max-width: 52ch;
    color: var(--ink-secondary);
    font-size: 16px;
    line-height: 1.75;
  }
  .hero-actions {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    margin: 24px 0 0;
  }
  .hero-actions a {
    display: inline-flex;
    align-items: center;
    padding: 10px 20px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-default);
    background: var(--surface-elevated);
    color: var(--accent-deep);
    text-decoration: none;
    font-size: 13.5px;
    font-weight: 600;
    box-shadow: var(--shadow-xs);
    transition: transform var(--t-fast) var(--ease-out), background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out);
  }
  .hero-actions a:first-child {
    border-color: transparent;
    background: linear-gradient(135deg, var(--accent), var(--accent-deep));
    color: #fff;
    box-shadow: var(--shadow-glow);
  }
  .hero-actions a:hover {
    transform: translateY(-2px);
  }
  .hero-actions a:first-child:hover {
    background: linear-gradient(135deg, var(--accent-bright), var(--accent));
  }
  .hero-actions a:not(:first-child):hover {
    background: var(--surface-tint);
    border-color: var(--border-strong);
  }
  .project-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 10px; }
  .project-status { display: inline-block; padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-soft); color: var(--ink-secondary); font-size: 12px; font-weight: 500; }
  .project-status.done { background: var(--surface-tint); border-color: var(--border-strong); color: var(--accent-deep); }
  .project-apply { max-width: 640px; margin: 18px 0; padding: 20px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-sm); }
  .project-apply h2 { margin: 0 0 6px; font-size: 16px; }
  .project-apply p { margin: 0 0 10px; color: var(--ink-muted); font-size: 13px; }
  .project-apply textarea { width: 100%; padding: 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; resize: vertical; }
  .project-apply label { display: grid; gap: 5px; margin-bottom: 10px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; }
  .project-apply input { width: 100%; padding: 9px 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .project-apply input:focus, .project-apply textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  .project-apply .message { margin: 8px 0; }
  .project-apply-hint { color: var(--ink-muted); font-size: 13px; }
  .follow-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin: -8px 0 18px; }
  .follow-button { display: inline-flex; align-items: center; gap: 6px; padding: 7px 14px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-elevated); color: var(--accent-deep); font-size: 13px; font-weight: 600; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), transform var(--t-fast) var(--ease-out); }
  .follow-button:hover:not(:disabled) { background: var(--surface-tint); border-color: var(--border-strong); transform: translateY(-1px); }
  .follow-button.following { border-color: #e3bcc4; background: #fff4f6; color: #a84358; }
  .follow-error { color: var(--danger); font-size: 12px; }
  .confirmed { color: var(--accent-deep); font-size: 12px; font-weight: 600; }
  .project-stage { color: var(--ink-muted); font-size: 12px; }
  .project-owner { margin: 0 0 10px; color: var(--ink-muted); font-size: 13px; }
  .owner-avatar { width: 22px; height: 22px; margin-right: 6px; border-radius: 50%; object-fit: cover; vertical-align: middle; background: var(--surface-tint); border: 1px solid var(--border-subtle); }
  .project-card h2 a { text-decoration: none; color: var(--ink-primary); transition: color var(--t-fast) var(--ease-out); }
  .project-card:hover h2 a { color: var(--accent-deep); }
  .project-card p { margin: 0 0 10px; color: var(--ink-muted); }
  .project-card p.project-owner { margin-bottom: 0; }
  .project-description { margin-top: 16px; max-width: 70ch; white-space: pre-wrap; color: var(--ink-secondary); }
  .project-detail-head { margin: 24px 0 32px; }
  .project-detail-head h1 { margin: 8px 0 12px; letter-spacing: -0.025em; }
  .project-section { margin: 32px 0; }
  .project-section h2 { margin-bottom: 12px; }
  .project-stages { display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
  .project-stages li { padding: 8px 0 8px 16px; border-left: 2px solid var(--border-default); }
  .project-stages li.current { border-color: var(--accent-bright); }
  .project-stages p { margin: 4px 0; color: var(--ink-muted); font-size: 13px; }
  .project-stage-state { color: var(--ink-subtle); font-size: 12px; }
  .project-members { display: grid; gap: 8px; margin: 0; padding: 0; list-style: none; }
  .project-members li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; }
  .project-members span { color: var(--ink-muted); font-size: 13px; }
  .public-skill-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr)); gap: 16px; margin: 0; padding: 0; list-style: none; }
  .public-skill { padding: 18px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); transition: transform var(--t-med) var(--ease-out), border-color var(--t-med) var(--ease-out), box-shadow var(--t-med) var(--ease-out); }
  .public-skill:hover { transform: translateY(-2px); border-color: var(--border-strong); box-shadow: var(--shadow-sm); }
  .public-skill-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; }
  .public-skill-images { display: grid; grid-template-columns: repeat(auto-fill, minmax(80px, 1fr)); gap: 8px; margin-top: 10px; }
  .public-skill-images img { display: block; width: 100%; height: 80px; object-fit: cover; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); }
  .public-skill-links { margin: 8px 0 0; padding-left: 16px; font-size: 12px; }
  .public-skill-links li { margin: 2px 0; overflow-wrap: anywhere; }
  .public-skill-links a { color: var(--accent-deep); }
  .public-skill p { margin: 6px 0; color: var(--ink-muted); font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .public-skill-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 10px; }
  .meta { color: var(--ink-subtle); font-size: 12px; }
  .member-identities { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0 20px; }
  .public-current-status { display: inline-block; margin-top: 8px; padding: 3px 10px; border-radius: 999px; background: var(--surface-tint); border: 1px solid var(--border-subtle); color: var(--accent-deep); font-size: 12px; font-weight: 500; }
  .public-current-status.busy { background: var(--warn-bg); border-color: #ecd1a2; color: var(--warn-ink); }
  .relation-panel { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 18px; margin: 0 0 22px; padding: 14px 16px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); }
  .relation-counts { display: flex; flex-wrap: wrap; gap: 14px; color: var(--ink-muted); font-size: 12.5px; }
  .relation-counts strong { color: var(--accent-deep); font-family: var(--font-mono); }
  .relation-actions { display: flex; flex-wrap: wrap; gap: 6px; }
  .relation-actions button { padding: 5px 12px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-elevated); color: var(--ink-secondary); font-size: 12px; font-weight: 500; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out); }
  .relation-actions button:hover:not(:disabled) { background: var(--surface-tint); border-color: var(--border-strong); }
  .relation-actions button.active { border-color: var(--accent); background: var(--surface-tint); color: var(--accent-deep); font-weight: 600; }
  .relation-current, .relation-hint, .relation-error { flex-basis: 100%; margin: 0; color: var(--ink-muted); font-size: 12px; }
  .relation-error { color: var(--danger); }
  .profile-tabs { display: flex; flex-wrap: wrap; gap: 6px; margin: 18px 0 22px; padding-bottom: 10px; border-bottom: 1px solid var(--border-subtle); }
  .profile-tabs button { border: 1px solid transparent; border-radius: var(--radius-sm); background: none; padding: 8px 14px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; transition: background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .profile-tabs button:hover { background: var(--surface-tint); color: var(--accent-deep); }
  .profile-tabs button.active { background: linear-gradient(135deg, var(--accent), var(--accent-deep)); color: #fff; box-shadow: var(--shadow-glow); }
  .profile-tabs button span { margin-left: 6px; opacity: 0.78; font-family: var(--font-mono); font-size: 11.5px; }
  .career-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 14px; }
  .career-card { padding: 16px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); }
  .career-head { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
  .career-head h3 { margin: 0; font-size: 15px; }
  .career-head span { font-size: 12px; }
  .career-card p { margin: 6px 0; color: var(--ink-muted); font-size: 13px; font-family: var(--font-mono); }
  .career-card small { color: var(--ink-subtle); font-size: 12px; }
  .career-card .progress { height: 7px; margin: 10px 0 6px; border-radius: 999px; overflow: hidden; background: var(--surface-tint); }
  .career-card .progress span { display: block; height: 100%; border-radius: 999px; background: linear-gradient(90deg, var(--accent), var(--accent-bright)); }
  .milestones { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
  .tab-empty { padding: 20px 0; color: var(--ink-subtle); }
  .home-shelf { margin: 38px 0; }
  .shelf-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
  .shelf-head h2 { margin: 0; font-size: 20px; font-weight: 700; letter-spacing: -0.02em; }
  .shelf-head a { color: var(--accent-deep); font-size: 13px; font-weight: 600; text-decoration: none; padding: 4px 8px; border-radius: var(--radius-sm); transition: background-color var(--t-fast) var(--ease-out), transform var(--t-fast) var(--ease-out); }
  .shelf-head a:hover { background: var(--surface-tint); transform: translateX(2px); }
  .home-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(236px, 1fr)); gap: 18px; }
  .yt-card {
    display: block;
    padding: 12px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--surface);
    color: inherit;
    text-decoration: none;
    box-shadow: var(--shadow-xs);
    transition: transform var(--t-med) var(--ease-out), border-color var(--t-med) var(--ease-out), box-shadow var(--t-med) var(--ease-out);
  }
  .yt-card:hover {
    transform: translateY(-3px);
    border-color: var(--border-strong);
    box-shadow: var(--shadow-sm);
  }
  /* shortcut: 缩略图为科技风占位图，项目封面（media_assets）上传链路落地后替换 */
  .yt-thumb {
    position: relative;
    display: block;
    aspect-ratio: 16 / 9;
    border-radius: var(--radius-sm);
    border: 1px solid rgba(15, 138, 95, 0.14);
    background:
      linear-gradient(to right, rgba(15, 138, 95, 0.08) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(15, 138, 95, 0.08) 1px, transparent 1px),
      linear-gradient(135deg, #dff3e8 0%, #b5e1c9 100%);
    background-size: 18px 18px, 18px 18px, auto;
    overflow: hidden;
  }
  .yt-thumb.yt-dungeon {
    border-color: rgba(45, 118, 152, 0.16);
    background:
      linear-gradient(to right, rgba(45, 118, 152, 0.08) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(45, 118, 152, 0.08) 1px, transparent 1px),
      linear-gradient(135deg, #ddf0f4 0%, #add7e3 100%);
    background-size: 18px 18px, 18px 18px, auto;
  }
  .yt-thumb.yt-boss {
    border-color: rgba(176, 82, 82, 0.16);
    background:
      linear-gradient(to right, rgba(176, 82, 82, 0.07) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(176, 82, 82, 0.07) 1px, transparent 1px),
      linear-gradient(135deg, #f5e2e4 0%, #e5b7bd 100%);
    background-size: 18px 18px, 18px 18px, auto;
  }
  .yt-thumb.yt-hardware {
    border-color: rgba(58, 96, 82, 0.16);
    background:
      linear-gradient(to right, rgba(58, 96, 82, 0.08) 1px, transparent 1px),
      linear-gradient(to bottom, rgba(58, 96, 82, 0.08) 1px, transparent 1px),
      linear-gradient(135deg, #e1ece7 0%, #b8cfc5 100%);
    background-size: 18px 18px, 18px 18px, auto;
  }
  .yt-badge { position: absolute; top: 8px; right: 8px; }
  .yt-body { display: flex; gap: 10px; margin-top: 12px; }
  .yt-avatar { flex: none; width: 34px; height: 34px; border-radius: 50%; object-fit: cover; background: var(--surface-tint); border: 1px solid var(--border-subtle); }
  .yt-card h3 { margin: 0; font-size: 15px; line-height: 1.35; color: var(--ink-primary); display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .yt-meta { margin: 4px 0 0; color: var(--ink-muted); font-size: 12px; overflow-wrap: anywhere; }
  .engineer-grid { grid-template-columns: repeat(auto-fill, minmax(186px, 1fr)); }
  .engineer-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 22px 14px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--surface);
    color: inherit;
    text-decoration: none;
    text-align: center;
    box-shadow: var(--shadow-xs);
    transition: transform var(--t-med) var(--ease-out), border-color var(--t-med) var(--ease-out), box-shadow var(--t-med) var(--ease-out);
  }
  .engineer-card:hover {
    transform: translateY(-3px);
    border-color: var(--border-strong);
    box-shadow: var(--shadow-sm);
  }
  .engineer-avatar { width: 72px; height: 72px; border-radius: 50%; object-fit: cover; background: linear-gradient(135deg, #d7eee1, #b3dec6); border: 2px solid var(--surface-elevated); box-shadow: 0 0 0 2px var(--border-default); }
  .engineer-card h3 { margin: 12px 0 0; font-size: 15px; color: var(--ink-primary); }
  .like-button { display: inline-flex; align-items: center; gap: 5px; padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 999px; background: transparent; color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .like-button:hover { background: var(--surface-tint); border-color: var(--border-strong); }
  .like-button.liked { color: var(--accent-deep); border-color: var(--accent); background: var(--accent-soft); }
  .like-button .thumb,
  .like-count .thumb { flex: none; }
  .like-sep { opacity: 0.45; }
  .like-number { font-variant-numeric: tabular-nums; }
  .like-count { display: inline-flex; align-items: center; gap: 5px; color: var(--ink-muted); font-size: 12px; }
  .recognized { display: inline-flex; align-items: center; gap: 4px; padding: 2px 9px; border: 1px solid transparent; border-radius: 999px; font-size: 11px; font-weight: 600; letter-spacing: 0.02em; }
  .recognized::before { content: '✦'; }
  @media (max-width: 600px) {
    .public-hero { padding: 30px 22px; margin-bottom: 28px; }
    .hero-orb { width: 180px; height: 180px; right: -40px; top: -40px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .hero-orb, .hero-badge i { animation: none !important; }
  }

  /* Matrix theme for the existing homepage content. */
  .public-hero {
    min-height: 390px;
    padding: 72px 56px;
    background: linear-gradient(to right, #62ff9b0b 1px, transparent 1px),
      linear-gradient(to bottom, #62ff9b0b 1px, transparent 1px), #041008;
    background-size: 32px 32px;
  }
  .hero-orb {
    width: 280px; height: 280px; right: 10%; top: 50%;
    margin-top: -140px; border: 1px solid #4fa56d;
    box-shadow: none;
  }
  .hero-orb::before { content: ''; position: absolute; inset: -24px; border: 1px dashed #4fa56d70; border-radius: 50%; }
  .hero-orb span { border-color: #4fa56d99; }
  .hero-orb span:nth-child(2) { border: 1px solid #62ff9b; border-radius: 0; transform: rotate(45deg); background: #0a2515; box-shadow: none; }
  .hero-badge { background: #071b0e; color: var(--accent-bright); }
  .public-hero h1 { background: none; color: var(--ink-primary); }
  .hero-en { color: var(--accent-bright); }
  .hero-motto { color: var(--ink-secondary); }
  .hero-actions a:first-child, .hero-actions a:first-child:hover { background: #62e893; color: #031008; }
  .home-shelf { border-top: 1px solid var(--border-subtle); padding-top: 26px; }
  .yt-thumb, .yt-thumb.yt-dungeon, .yt-thumb.yt-boss, .yt-thumb.yt-hardware {
    border-color: var(--border-default);
    background: linear-gradient(to right, #62ff9b10 1px, transparent 1px),
      linear-gradient(to bottom, #62ff9b10 1px, transparent 1px),
      #0b2013;
    background-size: 20px 20px, 20px 20px, auto;
  }
  .engineer-avatar { background: linear-gradient(135deg, #143c24, #07150d); }
  @media (max-width: 600px) {
    .public-hero { min-height: 0; padding: 38px 24px; }
    .hero-orb { width: 150px; height: 150px; right: -50px; top: 36px; margin: 0; opacity: .45; }
  }
</style>
