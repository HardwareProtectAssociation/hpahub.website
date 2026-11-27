<script>
  import { onMount } from 'svelte';
  import { locale, t } from './i18n.js';
  import { difficultyRank, levelRank } from './tiers.js';
  import RedactedText from './RedactedText.svelte';
  import Select from './Select.svelte';
  import MarkdownContent from './MarkdownContent.svelte';
  import CardCanvas from './CardCanvas.svelte';
  export let pathname;
  export let token = '';
  let data = null;
  let projectVersions = [];
  let latestProjectVersion = 1;
  let selectedProjectVersion = '';
  let selectedProjectVersionEditor = '';
  let projectVersionSnapshot = null;
  let projectVersionLoading = false;
  let projectVersionError = '';
  let home = null;
  let error = '';
  let skillError = '';
  let applyMessage = '';
  let applyProfession = '';
  let identityOptions = [];
  let applyTime = '';
  let applyAbility = '';
  let applyError = '';
  let applyNotice = '';
  let applying = false;
  let acceptTerms = false;
  let management = null, managementError = '', managementBusy = false;
  let leaveReason = '';
  let leaveBusy = false;
  let leaveError = '';
  let removalReason = '';
  let removalTarget = '';
  let followBusy = false;
  let followError = '';
  let improvementMessage = '';
  let improvementError = '';
  let improvementNotice = '';
  let improving = false;
  let completionMessage = '';
  let completionBusy = false;
  let completionError = '';
  let relationBusy = '';
  let relationError = '';
  let activeTab = 'completed';
  let loading = true;
  let errorParams = {};
  let projectOffset = 0;
  let projectHasMore = false;
  let projectPageLoading = false;
  let projectPageError = '';
  const parts = pathname.split('/').filter(Boolean);
  const section = parts[0];
  const id = parts[1];
  const pageAction = parts[2];
  $: projectInfo = projectVersionSnapshot || data || {};
  $: projectVersionOptions = projectVersions.map((version) => ({
    value: String(version.version),
    label: `${$t('版本')} ${version.version}${version.created ? ` · ${formatDate(version.created)}` : ''} · ${$t('修改者')}：${version.editorName || $t('未填写')}`,
  }));
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
    if (section === 'showcase' && !id) {
      data = { items: [] };
      await loadMoreProjects();
      return;
    }
    try {
      const endpoint = section === 'community' ? 'members' : 'projects';
      const response = await fetch(`/api/hpahub/${endpoint}${id ? '/' + encodeURIComponent(id) : ''}`, {
        cache: 'no-store',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) { error = response.status === 404 ? '内容不存在或未公开。' : '加载失败（{status}）'; errorParams = { status: response.status }; return; }
      data = await response.json();
      if (section === 'showcase' && id) {
        await loadPublicProjectVersions();
        if (token) {
          await loadManagement();
        }
        if (data.canApply && token) {
          try {
            const response = await fetch('/api/hpahub/me/center', { cache: 'no-store', headers: { Authorization: `Bearer ${token}` } });
            if (!response.ok) throw new Error();
            identityOptions = (await response.json()).identityOptions || [];
          } catch { applyError = '身份列表加载失败，请刷新页面重试。'; }
        }
      }
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
  async function loadMoreProjects() {
    if (projectPageLoading || (projectOffset > 0 && !projectHasMore)) return;
    projectPageLoading = true;
    projectPageError = '';
    try {
      const response = await fetch(`/api/hpahub/projects?offset=${projectOffset}`, {
        cache: 'no-store',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error(`加载失败（${response.status}）`);
      const page = await response.json();
      data = { ...data, items: [...(data?.items || []), ...page.items] };
      projectOffset = page.nextOffset;
      projectHasMore = page.hasMore;
    } catch (e) {
      projectPageError = e.message;
    } finally {
      projectPageLoading = false;
      loading = false;
    }
  }

  async function loadPublicProjectVersions() {
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/versions`, {
        cache: 'no-store',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error();
      const result = await response.json();
      projectVersions = result.items || [];
      latestProjectVersion = Number(result.currentVersion || data?.version || 1);
      selectedProjectVersion = String(latestProjectVersion);
      selectedProjectVersionEditor = projectVersions.find((version) => Number(version.version) === latestProjectVersion)?.editorName || '';
    } catch {
      projectVersionError = '版本记录加载失败，请刷新页面重试。';
    }
  }

  async function selectPublicProjectVersion(event) {
    const version = Number(event.detail);
    if (!Number.isInteger(version) || version < 1) return;
    selectedProjectVersion = String(version);
    selectedProjectVersionEditor = projectVersions.find((item) => Number(item.version) === version)?.editorName || '';
    projectVersionSnapshot = null;
    projectVersionError = '';
    if (version === latestProjectVersion) return;
    projectVersionLoading = true;
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/versions/${version}`, {
        cache: 'no-store',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error();
      const result = await response.json();
      projectVersionSnapshot = result.snapshot || {};
      selectedProjectVersionEditor = result.editorName || selectedProjectVersionEditor;
    } catch {
      projectVersionError = '版本记录加载失败，请刷新页面重试。';
    } finally {
      projectVersionLoading = false;
    }
  }
  $: items = Array.isArray(data) ? data : data?.items || [];
  // shortcut: 演示卡片在 /api/hpahub/home 无数据时展示版式，真实内容稳定后删除
  const demoProjects = [
    { id: '', title: '智能家居中控系统', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 12, updated: '', difficulty: 'S', adventureStyle: 'project' },
    { id: '', title: '开源键盘固件', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 8, updated: '', difficulty: 'A', adventureStyle: 'dungeon' },
    { id: '', title: '桌面灯带控制器', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 5, updated: '', difficulty: 'B', adventureStyle: 'boss' },
    { id: '', title: '桌面电源管理器', ownerName: '示例工程师', ownerAvatarUrl: '', followCount: 3, updated: '', difficulty: 'C', adventureStyle: 'project' },
  ];
  const demoEngineers = [
    { id: '', name: '示例工程师 A', avatarUrl: '', identities: [{ name: '电子工程师' }], admireCount: 24 },
    { id: '', name: '示例工程师 B', avatarUrl: '', identities: [{ name: '机械设计师' }], admireCount: 12 },
    { id: '', name: '示例工程师 C', avatarUrl: '', identities: [{ name: '固件工程师' }], admireCount: 6 },
  ];
  $: shownProjects = home?.popularProjects?.length ? home.popularProjects : demoProjects;
  $: shownEngineers = home?.popularEngineers?.length ? home.popularEngineers : demoEngineers;
  $: featuredSkills = (data?.skills || []).filter((skill) => skill.featured);
  $: shownSkills = featuredSkills.length ? featuredSkills : (data?.skills || []);
  const statusLabels = { not_started: '未开始', ongoing: '已启动', long_term: '长期项目', paused: '暂停中', terminated: '已中止', completed: '已完成' };
  const hallStatuses = ['not_started', 'ongoing', 'long_term', 'paused', 'completed', 'terminated'];
  const natureLabels = { commercial: '商业盈利', learning: '学习交流', showcase: '技术展示', research: '科学研究' };
  let hallQuery = '';
  let hallSort = 'latest';
  let hallStatus = '';
  let hallNature = '';
  $: hallQueryText = hallQuery.trim().toLowerCase();
  $: hallProjects = [...items]
    .filter((project) => {
      if (hallStatus && project.status !== hallStatus) return false;
      if (hallNature && project.projectNature !== hallNature) return false;
      if (!hallQueryText) return true;
      return [project.title, project.creatorName, project.ownerName].some((value) => String(value || '').toLowerCase().includes(hallQueryText));
    })
    .sort((a, b) => {
      if (hallSort === 'hot') return (b.followCount || 0) - (a.followCount || 0) || (b.participantCount || 0) - (a.participantCount || 0);
      if (hallSort === 'difficulty') return difficultyRank(b.difficulty) - difficultyRank(a.difficulty) || (b.followCount || 0) - (a.followCount || 0);
      if (hallSort === 'participants') return (b.participantCount || 0) - (a.participantCount || 0);
      return new Date(b.publishedAt || 0) - new Date(a.publishedAt || 0);
    });
  const currentStatusLabels = { busy: '忙碌', free: '空闲' };
  const genderLabels = { male: '男', female: '女', nonbinary: '非二元' };
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

  async function loadManagement() {
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/management`, { headers: { Authorization: `Bearer ${token}` }, cache: 'no-store' });
      if (response.ok) management = await response.json();
    } catch { managementError = '加载管理信息失败'; }
  }
  async function submitCompletionRequest() {
    if (!completionMessage.trim() || completionBusy) return;
    completionBusy = true; completionError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/completions`, { method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ message: completionMessage }) });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '提交完成申请失败');
      data = { ...data, viewerCompletionPending: true, canRequestCompletion: false };
      completionMessage = '';
    } catch (err) { completionError = err.message || '提交完成申请失败'; }
    finally { completionBusy = false; }
  }
  async function updateManagement(path, body) {
    managementBusy = true; managementError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/${path}`, { method: 'PATCH', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      if (!response.ok) { const result = await response.json().catch(() => ({})); throw new Error(result.message || '操作失败'); }
      const result = await response.json().catch(() => null);
      if (path.includes('/owner-response') && result) {
        data = { ...data, departureRequest: result };
        if (result.status === 'approved') { window.location.reload(); return; }
      }
      await loadManagement();
    } catch (err) { managementError = err.message || '操作失败'; }
    finally { managementBusy = false; }
  }

  async function awardProjectTitle(member) {
    managementBusy = true; managementError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/titles`, { method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ userId: member.id }) });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '授予头衔失败');
      await loadManagement();
    } catch (err) { managementError = err.message || '授予头衔失败'; }
    finally { managementBusy = false; }
  }

  async function submitLeaveRequest() {
    if (!leaveReason.trim() || leaveBusy) return;
    leaveBusy = true; leaveError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/leave-requests`, { method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ reason: leaveReason }) });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '申请退出失败');
      data = { ...data, canRequestLeave: false, departureRequest: result };
      leaveReason = '';
    } catch (err) { leaveError = err.message || '申请退出失败'; }
    finally { leaveBusy = false; }
  }

  async function respondToRemoval(decision) {
    if (!data?.removalRequest || managementBusy) return;
    managementBusy = true; managementError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/departure-requests/${encodeURIComponent(data.removalRequest.id)}/member-response`, { method: 'PATCH', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ decision }) });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '回应失败');
      if (result.status === 'approved') { window.location.reload(); return; }
      data = { ...data, removalRequest: result, alreadyMember: result.status !== 'approved' };
      await loadManagement();
    } catch (err) { managementError = err.message || '回应失败'; }
    finally { managementBusy = false; }
  }

  async function submitRemovalRequest(member) {
    if (!removalReason.trim() || managementBusy) return;
    managementBusy = true; managementError = '';
    try {
      const response = await fetch(`/api/hpahub/projects/${encodeURIComponent(id)}/members/${encodeURIComponent(member.id)}/removal-requests`, { method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ reason: removalReason }) });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || '请离申请提交失败');
      removalReason = ''; removalTarget = '';
      await loadManagement();
    } catch (err) { managementError = err.message || '请离申请提交失败'; }
    finally { managementBusy = false; }
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
          acceptedTerms: acceptTerms ? data.confidentialityTerms : '',
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
        body: JSON.stringify({ message: improvementMessage, acceptedTerms: acceptTerms ? data.confidentialityTerms : '' }),
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
      <p class="hero-disclaimer"><span class="disclaimer-mark" aria-hidden="true">※</span>{$t('本网站所有内容纯属娱乐，世界上不存在硬件保护协会或任何相关组织。')}<span class="disclaimer-sign" aria-hidden="true"><span class="disclaimer-corrupt">[ERR 0x5F] UTF8_DECODE_FAILED: å…¬ä¼—èµ„è®¯åº”å¯¹åŠžå…¬å®¤</span><span class="disclaimer-tag">// {$t('数据已隐藏')}</span></span></p>
      <p class="hero-actions"><a href="/showcase">{$t('项目大厅')}</a><a href="/brief">{$t('协会简介')}</a><a href="/rules">{$t('协会会规')}</a></p>
    </section>
    <section class="home-shelf">
      <div class="shelf-head"><h2>{$t('热门项目')}</h2><a href="/showcase">{$t('项目大厅')} →</a></div>
      <div class="home-grid">
        {#each shownProjects as project (project.id || project.title)}
          <a class="yt-card" href={project.id ? '/showcase/' + encodeURIComponent(project.id) : '/showcase'}>
            <span class={`yt-thumb yt-${project.adventureStyle}`}>{#if project.difficulty}<span class={`yt-badge rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('项目级别')} {project.difficulty}</span>{:else}<span class="yt-badge rank-pending">{$t('项目级别')} {$t('待定')}</span>{/if}</span>
            <div class="yt-body">
              {#if project.ownerAvatarUrl}<img class="yt-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="yt-avatar" aria-hidden="true"></span>{/if}
              <div>
                <h3>{project.title}</h3>
                {#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
                <p class="yt-meta"><RedactedText text={project.ownerName || '—'} hidden={!!project.ownerRedacted} /></p>
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
            <p class="yt-meta">{#if engineer.identities.length}{#each engineer.identities as identity, i}{#if i} · {/if}<RedactedText text={identity.name} hidden={!!identity.redacted} />{/each} · {/if}{$t('关注')} {engineer.admireCount}</p>
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
      <a href="/community">{$t('返回成员列表')}</a>
      <header class="profile-heading member-profile-heading">
        {#if data.avatarRedacted}<span class="avatar redacted-avatar" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{:else if data.avatarUrl}<img class="avatar" src={data.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}
        <div class="member-profile-copy">
          <h1>{data.name}</h1>
          <p><RedactedText text={data.bio} hidden={!!data.bioRedacted} /></p>
            <div class="profile-badges" aria-label={$t('个人资料')}>
              {#if data.currentStatus}<span class={`profile-badge status-badge status-${data.statusRedacted ? 'redacted' : data.currentStatus}`}>{$t('当前状态')} · <RedactedText text={$t(currentStatusLabels[data.currentStatus] || data.currentStatus)} hidden={!!data.statusRedacted} /></span>{/if}
              {#if data.gender}<span class={`profile-badge gender-badge gender-${data.gender}`}>{$t('性别')} · {$t(genderLabels[data.gender] || data.gender)}</span>{/if}
              {#if data.identities?.length}
                {#each data.identities as identity}
                  <span class={`profile-badge role-badge role-${identity.category === 'main' ? 'main' : 'support'}`}>
                    {#if identity.redacted}<RedactedText text={identity.name} hidden={true} />{:else}{$t(identity.category === 'main' ? '主力职业' : '辅助职业')} · {identity.name}{/if}
                  </span>
                {/each}
              {:else}<span class="profile-badge role-badge">{$t('职业')} · {$t('未填写')}</span>{/if}
              {#if (data.earnedTitles || data.officialTitles || []).length}
                {#each data.earnedTitles || data.officialTitles || [] as title}<span class="profile-badge title-badge">{$t(title.isOfficial ? '官方头衔' : '项目头衔')} · {title.name}{#if !title.isOfficial && title.issuedByName} · {$t('授予者')}：{title.issuedByName}{/if}</span>{/each}
              {:else if data.wornTitle}<span class="profile-badge profile-fact">{$t('佩戴头衔')} · {data.wornTitle}</span>
              {:else}<span class="profile-badge title-badge">{$t('头衔')} · {$t('未填写')}</span>{/if}
            </div>
        </div>
      </header>
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
      {#if data.cards?.length}<div class="public-grid">{#each data.cards as card}<article class="public-card"><h2><RedactedText text={card.title} hidden={!!card.redacted} /></h2>{#if card.redacted}<p><RedactedText text={card.text} hidden={true} /></p>{:else if card.layout?.version === 1}<CardCanvas layout={card.layout} images={card.imageUrls || []} />{:else}<MarkdownContent text={card.text} />{/if}{#if card.redacted || card.layout?.version !== 1}<div class="image-grid">{#if card.redacted}{#each Array(card.imageCount || 0) as _}<span class="redacted-image" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{/each}{/if}{#each card.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={card.title} loading="lazy" /></a>{/each}</div>{/if}</article>{/each}</div>{:else}<p>{$t('暂无公开卡片。')}</p>{/if}
      <nav class="profile-tabs" aria-label={$t('公开资料')}>
        <button type="button" class:active={activeTab === 'completed'} on:click={() => activeTab = 'completed'}>{$t('完成项目')}<span>{data.projectsCompleted?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'ongoing'} on:click={() => activeTab = 'ongoing'}>{$t('进行中项目')}<span>{data.projectsOngoing?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'career'} on:click={() => activeTab = 'career'}>{$t('职业等级')}<span>{data.career?.length || 0}</span></button>
        <button type="button" class:active={activeTab === 'skills'} on:click={() => activeTab = 'skills'}>{$t('技能展示')}<span>{shownSkills.length}</span></button>
      </nav>
      {#if activeTab === 'completed'}
        {#if data.projectsCompleted?.length}
          <div class="public-grid">
            {#each data.projectsCompleted as project}
              <article class="public-card project-card">
                {#if project.redacted}<h2><RedactedText text={project.title} hidden={true} /></h2>{:else}
                <div class="project-meta"><span class="project-status done">{$t('已完成')}</span>{#if project.completionNumber}<span>{$t('第 {number} 次', { number: project.completionNumber })}</span>{/if}{#if project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('项目级别')} {project.difficulty}</span>{:else}<span class="rank-pending">{$t('项目级别')} {$t('待定')}</span>{/if}</div>
                <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
                {#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
                <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：<RedactedText text={project.ownerName || '—'} hidden={!!project.ownerRedacted} />{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>{/if}
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
                {#if project.redacted}<h2><RedactedText text={project.title} hidden={true} /></h2>{:else}
                <div class="project-meta"><span class="project-status">{$t('进行中')}</span>{#if project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{$t('项目级别')} {project.difficulty}</span>{:else}<span class="rank-pending">{$t('项目级别')} {$t('待定')}</span>{/if}</div>
                <h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2>
                {#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
                <p class="project-owner">{#if project.ownerAvatarUrl}<img class="owner-avatar" src={project.ownerAvatarUrl} alt="" loading="lazy" />{:else}<span class="owner-avatar" aria-hidden="true"></span>{/if}{$t('负责人')}：<RedactedText text={project.ownerName || '—'} hidden={!!project.ownerRedacted} />{#if project.updated} · {$t('更新于')} {formatDate(project.updated)}{/if}</p>{/if}
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
                {#if badge.redacted}<h3><RedactedText text={badge.careerName} hidden={true} /></h3><RedactedText text={badge.levelTitle} hidden={true} />{:else}
                <div class="career-head"><h3>{badge.careerName}</h3><span class={`rank rank-text rank-${levelRank(badge.level)}`}>{badge.levelTitle || `Lv.${badge.level}`}</span></div>
                <p>{badge.totalXp} XP · {$t('最高等级')} Lv.{badge.maxLevel}</p>
                <div class="progress" role="img" aria-label={`${badge.totalXp} / ${badge.nextLevelXp} XP`}><span style={`width:${careerProgress(badge)}%`}></span></div>
                {#if badge.nextLevelXp}<small>{$t('下一等级')} {badge.nextLevelTitle} · {badge.nextLevelXp} XP</small>{:else}<small>{$t('已达到当前最高等级')}</small>{/if}
                {#if medalLevel(badge.level) >= 10}
                  <div class="milestones" aria-label={$t('徽章占位')}>
                    <span class={`rank rank-disc rank-${levelRank(badge.level)}`} title={`Lv.${badge.level} ${$t('徽章占位')}`}><span class="flash-layer" aria-hidden="true"></span>{medalLevel(badge.level)}</span>
                  </div>
                {/if}
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
            {#each shownSkills as skill}
              <li class="public-skill">
                {#if skill.redacted}<strong><RedactedText text={skill.name} hidden={true} /></strong><p><RedactedText text={skill.abilityNote} hidden={true} /></p><p><RedactedText text={skill.proficiency} hidden={true} /></p><div class="image-grid">{#each Array(skill.imageCount || 0) as _}<span class="redacted-image" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{/each}</div>{#each skill.links || [] as link}<p><RedactedText text={link} hidden={true} /></p>{/each}{:else}
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
                </div>{/if}
              </li>
            {/each}
          </ul>
        {:else}
          <p class="tab-empty">{$t('暂无技能展示。')}</p>
        {/if}
      {/if}
    {:else}
      <h1>{$t('成员')}</h1><p>{$t('认识 HPA 的成员。')}</p>
      {#if items.length}
        <div class="public-grid">
          {#each items as member}
            <a class="public-card member-card member-card-link" href={member.id ? '/community/' + encodeURIComponent(member.id) : undefined}>
              <div class="member-card-head">
                {#if member.avatarUrl}<img class="avatar" src={member.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}
                <h2>{member.name}</h2>
              </div>
              <div class="member-card-details">
                <span class="member-card-item">{$t('职业')} · {member.smallCard?.profession || $t('未填写')}</span>
                <span class="member-card-item">{$t('头衔')} · {member.smallCard?.title || $t('未填写')}{#if member.smallCard?.title && member.smallCard.titleIssuer} · {$t('授予者')}：{member.smallCard.titleIssuer}{/if}</span>
                <span class="member-card-item">{$t('技能')} · {member.smallCard?.skill || $t('未填写')}</span>
                <div class="member-card-ratings">
                  <span class="member-card-label">{$t('获得评分')}</span>
                  <div class="member-card-rating-list" aria-label={$t('获得评分')}>
                    {#if member.smallCard?.ratings?.length}
                      {#each member.smallCard.ratings as rating}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(rating.grade)}`}>{rating.grade}{rating.count}</span>{/each}
                    {:else}
                      <span class="member-card-item member-card-empty">{$t('暂未获得')}</span>
                    {/if}
                  </div>
                </div>
              </div>
            </a>
          {/each}
        </div>
      {:else}<p>{$t('暂无成员。')}</p>{/if}
    {/if}
  {:else if section === 'showcase'}
    {#if id && pageAction === 'apply'}
      {#if data}
        <main class="dashboard project-apply-page">
          <a href={'/showcase/' + encodeURIComponent(data.id)}>{$t('返回项目详情')}</a>
          <header class="page-heading"><div><h1>{$t(data.joinMode === 'open' ? '加入项目' : '申请加入')}</h1><p>{data.title}</p></div></header>
          {#if !token}
            <section class="project-apply"><p>{$t('登录后可加入或申请加入此项目。')}</p><a class="primary" href="/sign-in">{$t('登录')}</a></section>
          {:else if data.canApply}
            <section class="project-apply" aria-labelledby="project-apply-title">
              <h2 id="project-apply-title">{$t(data.joinMode === 'open' ? '加入项目' : '申请加入')}</h2>
              <p>{data.joinMode === 'open' ? $t('此项目允许自由加入，填写身份和帮助后即可成为成员。') : $t('此项目需要申请，填写身份和帮助后等待审批。')}</p>
              <label>{$t('参与身份')}<select bind:value={applyProfession}><option value="">{$t('请选择身份')}</option>{#each identityOptions as identity}<option value={identity.name}>{identity.name}</option>{/each}</select></label>
              <label>{$t('可提供的帮助')}<textarea bind:value={applyAbility} maxlength="200" rows="2" placeholder={$t('例如：负责结构设计与打样')}></textarea></label>
              <label>{$t('可投入时间')}<input bind:value={applyTime} maxlength="100" placeholder={$t('例如：每周 10 小时、周末')} /></label>
              <label>{$t('补充说明')}<textarea bind:value={applyMessage} maxlength="500" rows="3" placeholder={$t('例如：我可以负责结构设计与打样。')}></textarea></label>
              {#if data.confidentialityRequested}<label class="agreement-check"><input type="checkbox" bind:checked={acceptTerms} />{$t('我同意保密协议')}</label>{/if}
              {#if applyError}<p class="message error" role="alert">{$t(applyError)}</p>{/if}
              <button type="button" class="primary" on:click={applyToProject} disabled={applying || !applyProfession || !applyAbility.trim() || (data.confidentialityRequested && !acceptTerms)}>{applying ? $t('正在提交…') : $t(data.joinMode === 'open' ? '直接加入' : '提交申请')}</button>
            </section>
          {:else if data.applicationPending}
            <p class="message success" role="status">{$t('申请已提交，等待管理员审批。')}</p>
          {:else if data.alreadyMember}
            <p class="message success" role="status">{$t('你已加入此项目。')}</p>
          {:else}
            <section class="project-apply"><p>{$t('该项目当前不可加入。')}</p></section>
          {/if}
        </main>
      {/if}
    {:else if id}
      {#if pageAction === 'content'}
        <header class="project-content-heading">
          <div><a href={'/showcase/' + encodeURIComponent(data.id)}>{$t('返回项目信息')}</a><p class="project-content-kicker">{$t('项目内容')}</p><h1>{data.title}</h1>{#if data.projectNumber}<small class="project-number">{data.projectNumber}</small>{/if}</div>
          {#if data.canFollow || data.canApply}<div class="project-detail-actions">
            {#if data.canFollow}<div class="follow-row">
              <button type="button" class:following={data.following} class="follow-button" on:click={toggleFollow} disabled={followBusy}>
                <span aria-hidden="true">{data.following ? '♥' : '♡'}</span>
                {data.following ? $t('已关注') : $t('关注项目')}
              </button>
              {#if followError}<span class="follow-error">{$t(followError)}</span>{/if}
            </div>{/if}
            {#if data.canApply}<a class="primary" href={'/showcase/' + encodeURIComponent(data.id) + '/apply'}>{$t(data.joinMode === 'open' ? '加入项目' : '申请加入')}</a>{/if}
          </div>{/if}
        </header>
      {:else}
      <a href="/showcase">{$t('返回项目列表')}</a>
      <header class="project-detail-head">
        <div class="project-meta"><span class:done={data.status === 'completed'} class="project-status">{statusLabel(data.status)}</span>{#if data.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(data.difficulty)}`}>{$t('项目级别')} {data.difficulty}</span>{:else}<span class="rank-pending">{$t('项目级别')} {$t('待定')}</span>{/if}{#if data.currentStage}<span class="project-stage">{$t('当前阶段')}：{data.currentStage}</span>{/if}</div>
        <div class="project-title-row">
          <h1>{projectInfo.title || data.title}</h1>
          {#if data.canFollow || data.canApply}<div class="project-detail-actions">
            {#if data.canFollow}<div class="follow-row">
              <button type="button" class:following={data.following} class="follow-button" on:click={toggleFollow} disabled={followBusy}>
                <span aria-hidden="true">{data.following ? '♥' : '♡'}</span>
                {data.following ? $t('已关注') : $t('关注项目')}
              </button>
              {#if followError}<span class="follow-error">{$t(followError)}</span>{/if}
            </div>{/if}
            {#if data.canApply}<a class="primary" href={'/showcase/' + encodeURIComponent(data.id) + '/apply'}>{$t(data.joinMode === 'open' ? '加入项目' : '申请加入')}</a>{/if}
          </div>{/if}
        </div>
        <p class="project-title-meta">{#if data.projectNumber}<span class="project-number">{data.projectNumber}</span>{/if}<span>{$t('发起人')}：<RedactedText text={data.creatorName || $t('未填写')} hidden={!!data.creatorRedacted} /></span><span>{$t('发布时间')}：{data.publishedAt ? formatDate(data.publishedAt) : $t('未填写')}</span></p>
        {#if data.coverUrl}<img class="project-detail-cover" src={data.coverUrl} alt={data.title} />{:else}<div class="project-cover-placeholder project-detail-cover" aria-hidden="true"><span>HPA HUB</span></div>{/if}
        <section class="project-section project-introduction" aria-labelledby="project-introduction-title">
          <h2 id="project-introduction-title">{$t('项目简介')}</h2>
          <p class="project-description">{projectInfo.summary || $t('未填写')}</p>
        </section>
        <section class="project-information" aria-label={$t('项目信息')}>
          <div class="project-information-heading">
            <h2>{$t('项目信息')}</h2>
            {#if projectVersions.length > 1}<div class="project-version-picker"><span>{$t('版本')}</span><Select bind:value={selectedProjectVersion} ariaLabel={$t('版本')} options={projectVersionOptions} disabled={projectVersionLoading} on:value={selectPublicProjectVersion} /></div>{/if}
          </div>
          {#if projectVersions.length}<p class="project-version-message project-version-editor">{$t('当前版本修改者')}：{selectedProjectVersionEditor || $t('未填写')}</p>{/if}
          {#if projectVersionLoading}<p class="project-version-message">{$t('加载版本中…')}</p>{/if}
          {#if projectVersionError}<p class="project-version-message" role="alert">{$t(projectVersionError)}</p>{/if}
          <div class="project-info-table-wrap"><table class="project-info-table"><tbody>
            <tr><th scope="row">{$t('负责人')}</th><td><RedactedText text={data.ownerName || $t('未填写')} hidden={!!data.ownerRedacted} />{#if data.updated}<span class="info-secondary"> · {$t('更新于')} {formatDate(data.updated)}</span>{/if}</td></tr>
            <tr><th scope="row">{$t('管理员')}</th><td>{#if data.managers?.length}{#each data.managers as manager, index}{#if index}、{/if}<RedactedText text={manager.name || $t('未填写')} hidden={!!manager.redacted} />{/each}{:else}{$t('未填写')}{/if}</td></tr>
            <tr><th scope="row">{$t('赞助者')}</th><td>{projectInfo.sponsor || $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('危险性')}</th><td>{projectInfo.dangerLevel ? $t(({ low: '低', medium: '中', high: '高', extreme: '极高' })[projectInfo.dangerLevel] || projectInfo.dangerLevel) : $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('难度')}</th><td>{projectInfo.taskDifficulty ? $t(({ easy: '简单', normal: '一般', hard: '困难', extreme: '极难' })[projectInfo.taskDifficulty] || projectInfo.taskDifficulty) : $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('项目设置')}</th><td class="info-tags"><span>{$t('是否接受赞助')}：{$t(projectInfo.acceptsSponsorship ? '是' : '否')}</span><span>{$t('允许他人管理')}：{$t(projectInfo.allowOthersManagement ? '是' : '否')}</span><span>{$t('是否需要保密')}：{$t(projectInfo.confidentialityRequested ? '是' : '否')}</span></td></tr>
            <tr><th scope="row">{$t('项目需要')}</th><td>{#if projectInfo.projectNeeds?.length}{projectInfo.projectNeeds.map((need) => $t(({ funding: '资金支持', technical: '技术支持', equipment: '设备支持', venue: '场地支持', manpower: '人手支持', promotion: '宣传支持', emotional: '情绪支持' })[need] || need) + (projectInfo.projectNeedDetails?.[need] ? '：' + projectInfo.projectNeedDetails[need] : '')).join(' · ')}{:else}{$t('未填写')}{/if}</td></tr>
            <tr><th scope="row">{$t('发起人可提供')}</th><td>{#if projectInfo.providerOffers?.length}{projectInfo.providerOffers.map((offer) => $t(({ technical: '技术', funding: '资金', equipment: '设备', venue: '场地', manpower: '人手', promotion: '宣传', title: '头衔认证' })[offer] || offer) + (projectInfo.providerOfferDetails?.[offer] ? '：' + projectInfo.providerOfferDetails[offer] : '')).join(' · ')}{:else}{$t('未填写')}{/if}</td></tr>
            <tr><th scope="row">{$t('协作方式')}</th><td>{projectInfo.collaborationMode ? $t(projectInfo.collaborationMode === 'independent' ? '独立项目' : '协作项目') : $t('未填写')}</td></tr>
            {#if projectInfo.collaborationMode === 'collaborative'}<tr><th scope="row">{$t('加入方式')}</th><td>{$t(projectInfo.joinMode === 'open' ? '自由加入' : '申请加入')}</td></tr>{/if}
            <tr><th scope="row">{$t('计划开始日期')}</th><td>{projectInfo.plannedStartDate ? formatDate(projectInfo.plannedStartDate) : $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('时间安排')}</th><td>{projectInfo.scheduleNote || $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('状态原因')}</th><td>{projectInfo.statusReason || $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('项目性质')}</th><td>{projectInfo.projectNature ? $t(({ commercial: '商业盈利', learning: '学习交流', showcase: '技术展示', research: '科学研究' })[projectInfo.projectNature]) : $t('未填写')}</td></tr>
            <tr><th scope="row">{$t('重复项目')}</th><td>{#if projectInfo.repeatable}{$t('是')} · {$t('每位参与者最多完成次数')}：{projectInfo.repeatLimit}{:else}{$t('否')}{/if}</td></tr>
            <tr><th scope="row">{$t('预计周期')}</th><td>{#if projectInfo.longTerm}{$t('长期项目')}{:else if projectInfo.estimatedDurationDays}{projectInfo.estimatedDurationDays} {$t('天')}{:else}{$t('未填写')}{/if}</td></tr>
          </tbody></table></div>
          <div class="project-content-link"><a class="primary" href={'/showcase/' + encodeURIComponent(data.id) + '/content'}>{$t('进入项目内容')}</a></div>
        </section>
      </header>
      {/if}
      {#if pageAction === 'content'}
      {#if data.confidentialityRequested && data.confidentialityTerms}
        <section class="project-section"><h2>{$t('保密协议')}</h2><p class="project-description">{data.confidentialityTerms}</p></section>
      {/if}
      {#if data.canRequestImprovement}
        <section class="project-apply" aria-labelledby="project-improvement-title">
          <h2 id="project-improvement-title">{$t('申请改进')}</h2>
          <p>{$t('这个项目已经完成，你可以提出改进方向或补充方案。')}</p>
          <textarea bind:value={improvementMessage} maxlength="500" rows="3" placeholder={$t('例如：我想改进外壳结构，降低装配难度。')}></textarea>
          {#if data.confidentialityRequested}
            <label class="agreement-check"><input type="checkbox" bind:checked={acceptTerms} />{$t('我同意保密协议')}</label>
          {/if}
          {#if improvementError}<p class="message error" role="alert">{$t(improvementError)}</p>{/if}
          <button type="button" class="primary" on:click={requestImprovement} disabled={improving || (data.confidentialityRequested && !acceptTerms)}>{improving ? $t('正在提交…') : $t('提交改进申请')}</button>
        </section>
      {:else if data.improvementPending}
        <p class="message success" role="status">{$t(improvementNotice || '改进申请已提交，等待管理员审批。')}</p>
      {:else if data.applicationPending}
        <p class="message success" role="status">{$t(applyNotice || '申请已提交，等待管理员审批。')}</p>
      {:else if data.alreadyMember}
        <p class="message success" role="status">{$t('你已加入此项目。')}</p>
        {#if data.repeatable}
          <section class="project-apply">
            <h2>{$t('重复任务完成')} · {data.viewerCompletionCount || 0}/{data.repeatLimit}</h2>
            {#if data.canRequestCompletion && !data.departureRequest && !data.removalRequest}
              <p>{$t('完成一次任务后提交说明，项目管理者审核通过后会计入你的完成项目；项目会继续开放。')}</p>
              <textarea bind:value={completionMessage} maxlength="5000" rows="4" placeholder={$t('请说明本次完成的任务或成果')}></textarea>
              {#if completionError}<p class="message error" role="alert">{$t(completionError)}</p>{/if}
              <button type="button" class="primary" on:click={submitCompletionRequest} disabled={completionBusy || !completionMessage.trim()}>{completionBusy ? $t('正在提交…') : $t('申请任务完成')}</button>
            {:else if data.viewerCompletionPending}
              <p class="message success" role="status">{$t('完成申请已提交，等待项目管理者审核。')}</p>
            {:else if (data.viewerCompletionCount || 0) >= data.repeatLimit}
              <p>{$t('你已达到该项目的个人完成次数上限。')}</p>
            {/if}
          </section>
        {/if}
        {#if data.departureRequest}
          <p class="message success" role="status">{$t(data.departureRequest.status === 'pending_admin' ? '退出申请已转交管理员审核。' : '退出申请已提交，等待项目发起人在48小时内处理。')}</p>
        {:else if data.removalRequest}
          <section class="project-apply">
            <h2>{$t('项目发起人申请你离开项目')}</h2><p>{$t('申请原因')}：{data.removalRequest.reason}</p>
            {#if data.removalRequest.status === 'pending_member'}
              <p>{$t('请在48小时内同意或拒绝；拒绝或超时将转交管理员审核。')}</p>
              <button type="button" class="primary" on:click={() => respondToRemoval('approve')} disabled={managementBusy}>{$t('同意请离')}</button>
              <button type="button" class="secondary" on:click={() => respondToRemoval('reject')} disabled={managementBusy}>{$t('拒绝并交管理员审核')}</button>
            {:else}<p class="message success">{$t('请离申请已转交管理员审核。')}</p>{/if}
          </section>
        {:else if data.canRequestLeave}
          <section class="project-apply">
            <h2>{$t('申请退出项目')}</h2><p>{$t('说明退出原因，项目发起人将在48小时内同意或拒绝；拒绝或超时后交管理员审核。')}</p>
            <textarea bind:value={leaveReason} maxlength="2000" rows="3" placeholder={$t('请填写退出原因（必填）')}></textarea>
            {#if leaveError}<p class="message error" role="alert">{$t(leaveError)}</p>{/if}
            <button type="button" class="secondary" on:click={submitLeaveRequest} disabled={leaveBusy || !leaveReason.trim()}>{leaveBusy ? $t('正在提交…') : $t('提交退出申请')}</button>
          </section>
        {/if}
      {:else if !token}
        <p class="project-apply-hint"><a href="/sign-in">{$t('登录')}</a>{$t('后可申请加入或改进。')}</p>
      {/if}
      {#if management}
        <section class="project-section">
          <h2>{$t('项目管理')}</h2>
          {#if managementError}<p class="message error" role="alert">{$t(managementError)}</p>{/if}
          {#if management.requests.length}<h3>{$t('加入申请')}</h3>
            {#each management.requests as request}
              <div class="management-row"><span>{request.name} · {$t(request.source === 'improvement' ? '申请改进' : '申请加入')} {request.profession} {request.ability} {request.time} {request.message}</span><button type="button" disabled={managementBusy} on:click={() => updateManagement('applications/' + encodeURIComponent(request.id), { status: 'accepted' })}>{$t('通过')}</button><button type="button" disabled={managementBusy} on:click={() => updateManagement('applications/' + encodeURIComponent(request.id), { status: 'rejected' })}>{$t('拒绝')}</button></div>
            {/each}
          {/if}
          {#if management.completionRequests?.length}<h3>{$t('任务完成申请')}</h3>
            {#each management.completionRequests as request}
              <div class="management-row"><span>{request.name} · {$t('第 {number} 次', { number: request.number })} · {request.message}</span><button type="button" class="primary" disabled={managementBusy} on:click={() => updateManagement('completion-requests/' + encodeURIComponent(request.id), { decision: 'approve' })}>{$t('通过并计入完成项目')}</button><button type="button" class="secondary" disabled={managementBusy} on:click={() => updateManagement('completion-requests/' + encodeURIComponent(request.id), { decision: 'reject' })}>{$t('拒绝')}</button></div>
            {/each}
          {/if}
          {#if management.isOwner && management.departureRequests?.some((request) => request.type === 'leave' && request.status === 'pending_owner')}
            <h3>{$t('退出申请')}</h3>
            {#each management.departureRequests.filter((request) => request.type === 'leave' && request.status === 'pending_owner') as request}
              <div class="management-row"><span>{request.memberName}：{request.reason} · {$t('截止')} {new Date(request.expiresAt).toLocaleString()}</span><button type="button" class="primary" disabled={managementBusy} on:click={() => updateManagement('departure-requests/' + encodeURIComponent(request.id) + '/owner-response', { decision: 'approve' })}>{$t('同意退出')}</button><button type="button" class="secondary" disabled={managementBusy} on:click={() => updateManagement('departure-requests/' + encodeURIComponent(request.id) + '/owner-response', { decision: 'reject' })}>{$t('拒绝并交管理员审核')}</button></div>
            {/each}
          {/if}
          {#if management.isOwner && management.titleAwardAvailable}
            <h3>{$t('项目头衔授予')} · {management.titleAwardName}</h3>
            <p>{$t('项目完成后，可向已加入的参与者授予此头衔。')}</p>
            {#each management.members as member}
              <div class="management-row"><span>{member.name}</span>{#if member.titleAwarded}<span class="muted">{$t('已授予')}</span>{:else}<button type="button" class="primary" disabled={managementBusy} on:click={() => awardProjectTitle(member)}>{$t('授予头衔')}</button>{/if}</div>
            {/each}
          {/if}
          {#if management.canAssign}<h3>{$t('项目管理权')}</h3>
            {#each management.members as member}
              <div class="management-row"><span>{member.name}</span><button type="button" disabled={managementBusy || member.id === management.viewerId} on:click={() => updateManagement('managers/' + encodeURIComponent(member.id), { manager: !member.manager })}>{$t(member.manager ? '撤销管理权' : '授予管理权')}</button></div>
            {/each}
          {/if}
          {#if management.isOwner}<h3>{$t('请离参与者')}</h3>
            {#each management.members.filter((member) => member.id !== management.viewerId) as member}
              <div class="management-row"><span>{member.name}{#if member.departureRequest}<span class="muted"> · {$t(member.departureRequest.status === 'pending_admin' ? '管理员审核中' : '等待参与者回应')}</span>{/if}</span>
                {#if !member.departureRequest}<button type="button" class="secondary" on:click={() => removalTarget = removalTarget === member.id ? '' : member.id}>{$t('申请请离')}</button>{/if}
              </div>
              {#if removalTarget === member.id}<div class="management-row"><textarea bind:value={removalReason} maxlength="2000" rows="2" placeholder={$t('请填写请离原因（必填）')}></textarea><button type="button" class="primary" disabled={managementBusy || !removalReason.trim()} on:click={() => submitRemovalRequest(member)}>{$t('提交请离申请')}</button></div>{/if}
            {/each}
          {/if}
        </section>
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
              <li><strong><RedactedText text={member.name || '—'} hidden={!!member.redacted} /></strong>{#if member.duties}<span><RedactedText text={member.duties} hidden={!!member.redacted} /></span>{/if}</li>
            {/each}
          </ul>
        {:else}
          <p>{$t('暂无协作成员。')}</p>
        {/if}
      </section>
      {/if}
    {:else}
      <h1>{$t('项目大厅')}</h1>
      <p>{$t('浏览公开的共创项目。')}</p>
      <div class="hall-toolbar">
        <input type="search" bind:value={hallQuery} placeholder={$t('搜索项目名称、简介或负责人')} aria-label={$t('搜索项目')} />
        <div class="hall-sort"><span>{$t('排序')}</span><Select bind:value={hallSort} ariaLabel={$t('排序')} options={[{ value: 'latest', label: $t('最新发布') }, { value: 'hot', label: $t('热度') }, { value: 'difficulty', label: $t('项目级别') }, { value: 'participants', label: $t('参与人数') }]} /></div>
      </div>
      <div class="hall-filters">
        <div class="filter-row" role="group" aria-label={$t('项目状态')}><button type="button" class:active={!hallStatus} on:click={() => hallStatus = ''}>{$t('全部')}</button>{#each hallStatuses as status}<button type="button" class:active={hallStatus === status} on:click={() => hallStatus = hallStatus === status ? '' : status}>{$t(statusLabels[status])}</button>{/each}</div>
        <div class="filter-row" role="group" aria-label={$t('项目性质')}><button type="button" class:active={!hallNature} on:click={() => hallNature = ''}>{$t('全部')}</button>{#each Object.entries(natureLabels) as [value, label]}<button type="button" class:active={hallNature === value} on:click={() => hallNature = hallNature === value ? '' : value}>{$t(label)}</button>{/each}</div>
      </div>
      <p class="hall-count">{$t('已加载 {count} 个项目', { count: items.length })}</p>
      {#if hallProjects.length}
        <div class="public-grid">
          {#each hallProjects as project}
            <a class="public-card project-card project-card-link" href={'/showcase/' + encodeURIComponent(project.id)}>
              {#if project.difficulty}<span class={`difficulty-star star-${difficultyRank(project.difficulty)}`} title={`${$t('项目级别')} ${project.difficulty}`} aria-label={`${$t('项目级别')} ${project.difficulty}`}>{project.difficulty}</span>{:else}<span class="difficulty-star star-0" title={$t('项目级别')} aria-label={$t('项目级别')}>?</span>{/if}
              {#if project.coverUrl}<img class="project-card-cover" src={project.coverUrl} alt={project.title} loading="lazy" />{:else}<div class="project-cover-placeholder project-card-cover" aria-hidden="true"><span>HPA HUB</span></div>{/if}
              <div class="project-meta"><span class:done={project.status === 'completed'} class="project-status">{statusLabel(project.status)}</span></div>
              <h2>{project.title}</h2>
              {#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
              <div class="project-people">
                <span class="people-block"><span class="people-label">{$t('发起人')}</span>{#if project.creatorRedacted}<span class="people-avatar hidden" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{:else if project.creatorAvatarUrl}<img class="people-avatar" src={project.creatorAvatarUrl} alt={project.creatorName || ''} title={project.creatorName || ''} loading="lazy" />{:else if project.creatorName}<span class="people-avatar" title={project.creatorName}></span>{:else}<span class="people-vacant">{$t('待加入')}</span>{/if}</span>
                <span class="people-block"><span class="people-label">{$t('首席工程师')}</span>{#if project.ownerRedacted}<span class="people-avatar hidden" title={$t('资料隐藏')} aria-label={$t('资料隐藏')}></span>{:else if project.ownerAvatarUrl}<img class="people-avatar" src={project.ownerAvatarUrl} alt={project.ownerName || ''} title={project.ownerName || ''} loading="lazy" />{:else if project.ownerName}<span class="people-avatar" title={project.ownerName}></span>{:else}<span class="people-vacant">{$t('待加入')}</span>{/if}</span>
                <span class="people-block"><span class="people-label">{$t('赞助者')}</span>{#if project.sponsor}<span>{project.sponsor}</span>{:else}<span class="people-vacant">{$t('待加入')}</span>{/if}</span>
              </div>
              <p class="project-owner">{$t('发布时间')}：{project.publishedAt ? formatDate(project.publishedAt) : '—'} · {$t('参与人数')}：{project.participantCount || 0}</p>
              <p class="project-owner">{$t('关注度')}：{project.followCount || 0}</p>
            </a>
          {/each}
        </div>
      {:else}<p>{$t(items.length ? '没有符合筛选条件的项目。' : '暂无公开项目。')}</p>{/if}
      {#if projectPageError}<p role="alert">{projectPageError}</p>{/if}
      {#if projectHasMore || projectPageLoading || projectPageError}
        <button class="secondary load-more" type="button" on:click={loadMoreProjects} disabled={projectPageLoading}>
          {$t(projectPageLoading ? '加载中…' : '加载更多项目')}
        </button>
      {/if}
    {/if}
  {:else}<h1>{$t('页面不存在')}</h1><a href="/">{$t('返回首页')}</a>{/if}
</main>

<style>
  .redacted-avatar { display:inline-block; background:#000; }
  .redacted-image { display:inline-block; width:80px; height:80px; background:#000; border-radius:var(--radius-sm); }
  .public-hero {
    position: relative;
    overflow: hidden;
    padding: 48px 44px;
    margin: 8px 0 42px;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    background: linear-gradient(135deg, var(--surface), var(--surface-elevated));
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
    color: var(--ink-primary);
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
  .hero-disclaimer {
    position: relative;
    display: block;
    max-width: 66ch;
    margin: 18px 0 0;
    padding: 9px 13px;
    border: 1px dashed var(--border-default);
    border-radius: var(--radius-sm);
    background: var(--surface-soft);
    color: var(--ink-muted);
    font-size: 12px;
    line-height: 1.75;
  }
  .disclaimer-mark {
    margin-right: 7px;
    color: var(--accent);
    font-weight: 700;
  }
  .disclaimer-sign {
    margin-left: 4px;
    color: var(--ink-subtle);
    white-space: normal;
    overflow-wrap: anywhere;
  }
  .disclaimer-tag {
    margin-left: 2px;
    padding: 0;
    color: var(--accent-deep);
    font-family: var(--font-mono), monospace;
    font-size: 11.5px;
    letter-spacing: -0.01em;
    vertical-align: baseline;
    white-space: nowrap;
  }
  .disclaimer-corrupt {
    color: var(--danger);
    font-family: var(--font-mono), monospace;
    letter-spacing: 0.02em;
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
    justify-content: center;
    min-height: 44px;
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
  .hero-actions a:focus-visible {
    outline: 2px solid var(--accent-bright);
    outline-offset: 3px;
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
  .project-apply select { width: 100%; min-height: 44px; padding: 9px 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .project-apply input:focus, .project-apply select:focus, .project-apply textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  .project-apply .agreement { white-space: pre-wrap; }
  .project-apply .agreement-check { display: flex; align-items: center; gap: 8px; }
  .project-apply .agreement-check input { width: auto; }
  .project-apply .message { margin: 8px 0; }
  .project-apply-hint { color: var(--ink-muted); font-size: 13px; }
  .follow-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin: -8px 0 18px; }
  .project-title-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
  .project-title-row h1 { flex: 1; min-width: 0; }
  .project-title-meta { display: flex; flex-wrap: wrap; gap: 8px 20px; margin: -2px 0 12px; color: var(--ink-muted); font-size: 13px; }
  .project-content-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin: 24px 0 32px; padding-bottom: 18px; border-bottom: 1px solid var(--border-subtle); }
  .project-content-heading h1 { margin: 8px 0 0; }
  .project-content-kicker { margin: 14px 0 0; color: var(--ink-muted); font-size: 12px; }
  .project-content-link { display: flex; justify-content: flex-end; margin-top: 18px; }
  .project-detail-actions { display: flex; flex: none; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 8px; }
  .project-detail-actions .follow-row { margin: 0; }
  .project-detail-actions button { white-space: nowrap; }
  .follow-button { display: inline-flex; align-items: center; gap: 6px; padding: 7px 14px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-elevated); color: var(--accent-deep); font-size: 13px; font-weight: 600; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), transform var(--t-fast) var(--ease-out); }
  .follow-button:hover:not(:disabled) { background: var(--surface-tint); border-color: var(--border-strong); transform: translateY(-1px); }
  .follow-button.following { border-color: #e3bcc4; background: #fff4f6; color: #a84358; }
  .follow-error { color: var(--danger); font-size: 12px; }
  .confirmed { color: var(--accent-deep); font-size: 12px; font-weight: 600; }
  .project-stage { color: var(--ink-muted); font-size: 12px; }
  .project-owner { margin: 0 0 10px; color: var(--ink-muted); font-size: 13px; }
  .project-people { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 18px; margin: 0 0 10px; }
  .people-block { display: inline-flex; align-items: center; gap: 6px; }
  .people-label { color: var(--ink-muted); font-size: 12px; }
  .people-avatar { display: inline-block; width: 26px; height: 26px; border-radius: 50%; object-fit: cover; background: var(--surface-tint); border: 1px solid var(--border-subtle); }
  .people-avatar.hidden { background: var(--ink-primary); }
  .people-vacant { display: inline-flex; align-items: center; padding: 2px 10px; border: 1px dashed var(--border-default); border-radius: 999px; color: var(--ink-subtle); font-size: 12px; }
  .people-avatar-slot { display: inline-flex; }
  .owner-avatar { width: 22px; height: 22px; margin-right: 6px; border-radius: 50%; object-fit: cover; vertical-align: middle; background: var(--surface-tint); border: 1px solid var(--border-subtle); }
  .project-card-link { display: block; color: inherit; text-decoration: none; }
  .hall-toolbar { display: flex; flex-wrap: wrap; gap: 10px; margin: 18px 0 12px; }
  .hall-toolbar input { flex: 1 1 260px; min-width: 0; padding: 9px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .hall-sort { display: inline-flex; align-items: center; gap: 8px; color: var(--ink-secondary); font-size: 13px; }
  .hall-filters { display: grid; gap: 8px; margin-bottom: 8px; }
  .filter-row { display: flex; flex-wrap: wrap; gap: 6px; }
  .filter-row button { padding: 4px 12px; border: 1px solid var(--border-default); border-radius: 999px; background: transparent; color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .filter-row button:hover { background: var(--surface-tint); }
  .filter-row button.active { border-color: var(--accent); background: var(--accent-soft); color: var(--accent-deep); }
  .hall-count { color: var(--ink-subtle); font-size: 12px; margin: 0 0 16px; }
  .project-card-cover { display: block; width: 100%; height: 150px; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 12px; background: var(--surface-tint); }
  .project-detail-cover { display: block; width: 100%; max-height: 420px; object-fit: cover; border-radius: var(--radius-md); border: 1px solid var(--border-subtle); margin: 16px 0 8px; }
  .project-cover-placeholder {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    aspect-ratio: 16 / 9;
    color: rgba(190, 235, 211, 0.72);
    background:
      linear-gradient(135deg, transparent 49.7%, rgba(99, 224, 150, 0.09) 50%, transparent 50.3%),
      radial-gradient(circle at 72% 28%, rgba(99, 224, 150, 0.24), transparent 24%),
      linear-gradient(135deg, #102b1e, #07150d 68%);
  }
  .project-cover-placeholder::before,
  .project-cover-placeholder::after {
    position: absolute;
    width: 42%;
    aspect-ratio: 1;
    border: 1px solid rgba(99, 224, 150, 0.2);
    border-radius: 50%;
    content: '';
  }
  .project-cover-placeholder::after { width: 28%; border-color: rgba(190, 235, 211, 0.16); }
  .project-cover-placeholder span { z-index: 1; font-size: 12px; font-weight: 700; letter-spacing: 0.26em; }
  .project-card-cover.project-cover-placeholder { height: 150px; }
  .project-detail-cover.project-cover-placeholder { height: auto; max-height: none; }
  .project-card-link:hover h2 { color: var(--accent-deep); }
  .project-card h2 a { text-decoration: none; color: var(--ink-primary); transition: color var(--t-fast) var(--ease-out); }
  .project-card:hover h2 a { color: var(--accent-deep); }
  .project-card p { margin: 0 0 10px; color: var(--ink-muted); }
  .project-card p.project-owner { margin-bottom: 0; }
  .project-description { margin-top: 16px; max-width: 70ch; white-space: pre-wrap; color: var(--ink-secondary); }
  .project-detail-head { margin: 24px 0 32px; }
  .project-information { margin: 20px 0; }
  .project-information-heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-bottom: 10px; }
  .project-information h2 { margin: 0; font-size: 17px; }
  .project-version-picker { display: flex; align-items: center; gap: 8px; color: var(--ink-subtle); font-size: 12px; }
  .project-version-picker .select { width: min(230px, 55vw); }
  .project-version-message { margin: 0 0 8px; color: var(--ink-subtle); font-size: 12px; }
  .project-info-table-wrap { overflow: hidden; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface-elevated); }
  .project-info-table { width: 100%; border-collapse: collapse; table-layout: fixed; color: var(--ink-secondary); font-size: 13px; }
  .project-info-table th, .project-info-table td { padding: 11px 14px; border-bottom: 1px solid var(--border-subtle); text-align: left; vertical-align: top; overflow-wrap: anywhere; }
  .project-info-table tr:last-child th, .project-info-table tr:last-child td { border-bottom: 0; }
  .project-info-table th { width: 22%; color: var(--ink-muted); background: var(--surface-soft); font-weight: 600; }
  .project-info-table td { white-space: pre-wrap; }
  .project-info-table .info-secondary { color: var(--ink-subtle); }
  .project-info-table .info-tags { display: flex; flex-wrap: wrap; gap: 6px 16px; }
  .project-info-table .info-tags span { white-space: nowrap; }
  .project-detail-head h1 { margin: 8px 0 12px; letter-spacing: -0.025em; }
  .project-section { margin: 32px 0; }
  .project-section h2 { margin-bottom: 12px; }
  .project-stages { display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
  .project-stages li { padding: 8px 0 8px 16px; border-left: 2px solid var(--border-default); }
  .project-stages li.current { border-color: var(--accent-bright); }
  .project-stages p { margin: 4px 0; color: var(--ink-muted); font-size: 13px; }
  .project-stage-state { color: var(--ink-subtle); font-size: 12px; }
  .management-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin: 10px 0; }
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
  .member-profile-heading { align-items: flex-start; gap: 20px; }
  .member-profile-copy { flex: 1; min-width: 0; }
  .member-profile-copy h1 { margin: 0 0 8px; }
  .member-profile-copy > p { margin: 0; }
  .member-profile-copy .profile-badges { margin-top: 12px; }
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
    .project-information-heading { align-items: flex-start; flex-wrap: wrap; }
    .project-version-picker { width: 100%; }
    .project-version-picker .select { flex: 1; width: auto; }
    .project-title-row { align-items: flex-start; flex-wrap: wrap; }
    .project-detail-actions { width: 100%; justify-content: flex-start; }
    .project-content-heading { align-items: flex-start; flex-direction: column; }
    .project-info-table th, .project-info-table td { padding: 9px 10px; }
    .project-info-table th { width: 32%; }
    .project-info-table .info-tags { display: grid; gap: 5px; }
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
  .yt-thumb, .yt-thumb.yt-dungeon, .yt-thumb.yt-boss {
    border-color: var(--border-default);
    background: linear-gradient(to right, #62ff9b10 1px, transparent 1px),
      linear-gradient(to bottom, #62ff9b10 1px, transparent 1px),
      #0b2013;
    background-size: 20px 20px, 20px 20px, auto;
  }
  .engineer-avatar { background: linear-gradient(135deg, #143c24, #07150d); }
  @media (max-width: 600px) {
    .public-hero { min-height: 0; padding: 32px 20px; margin-bottom: 28px; }
    .hero-orb { width: 130px; height: 130px; right: -55px; top: 22px; margin: 0; opacity: .25; }
    .hero-disclaimer { font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
    .hero-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
    .hero-actions a:first-child { grid-column: 1 / -1; }
    .hero-actions a { padding-inline: 10px; text-align: center; }
    .home-grid, .engineer-grid { grid-template-columns: minmax(0, 1fr); }
    .yt-card h3 { display: block; overflow: visible; overflow-wrap: anywhere; }
    .shelf-head { flex-wrap: wrap; }
    .home-shelf { margin: 28px 0; padding-top: 20px; }
    .home-grid { gap: 12px; }
    .yt-body > div { min-width: 0; }
    .engineer-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .engineer-card { min-width: 0; padding: 16px 10px; }
    .engineer-card h3 { max-width: 100%; overflow-wrap: anywhere; }
  }
  @media (max-width: 320px) {
    .engineer-grid { grid-template-columns: minmax(0, 1fr); }
  }
</style>
