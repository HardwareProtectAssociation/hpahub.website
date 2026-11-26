<script>
  import { onDestroy, onMount } from 'svelte';
  import { t } from './i18n.js';

  export let token;

  let data = null;
  let loading = true;
  let error = '';
  let notice = '';
  let avatarSrc = '';
  let editing = false;
  let saving = false;
  let formError = '';
  let formName = '';
  let formBio = '';
  let formPublic = false;
  let formGender = '';
  let formPersonality = '';
  let formAge = '';
  let formCurrentStatus = '';
  let formContacts = [];
  let formActivityAreas = [];
  let formIdentityIds = [];
  let activityDraft = '';
  let removeAvatar = false;
  let avatarFile = null;
  let avatarPreview = '';
  let invitations = [];
  let invitationError = '';
  let invitationNotice = '';
  let generatingInvitation = false;
  let inviteCount = 1;
  let inviteMaxUses = 1;
  let copiedCode = '';
  let hardwareConfig = [];
  let hwEditing = false;
  let hwSaving = false;
  let hwError = '';
  let hwNotice = '';
  let hwRows = [];
  const hwCategories = ['CPU', 'GPU', '主板', '内存', '存储', '显示器', '键盘', '鼠标', '耳机', '其他'];

  const genderLabels = { male: '男', female: '女', nonbinary: '非二元', private: '不愿透露' };
  const genderOptions = Object.entries(genderLabels);
  const currentStatusLabels = { busy: '忙碌', free: '空闲' };
  const personalityPresets = ['外向开朗', '内向沉稳', '理性冷静', '热情奔放', '细心谨慎', '大胆冒险', '幽默风趣', '勤奋踏实', '创新思维', '团队协作', '独立自主', '追求完美', '随和友善', '果断干练'];
  const contactTypes = [
    { value: 'email', label: '邮箱' },
    { value: 'wechat', label: '微信' },
    { value: 'qq', label: 'QQ' },
    { value: 'discord', label: 'Discord' },
    { value: 'phone', label: '电话' },
    { value: 'other', label: '其他' },
  ];
  const activityOptions = ['线上', '同城', '省内', '全国', '海外'];
  $: featuredIdentities = (data?.identities || []).filter((identity) => identity.featured);
  $: completedProjects = [...(data?.projects?.completed || []), ...(data?.projects?.assisted || [])];
  // shortcut: 演示数据用于查看难度标签配色，完成项目都设置 difficulty 后自动失效，之后可删
  const demoDifficultyTags = [{ level: 'S', count: 4 }, { level: 'A', count: 3 }, { level: 'B', count: 2 }, { level: 'C', count: 1 }, { level: 'D', count: 1 }];
  $: difficultyTags = ['S', 'A', 'B', 'C', 'D'].map((level) => ({ level, count: completedProjects.filter((project) => (project.difficulty || '').toUpperCase() === level).length })).filter((tag) => tag.count > 0);
  $: completedDifficulties = difficultyTags.length ? difficultyTags : (completedProjects.length ? demoDifficultyTags : []);
  // shortcut: 演示认可数用于查看五档徽章效果，真实点赞数据出现后自动失效，之后可删
  const demoLikeCounts = [10, 50, 100, 1000, 5000];
  $: realSkills = data?.skills || [];
  $: displaySkills = realSkills.some((skill) => (skill.likeCount || 0) > 0)
    ? realSkills
    : realSkills.map((skill, index) => (index < demoLikeCounts.length ? { ...skill, likeCount: demoLikeCounts[index], likeTier: index + 1 } : skill));
  $: totalLevel = (data?.badges || []).reduce((sum, badge) => sum + (badge.level || 0), 0);
  $: topSkill = (data?.skills || []).reduce((best, skill) => (!best || (skill.likeCount || 0) > (best.likeCount || 0) ? skill : best), null);

  const rewardLabels = { money: '资金', product: '实物', title: '头衔', credit: '署名', learning: '学习', none: '无', other: '其他' };
  const statusLabels = { ongoing: '进行中', paused: '已暂停', terminated: '已终止', completed: '已完成' };
  const recognizedLabels = { 1: '有点东西', 2: '他不对劲', 3: '他真会啊', 4: '他太强了！', 5: '他是超人！' };
  const projectStatusFilters = ['ongoing', 'completed', 'paused', 'terminated'];
  let projectTab = 'initiated';
  let projectStatusFilter = '';
  function projectItems(key) {
    const items = data?.projects?.[key] || [];
    return projectStatusFilter ? items.filter((project) => project.status === projectStatusFilter) : items;
  }

  async function loadAvatar(url) {
    if (!url) return '';
    try {
      const response = await fetch(url, { headers: token ? { Authorization: `Bearer ${token}` } : {}, cache: 'no-store' });
      if (!response.ok) return '';
      return URL.createObjectURL(await response.blob());
    } catch {
      return '';
    }
  }

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
      if (avatarSrc) URL.revokeObjectURL(avatarSrc);
      avatarSrc = await loadAvatar(body.profile?.avatarUrl);
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

  function milestones(badge) {
    const list = [];
    for (let level = 10; level <= (badge.maxLevel || 0); level += 10) list.push(level);
    return list;
  }

  function startEdit() {
    formName = data?.profile?.name || '';
    formBio = data?.profile?.bio || '';
    formPublic = !!data?.profile?.profilePublic;
    formGender = data?.profile?.gender || '';
    formPersonality = data?.profile?.personality || '';
    formAge = data?.profile?.age || '';
    formCurrentStatus = data?.profile?.currentStatus || '';
    formContacts = (data?.profile?.contacts || []).map((contact) => ({ ...contact }));
    formActivityAreas = [...(data?.profile?.activityAreas || [])];
    formIdentityIds = (data?.identities || []).filter((identity) => identity.featured).map((identity) => identity.id);
    activityDraft = '';
    removeAvatar = false;
    avatarFile = null;
    formError = '';
    notice = '';
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatarPreview = '';
    editing = true;
  }

  function addContact() {
    if (formContacts.length >= 5) return;
    formContacts = [...formContacts, { type: 'email', value: '' }];
  }

  function removeContact(index) {
    formContacts = formContacts.filter((_, itemIndex) => itemIndex !== index);
  }

  function addActivityArea() {
    const value = activityDraft.trim();
    if (!value) return;
    if (formActivityAreas.length >= 2) {
      formError = '最多选择 2 个活动范围。';
      return;
    }
    if (!formActivityAreas.includes(value)) formActivityAreas = [...formActivityAreas, value];
    activityDraft = '';
  }

  function removeActivityArea(index) {
    formActivityAreas = formActivityAreas.filter((_, itemIndex) => itemIndex !== index);
  }

  function togglePersonality(preset) {
    const parts = formPersonality.split(/[,，、\s]+/).filter(Boolean);
    const idx = parts.indexOf(preset);
    if (idx >= 0) {
      parts.splice(idx, 1);
    } else {
      parts.push(preset);
    }
    formPersonality = parts.join('、');
  }

  function isPersonalitySelected(preset) {
    return formPersonality.split(/[,，、\s]+/).includes(preset);
  }

  function toggleIdentity(id) {
    if (formIdentityIds.includes(id)) {
      formIdentityIds = formIdentityIds.filter((item) => item !== id);
      return;
    }
    if (formIdentityIds.length >= 3) {
      formError = '最多选择 3 个职业。';
      return;
    }
    formIdentityIds = [...formIdentityIds, id];
  }

  function cancelEdit() {
    editing = false;
    saving = false;
    formError = '';
    removeAvatar = false;
    avatarFile = null;
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatarPreview = '';
  }

  function selectAvatar(event) {
    const file = event.currentTarget.files?.[0] || null;
    avatarFile = file;
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatarPreview = file ? URL.createObjectURL(file) : '';
    if (file) removeAvatar = false;
  }

  async function saveProfile() {
    const name = formName.trim();
    if (!name) {
      formError = '请填写昵称。';
      return;
    }
    saving = true;
    formError = '';
    notice = '';
    try {
      const body = new FormData();
      body.set('name', name);
      body.set('bio', formBio);
      body.set('profilePublic', formPublic ? 'true' : 'false');
      body.set('gender', formGender);
      body.set('personality', formPersonality);
      body.set('age', String(formAge || 0));
      body.set('currentStatus', formCurrentStatus);
      body.set('contacts', JSON.stringify(formContacts.filter((contact) => contact.value.trim())));
      body.set('activityAreas', JSON.stringify(formActivityAreas));
      body.set('identityIds', JSON.stringify(formIdentityIds));
      if (avatarFile) body.set('avatar', avatarFile);
      if (removeAvatar) body.set('removeAvatar', 'true');
      const response = await fetch('/api/hpahub/me/center/profile', {
        method: 'PATCH',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body,
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || `请求失败（${response.status}）`);
      data = result;
      if (avatarSrc) URL.revokeObjectURL(avatarSrc);
      avatarSrc = await loadAvatar(result.profile?.avatarUrl);
      cancelEdit();
      notice = '资料已保存。';
    } catch (err) {
      formError = err.message || String(err);
    } finally {
      saving = false;
    }
  }

  async function loadInvitations() {
    try {
      const response = await fetch('/api/hpahub/invitations', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || `请求失败（${response.status}）`);
      invitations = body.items || [];
    } catch (err) {
      invitationError = err.message || String(err);
    }
  }

  async function generateInvitation() {
    generatingInvitation = true;
    invitationError = '';
    invitationNotice = '';
    try {
      const response = await fetch('/api/hpahub/invitations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({ count: Number(inviteCount), maxUses: Number(inviteMaxUses) }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || '生成失败，请重试。');
      await loadInvitations();
      invitationNotice = $t('已生成 {count} 个邀请码。', { count: Number(inviteCount) });
    } catch (err) {
      invitationError = err.message || String(err);
    } finally {
      generatingInvitation = false;
    }
  }

  async function copyInvitation(code) {
    let copied = false;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
        copied = true;
      }
    } catch {
      copied = false;
    }
    if (!copied) {
      try {
        const area = document.createElement('textarea');
        area.value = code;
        area.setAttribute('readonly', '');
        area.style.position = 'fixed';
        area.style.opacity = '0';
        document.body.appendChild(area);
        area.select();
        copied = document.execCommand('copy');
        area.remove();
      } catch {
        copied = false;
      }
    }
    if (!copied) {
      invitationError = $t('复制失败，请手动复制。');
      return;
    }
    copiedCode = code;
    invitationNotice = $t('邀请码已复制。');
    setTimeout(() => {
      if (copiedCode === code) copiedCode = '';
    }, 1500);
  }

  function invitationStatus(invitation) {
    if (invitation.maxUses === 0) return '无限次使用';
    if (invitation.maxUses > 1 && invitation.useCount < invitation.maxUses) return '使用中';
    if (invitation.used) return '已使用';
    if (invitation.expires > 0 && invitation.expires * 1000 <= Date.now()) return '已过期';
    return '未使用';
  }

  function invitationDate(value) {
    return value ? new Date(value * 1000).toLocaleString() : '—';
  }

  async function loadHardwareConfig() {
    try {
      const response = await fetch('/api/hpahub/me/hardware-config', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) return;
      hardwareConfig = body.items || [];
    } catch {}
  }

  function startHwEdit() {
    hwRows = hardwareConfig.map((item) => ({ ...item }));
    if (!hwRows.length) hwRows = [{ category: 'CPU', name: '', detail: '' }];
    hwError = '';
    hwNotice = '';
    hwEditing = true;
  }

  function addHwRow() {
    if (hwRows.length >= 50) return;
    hwRows = [...hwRows, { category: '其他', name: '', detail: '' }];
  }

  function removeHwRow(index) {
    hwRows = hwRows.filter((_, i) => i !== index);
  }

  async function saveHwConfig() {
    hwSaving = true;
    hwError = '';
    try {
      const items = hwRows.filter((r) => r.name.trim());
      const response = await fetch('/api/hpahub/me/hardware-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({ items }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message || '保存失败，请重试。');
      await loadHardwareConfig();
      hwEditing = false;
      hwNotice = '硬件配置已保存。';
    } catch (err) {
      hwError = err.message || String(err);
    } finally {
      hwSaving = false;
    }
  }

  onMount(async () => {
    await load();
    await loadInvitations();
    await loadHardwareConfig();
  });
  onDestroy(() => {
    if (avatarSrc) URL.revokeObjectURL(avatarSrc);
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
  });
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
      {#if avatarSrc}
        <img class="avatar" src={avatarSrc} alt="" />
      {:else}
        <span class="avatar avatar-default" aria-hidden="true"></span>
      {/if}
      <div class="profile-copy">
        <h1 id="center-name">{data.profile.name || data.profile.email}</h1>
        {#if data.profile.name && data.profile.email && data.profile.email !== data.profile.name}
          <p class="center-email">{data.profile.email}</p>
        {/if}
        {#if data.profile.bio}<p class="center-bio">{data.profile.bio}</p>{/if}
        {#if featuredIdentities.length}
          <div class="chips">
            {#each featuredIdentities as identity}
              <span class="chip">{$t(identity.category === 'main' ? '主力职业' : '辅助职业')} · {identity.name}</span>
            {/each}
          </div>
        {/if}
        {#if data.profile.gender || data.profile.age || data.profile.activityAreas?.length}
          <p class="profile-meta">
            {#if data.profile.currentStatus}<span class:busy={data.profile.currentStatus === 'busy'} class="current-status">{$t('当前状态')}：{$t(currentStatusLabels[data.profile.currentStatus] || data.profile.currentStatus)}</span>{/if}
            {#if data.profile.gender}<span>{$t('性别')}：{$t(genderLabels[data.profile.gender] || data.profile.gender)}</span>{/if}
            {#if data.profile.age}<span>{$t('年龄')}：{data.profile.age}</span>{/if}
            {#if data.profile.activityAreas?.length}<span>{$t('活动范围')}：{data.profile.activityAreas.join('、')}</span>{/if}
          </p>
        {/if}
        {#if data.profile.personality}<p class="center-personality">{$t('性格')}：{data.profile.personality}</p>{/if}
        {#if data.profile.contacts?.length}
          <div class="center-contacts">
            <strong>{$t('联系方式')}</strong>
            {#each data.profile.contacts as contact}
              <span>{$t(contactTypes.find((item) => item.value === contact.type)?.label || contact.type)}：{contact.value}</span>
            {/each}
          </div>
        {/if}
        {#if data.profile.member}
          <div class="profile-actions">
            {#if !editing}<button type="button" class="link-button" on:click={startEdit}>{$t('编辑资料')}</button>{/if}
            <a class="text-link" href="/settings">{$t('个人设置')}</a>
            {#if data.profile.profilePublic}
              <a class="text-link" href={`/community/${data.profile.id}`}>{$t('查看公开主页')}</a>
            {/if}
          </div>
        {/if}
      </div>
    </section>

    {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}

    {#if editing}
      <form class="profile-edit" on:submit|preventDefault={saveProfile}>
        <div class="section-head">
          <div><h2>{$t('编辑个人资料')}</h2><p>{$t('修改后会立即更新个人中心和公开主页。')}</p></div>
        </div>
        {#if formError}<p class="message error" role="alert">{$t(formError)}</p>{/if}
        <div class="edit-avatar">
          {#if avatarPreview && !removeAvatar}
            <img class="avatar" src={avatarPreview} alt={$t('新头像预览')} />
          {:else if avatarSrc && !removeAvatar}
            <img class="avatar" src={avatarSrc} alt={$t('当前头像')} />
          {:else}
            <span class="avatar avatar-default" aria-hidden="true"></span>
          {/if}
          <div class="avatar-controls">
            <label class="file-label">{$t('更换头像')}<input type="file" accept="image/jpeg,image/png,image/webp,image/gif" on:change={selectAvatar} /></label>
            {#if avatarSrc || avatarPreview}
              <label class="check-label"><input type="checkbox" bind:checked={removeAvatar} />{$t('删除头像')}</label>
            {/if}
          </div>
        </div>
        <label class="edit-field">{$t('昵称')}<input bind:value={formName} maxlength="80" required /></label>
        <label class="edit-field">{$t('简介')}<textarea bind:value={formBio} maxlength="600" rows="4"></textarea></label>
        <div class="edit-grid">
          <label class="edit-field">{$t('性别')}<select bind:value={formGender}><option value="">{$t('未填写')}</option>{#each genderOptions as [value, label]}<option {value}>{$t(label)}</option>{/each}</select></label>
          <label class="edit-field">{$t('年龄')}<input type="number" min="1" max="120" bind:value={formAge} placeholder={$t('可不填')} /></label>
        </div>
        <label class="edit-field">{$t('性格')}<input bind:value={formPersonality} maxlength="200" placeholder={$t('点击下方标签快速选择，或自由输入')} /></label>
        <div class="personality-presets">
          {#each personalityPresets as preset}
            <button type="button" class:active={isPersonalitySelected(preset)} class="preset-chip" on:click={() => togglePersonality(preset)}>{preset}</button>
          {/each}
        </div>
        <label class="edit-field">{$t('当前状态')}<select bind:value={formCurrentStatus}><option value="">{$t('未设置')}</option><option value="busy">{$t('忙碌')}</option><option value="free">{$t('空闲')}</option></select></label>
        <fieldset class="edit-block">
          <legend>{$t('联系方式（最多 5 项）')}</legend>
          {#each formContacts as contact, index}
            <div class="contact-row">
              <select bind:value={contact.type}>{#each contactTypes as type}<option value={type.value}>{$t(type.label)}</option>{/each}</select>
              <input bind:value={contact.value} maxlength="200" placeholder={$t('填写具体账号或号码')} />
              <button type="button" class="small" on:click={() => removeContact(index)}>{$t('删除')}</button>
            </div>
          {/each}
          {#if formContacts.length < 5}<button type="button" class="link-button" on:click={addContact}>{$t('添加联系方式')}</button>{/if}
        </fieldset>
        <fieldset class="edit-block">
          <legend>{$t('活动范围（最多 2 个）')}</legend>
          {#if formActivityAreas.length}
            <div class="tag-list">
              {#each formActivityAreas as area, index}
                <span class="chip">{area}<button type="button" aria-label={$t('删除')} on:click={() => removeActivityArea(index)}>×</button></span>
              {/each}
            </div>
          {/if}
          {#if formActivityAreas.length < 2}
            <div class="tag-input">
              <input bind:value={activityDraft} list="activity-options" maxlength="40" placeholder={$t('线上 / 上海 / 江浙沪')} />
              <button type="button" class="small" on:click={addActivityArea}>{$t('添加')}</button>
            </div>
            <datalist id="activity-options">{#each activityOptions as option}<option value={option}></option>{/each}</datalist>
          {/if}
        </fieldset>
        <fieldset class="edit-block">
          <legend>{$t('想要展示的职业（最多 3 个）')}</legend>
          <div class="identity-picker">
            {#each data.identityOptions as identity}
              <label class:checked={formIdentityIds.includes(identity.id)}>
                <input type="checkbox" checked={formIdentityIds.includes(identity.id)} on:change={() => toggleIdentity(identity.id)} />
                <span>{identity.name}</span>
              </label>
            {/each}
          </div>
        </fieldset>
        <label class="check-label"><input type="checkbox" bind:checked={formPublic} />{$t('公开我的个人资料')}</label>
        <div class="edit-actions">
          <button class="primary" type="submit" disabled={saving}>{saving ? $t('正在保存…') : $t('保存资料')}</button>
          <button type="button" on:click={cancelEdit} disabled={saving}>{$t('取消')}</button>
        </div>
      </form>
    {/if}

    {#if !data.profile.member}
      <p class="message">{$t('当前为平台超级管理员账号，个人中心不包含会员资料；请使用会员账号查看完整信息。')}</p>
    {/if}

    <div class="stat-grid">
      <article class="stat">
        <p class="stat-title">{$t('完成项目')}</p>
        <span>{completedProjects.length}</span>
        {#if completedDifficulties.length}
          <div class="stat-tags">{#each completedDifficulties as tag}<span class={`stat-tag ${tag.level}`}><span class="tag-level">{tag.level}</span><span class="tag-count">{tag.count}</span></span>{/each}</div>
        {/if}
      </article>
      <article class="stat">
        <p class="stat-title">{$t('职业徽章')}</p>
        <span>{data.badges.length}</span>
        <p class="stat-note">{$t('总等级')} {totalLevel}</p>
      </article>
      <article class="stat">
        <p class="stat-title">{$t('技能展示')}</p>
        <span>{data.skills.length}</span>
        {#if topSkill}<p class="stat-note">{topSkill.name}{#if topSkill.likeCount} ×{topSkill.likeCount}{/if}</p>{/if}
      </article>
      <article class="stat">
        <p class="stat-title">{$t('关注人数')}</p>
        <span>{data.relations?.admireCount || 0}</span>
      </article>
    </div>

    <div class="content-grid center-grid">
      <section class="center-section wide" aria-labelledby="center-projects">
        <div class="section-head">
          <div><h2 id="center-projects">{$t('我的项目')}</h2><p>{$t('按角色分类展示你参与的所有项目。')}</p></div>
        </div>
        <div class="project-categories">
          <nav class="project-tabs" aria-label={$t('我的项目分类')}>
            <button type="button" class:active={projectTab === 'initiated'} on:click={() => projectTab = 'initiated'}>{$t('我发起的项目')}<span>{data.projects.initiated.length}</span></button>
            <button type="button" class:active={projectTab === 'participated'} on:click={() => projectTab = 'participated'}>{$t('我参与的项目')}<span>{data.projects.participated.length}</span></button>
            <button type="button" class:active={projectTab === 'followed'} on:click={() => projectTab = 'followed'}>{$t('我关注的项目')}<span>{data.projects.followed.length}</span></button>
          </nav>
          <div class="status-filters" role="group" aria-label={$t('执行状态')}>
            {#each projectStatusFilters as status}
              <button type="button" class:active={projectStatusFilter === status} aria-pressed={projectStatusFilter === status} on:click={() => projectStatusFilter = projectStatusFilter === status ? '' : status}>{$t(statusLabels[status])}</button>
            {/each}
          </div>
          {#if projectItems(projectTab).length}
            <div class="project-mini-list">
              {#each projectItems(projectTab) as project (project.id)}
                <a class="project-mini-card" href={'/showcase/' + encodeURIComponent(project.id)}>
                  <span class:done={project.status === 'completed'} class="mini-status">{$t(statusLabels[project.status] || project.status)}</span>
                  <strong>{project.title}</strong>
                  {#if project.summary}<p>{project.summary}</p>{/if}
                </a>
              {/each}
            </div>
          {:else}
            <p class="mini-empty">{$t(projectStatusFilter ? '该状态下暂无项目。' : projectTab === 'initiated' ? '还没有发起项目。' : projectTab === 'participated' ? '还没有参与项目。' : '还没有关注项目。')}</p>
          {/if}
        </div>
      </section>

      <section class="center-section" aria-labelledby="center-badges">
        <div class="section-head">
          <div>
            <h2 id="center-badges">{$t('职业徽章')}</h2>
            <p>{$t('按实际职业贡献累计的经验与等级。')}</p>
            <p class="badge-hint">{$t('每 10 级一个徽章，当前为占位样式。')}</p>
          </div>
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
                  <div class="milestones" aria-label={$t('徽章占位')}>
                    {#each milestones(badge) as milestone}
                      <span class:earned={badge.level >= milestone} class="milestone" title={`Lv.${milestone} ${$t('徽章占位')}`}>{milestone}</span>
                    {/each}
                  </div>
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
          <div><h2 id="center-skills">{$t('技能展示')}</h2><p>{$t('技能为本人自述，作品与贡献作为依据。')}</p><p class="skill-hint">{$t('认可由其他成员点赞产生，不能给自己点赞。')}</p></div>
          {#if data.profile.member}<a class="link-button" href="/me/skills">{$t('编辑技能')}</a>{/if}
        </div>
        {#if data.skills.length}
          <ul class="skill-list">
            {#each displaySkills as skill (skill.id)}
              <li class="skill-card">
                <div class="skill-head"><strong>{skill.name}</strong>{#if skill.category}<span class="chip small">{$t(skill.category)}</span>{/if}</div>
                {#if skill.proficiency}<span class="meta">{$t('熟练程度')}：{$t(skill.proficiency)}</span>{/if}
                {#if skill.abilityNote}<p>{skill.abilityNote}</p>{/if}
                {#if skill.images?.length}
                  <div class="skill-images">
                    {#each skill.images as url}
                      <a href={url} target="_blank" rel="noreferrer"><img src={url} alt={skill.name} loading="lazy" /></a>
                    {/each}
                  </div>
                {/if}
                {#if skill.links?.length}
                  <ul class="skill-links">
                    {#each skill.links as link}
                      <li><a href={link} target="_blank" rel="noreferrer">{link}</a></li>
                    {/each}
                  </ul>
                {/if}
                <div class="skill-foot">
                  <span class="like-count"><span aria-hidden="true">♥</span> {skill.likeCount}</span>
                  {#if skill.likeTier > 0}<span class={`recognized tier-${skill.likeTier}`}>{$t(recognizedLabels[skill.likeTier])}</span>{/if}
                </div>
              </li>
            {/each}
          </ul>
        {:else}
          <div class="center-empty"><p>{$t('还没有添加技能')}</p></div>
        {/if}
      </section>

      {#if data.profile.member}
        <section class="center-section wide" aria-labelledby="center-hardware">
        <div class="section-head">
          <div><h2 id="center-hardware">{$t('我的硬件配置')}</h2><p>{$t('展示你日常使用的硬件设备。')}</p></div>
          {#if !hwEditing}<button type="button" class="link-button" on:click={startHwEdit}>{$t('编辑硬件')}</button>{/if}
        </div>
        {#if hwNotice}<p class="message success" role="status">{hwNotice}</p>{/if}
        {#if hwEditing}
          <div class="hw-edit">
            {#if hwError}<p class="message error" role="alert">{hwError}</p>{/if}
            {#each hwRows as row, index}
              <div class="hw-row">
                <select bind:value={row.category}>{#each hwCategories as cat}<option value={cat}>{cat}</option>{/each}</select>
                <input bind:value={row.name} maxlength="200" placeholder={$t('硬件名称')} />
                <input bind:value={row.detail} maxlength="500" placeholder={$t('型号或补充说明')} />
                <button type="button" class="small danger" on:click={() => removeHwRow(index)}>{$t('删除')}</button>
              </div>
            {/each}
            <div class="hw-actions">
              <button type="button" class="secondary" on:click={addHwRow}>{$t('添加硬件')}</button>
              <button type="button" class="primary" on:click={saveHwConfig} disabled={hwSaving}>{hwSaving ? $t('正在保存…') : $t('保存硬件配置')}</button>
              <button type="button" on:click={() => { hwEditing = false; }}>{$t('取消')}</button>
            </div>
          </div>
        {:else if hardwareConfig.length}
          <div class="hw-display">
            {#each hardwareConfig as item (item.id)}
              <div class="hw-item">
                <span class="hw-category">{item.category}</span>
                <strong>{item.name}</strong>
                {#if item.detail}<span class="hw-detail">{item.detail}</span>{/if}
              </div>
            {/each}
          </div>
        {:else}
          <div class="center-empty"><p>{$t('还没有填写硬件配置')}</p></div>
        {/if}
      </section>

      <section class="center-section wide" aria-labelledby="center-invitations">
          <div class="section-head">
            <div><h2 id="center-invitations">{$t('我的邀请码')}</h2><p>{$t('邀请码有效 24 小时，仅可使用一次。')}</p></div>
            <div class="invite-controls">
              <label>{$t('生成数量')}<select bind:value={inviteCount} aria-label={$t('生成数量')}>{#each [1, 2, 3, 4, 5] as count}<option value={count}>{count}</option>{/each}</select></label>
              <label>{$t('使用次数')}<select bind:value={inviteMaxUses} aria-label={$t('使用次数')}><option value={1}>1</option><option value={5}>5</option><option value={10}>10</option><option value={0}>{$t('无限次')}</option></select></label>
              <button type="button" class="link-button" on:click={generateInvitation} disabled={generatingInvitation}>{generatingInvitation ? $t('正在生成…') : $t('生成邀请码')}</button>
            </div>
          </div>
          {#if invitationError}<p class="message error" role="alert">{invitationError}</p>{/if}
          {#if invitationNotice}<p class="message success" role="status">{invitationNotice}</p>{/if}
          {#if invitations.length}
            <div class="invitation-list">
              {#each invitations as invitation (invitation.code)}
                <article class="invitation-card">
                  <div class="invitation-head">
                    <strong>{invitation.code}</strong>
                    {#if invitation.maxUses === 0}<span class="unlimited-tag">{$t('无限次')}</span>{/if}
                    <button type="button" class="small copy-button" on:click={() => copyInvitation(invitation.code)}>{copiedCode === invitation.code ? $t('已复制') : $t('复制')}</button>
                  </div>
                  <div class="invitation-status-line"><span class:used={invitation.used} class="invitation-status">{$t(invitationStatus(invitation))}{#if invitation.maxUses === 0} · {$t('已用')} {invitation.useCount} {$t('次')}{:else if invitation.maxUses > 1} · {$t('已用')} {invitation.useCount}/{invitation.maxUses}{/if}</span></div>
                  <small>{$t('到期时间')}：{invitationDate(invitation.expires)}</small>
                  {#if invitation.used}<small>{$t('使用者')}：{invitation.usedBy || '—'} · {invitationDate(invitation.used)}</small>{/if}
                </article>
              {/each}
            </div>
          {:else}
            <div class="center-empty"><p>{$t('还没有邀请码')}</p></div>
          {/if}
        </section>
      {/if}

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
  .profile-meta { display: flex; flex-wrap: wrap; gap: 6px 14px; margin: 0 0 6px; color: #52695a; font-size: 13px; }
  .center-personality { margin: 0 0 8px; color: #52695a; font-size: 13px; max-width: 70ch; }
  .center-contacts { display: flex; flex-wrap: wrap; gap: 6px 14px; margin: 0 0 10px; color: #52695a; font-size: 13px; }
  .center-contacts strong { color: #2f4534; }
  .current-status { padding: 2px 8px; border-radius: 999px; background: #eef3e8; color: #4d6448; }
  .current-status.busy { background: #fff3df; color: #7a4b12; }
  .personality-presets { display: flex; flex-wrap: wrap; gap: 6px; margin-top: -6px; }
  .project-categories { display: grid; gap: 10px; }
  .project-tabs { display: flex; flex-wrap: wrap; gap: 6px; padding-bottom: 8px; border-bottom: 1px solid #dcded4; }
  .project-tabs button { border: 1px solid transparent; background: none; padding: 7px 12px; color: #52695a; font-size: 13px; cursor: pointer; }
  .project-tabs button:hover { background: #eef3e8; }
  .project-tabs button.active { background: #274732; color: #fff; }
  .project-tabs button span { margin-left: 5px; opacity: 0.72; }
  .status-filters { display: flex; flex-wrap: wrap; gap: 8px; }
  .status-filters button { padding: 4px 12px; border: 1px solid #c8d4c0; border-radius: 999px; background: none; color: #3f5d3a; font-size: 12px; cursor: pointer; }
  .status-filters button:hover { background: #eef3e8; }
  .status-filters button.active { background: #274732; border-color: #274732; color: #fff; }
  .project-mini-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 8px; }
  .project-mini-card { display: grid; gap: 3px; padding: 10px 12px; border: 1px solid #dce3d6; border-radius: 6px; background: #fff; text-decoration: none; color: inherit; transition: border-color .15s, box-shadow .15s; }
  .project-mini-card:hover { border-color: #7c9b6c; box-shadow: 0 2px 8px #1c302210; }
  .project-mini-card strong { font-size: 13px; color: #274732; }
  .project-mini-card p { margin: 0; color: #6f7b6d; font-size: 11px; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
  .mini-status { font-size: 10px; color: #4d6448; }
  .mini-status.done { color: #6e9a61; }
  .mini-empty { color: #8b9587; font-size: 12px; margin: 0; }
  .hw-edit { display: grid; gap: 10px; }
  .hw-row { display: grid; grid-template-columns: 100px 1fr 1fr auto; gap: 8px; align-items: center; }
  .hw-row select, .hw-row input { padding: 7px; border: 1px solid #b9c3b3; background: #fff; font: inherit; font-size: 13px; }
  .hw-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  .hw-display { display: grid; gap: 6px; }
  .hw-item { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; padding: 8px 12px; border: 1px solid #e4e8df; border-radius: 6px; background: #fff; }
  .hw-category { display: inline-block; padding: 1px 8px; border-radius: 999px; background: #eef3e8; color: #4d6448; font-size: 11px; font-weight: 600; min-width: 48px; text-align: center; }
  .hw-item strong { font-size: 13px; color: #274732; }
  .hw-detail { color: #6f7b6d; font-size: 12px; }
  .preset-chip { padding: 4px 12px; border: 1px solid #d6ddd2; border-radius: 999px; background: #fff; color: #52695a; font-size: 12px; cursor: pointer; }
  .preset-chip:hover { border-color: #7c9b6c; background: #eef3e8; }
  .preset-chip.active { border-color: #7c9b6c; background: #eef3e8; color: #274732; font-weight: 600; }
  .chips { display: flex; flex-wrap: wrap; gap: 8px; }
  .chip { display: inline-block; padding: 4px 10px; border: 1px solid #c8d4c0; border-radius: 999px; background: #eef3e8; color: #3f5d3a; font-size: 12px; }
  .chip.small { padding: 2px 8px; font-size: 11px; }
  .profile-actions { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 16px; font-size: 13px; }
  .text-link { color: #345d3d; text-underline-offset: 3px; }
  .link-button { padding: 0; border: 0; background: none; color: #345d3d; font: inherit; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .profile-edit { display: grid; gap: 16px; max-width: 760px; margin: 0 0 26px; padding: 24px; border: 1px solid #dcded4; background: #fafbf7; }
  .profile-edit .section-head { margin-bottom: 0; }
  .profile-edit .message { margin: 0; }
  .edit-avatar { display: flex; flex-wrap: wrap; align-items: center; gap: 18px; }
  .avatar-controls { display: flex; flex-direction: column; gap: 8px; font-size: 13px; }
  .file-label { display: inline-flex; align-items: center; gap: 8px; color: #345d3d; font-size: 13px; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .file-label input { max-width: 220px; }
  .edit-field { display: grid; gap: 6px; max-width: 620px; color: #52695a; font-size: 13px; }
  .edit-field input, .edit-field textarea, .edit-field select { width: 100%; padding: 10px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .edit-field textarea { resize: vertical; }
  .edit-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; max-width: 620px; }
  .edit-block { display: grid; gap: 10px; margin: 0; padding: 14px; border: 1px solid #e0e5da; background: #fff; }
  .edit-block legend { padding: 0 6px; color: #52695a; font-size: 13px; }
  .contact-row { display: grid; grid-template-columns: 120px minmax(0, 1fr) auto; gap: 8px; align-items: center; }
  .contact-row select, .contact-row input { width: 100%; padding: 8px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .tag-list { display: flex; flex-wrap: wrap; gap: 8px; }
  .chip button { margin-left: 6px; padding: 0; border: 0; background: none; color: inherit; cursor: pointer; font-size: 14px; line-height: 1; }
  .tag-input { display: flex; gap: 8px; }
  .tag-input input { flex: 1; padding: 8px; border: 1px solid #b9c3b3; background: #fff; font: inherit; }
  .identity-picker { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 6px; max-height: 260px; overflow: auto; }
  .identity-picker label { display: flex; align-items: center; gap: 6px; padding: 5px 7px; border: 1px solid #dce3d6; background: #fafbf7; font-size: 13px; cursor: pointer; }
  .identity-picker label.checked { border-color: #7c9b6c; background: #eef3e8; }
  .edit-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  .stat-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin: 26px 0; }
  .stat { border: 1px solid #dcded4; background: #fafbf7; padding: 18px 20px; }
  .stat span { display: block; font-size: 30px; font-weight: 600; letter-spacing: -0.03em; }
  .stat p { margin: 4px 0 0; color: #818b7f; font-size: 13px; }
  .stat .stat-title { margin: 0 0 4px; }
  .stat p.stat-note { margin-top: 6px; font-size: 12px; overflow-wrap: anywhere; }
  .stat-tags { display: flex; flex-wrap: nowrap; gap: 12px; margin-top: 8px; }
  .stat-tag { display: inline-flex; align-items: baseline; gap: 2px; }
  .tag-level { font-size: 12px; font-weight: 700; background-size: 200% 100%; -webkit-background-clip: text; background-clip: text; color: transparent; animation: stat-tag-shine 4s linear infinite; }
  .tag-count { font-size: 10px; font-weight: 600; color: #818b7f; }
  .stat-tag.S .tag-level { background-image: linear-gradient(100deg, #7c4dff, #d04dff, #ffb84d, #4dc9ff, #7c4dff); }
  .stat-tag.A .tag-level { background-image: linear-gradient(100deg, #aa771c, #fcf6ba, #b38728, #fbf5b7, #aa771c); }
  .stat-tag.B .tag-level { background-image: linear-gradient(100deg, #9aa2ab, #ffffff, #b8c0c8, #f2f4f6, #9aa2ab); }
  .stat-tag.C .tag-level { background-image: linear-gradient(100deg, #8c4a21, #e8b88a, #b87333, #f0c9a4, #8c4a21); }
  .stat-tag.D .tag-level { background-image: none; color: #cfd4dc; animation: none; }
  @keyframes stat-tag-shine { to { background-position: 200% 0; } }
  @media (prefers-reduced-motion: reduce) { .tag-level { animation: none; } }
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
  .badge-hint { margin: 6px 0 0 !important; font-size: 12px !important; color: #8b9587 !important; }
  .milestones { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
  .milestone { display: grid; width: 18px; height: 18px; place-items: center; border: 1px dashed #b9c3b3; border-radius: 50%; color: #9aa596; font-size: 9px; line-height: 1; }
  .milestone.earned { border-style: solid; border-color: #6f9661; background: #e3eddd; color: #3f5d3a; }
  .progress { height: 6px; margin: 8px 0 4px; background: #e6e9df; }
  .progress span { display: block; height: 100%; background: #6f9661; }
  .skill-card { padding-top: 12px; border-top: 1px solid #e6e9df; }
  .skill-card:first-child { padding-top: 0; border-top: 0; }
  .skill-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
  .skill-card p { margin: 6px 0 0; color: #6f7b6d; font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .skill-images { display: grid; grid-template-columns: repeat(auto-fit, minmax(80px, 1fr)); gap: 8px; margin-top: 10px; }
  .skill-images img { display: block; width: 100%; height: 90px; object-fit: cover; border: 1px solid #dcded4; }
  .skill-links { margin: 8px 0 0; padding-left: 16px; font-size: 12px; }
  .skill-links li { margin: 2px 0; overflow-wrap: anywhere; }
  .skill-links a { color: #345d3d; }
  .skill-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 8px; }
  .skill-hint { margin: 6px 0 0 !important; font-size: 12px !important; color: #8b9587 !important; }
  .like-count { color: #a84358; font-size: 12px; }
  .invitation-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; }
  .invite-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
  .invite-controls label { display: flex; align-items: center; gap: 6px; color: #52695a; font-size: 12px; }
  .invite-controls select { padding: 6px 8px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; }
  .invitation-code { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  .copy-button { padding: 2px 8px; }
  .unlimited-tag { display: inline-block; padding: 1px 8px; border-radius: 999px; background: #fff3df; color: #7a4b12; font-size: 11px; font-weight: 600; }
  .invitation-card { display: grid; gap: 4px; padding: 12px; border: 1px solid #dce3d6; background: #fff; }
  .invitation-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .invitation-status-line { display: flex; align-items: center; }
  .invitation-status { color: #4d6448; font-size: 12px; }
  .invitation-status.used { color: #8b9587; }
  .invitation-card small { color: #6f7b6d; }
  .recognized { display: inline-flex; align-items: center; gap: 4px; padding: 2px 9px; border: 1px solid transparent; border-radius: 999px; font-size: 11px; font-weight: 600; letter-spacing: 0.02em; }
  .recognized::before { content: '✦'; }
  .tier-1 { color: #4d6b3f; background: #e8f0e0; border-color: #c4d8b4; }
  .tier-2 { color: #285a78; background: linear-gradient(135deg, #e8f4fb, #d6e9f7); border-color: #a9cde4; }
  .tier-3 { color: #7a5a12; background: linear-gradient(135deg, #fff7d6, #f2d98b); border-color: #e2c25e; box-shadow: 0 0 0 1px #f6e7b0 inset, 0 2px 6px rgba(180, 140, 20, 0.18); }
  .tier-4 { color: #fff; background: linear-gradient(120deg, #7c4dff, #d04dff, #ffb84d, #7c4dff); background-size: 300% 300%; border-color: #e0c7ff; box-shadow: 0 0 12px rgba(160, 80, 255, 0.45); animation: recognized-shine 6s linear infinite; }
  .tier-5 { color: #fff; background: linear-gradient(120deg, #ff5f6d, #ffc371, #36d1dc, #7c4dff, #ff5f6d); background-size: 300% 300%; border-color: transparent; box-shadow: 0 0 16px rgba(255, 120, 80, 0.6); animation: recognized-shine 5s linear infinite; }
  @keyframes recognized-shine { to { background-position: 300% 50%; } }
  @media (prefers-reduced-motion: reduce) { .tier-4, .tier-5 { animation: none; } }
  @media (max-width: 600px) { .center-grid { grid-template-columns: 1fr; } .edit-grid, .contact-row { grid-template-columns: 1fr; } }
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
