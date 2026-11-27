<script>
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import { t } from './i18n.js';
  import AvatarCrop from './AvatarCrop.svelte';
  import Editors from './Editors.svelte';
  import Select from './Select.svelte';
  import { difficultyRank, levelRank } from './tiers.js';

  export let token;
  const dispatch = createEventDispatcher();

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
  let formAge = '';
  let formBirthday = '';
  let formProfileTitle = '';
  let formCurrentStatus = '';
  let formContacts = [];
  let formActivityAreas = [];
  let formIdentityIds = [];
  let activityScope = '';
  let activityRegion = '';
  let removeAvatar = false;
  let avatarFile = null;
  let avatarThumb = null;
  let cropPending = false;
  let avatarPreview = '';
  let invitations = [];
  let invitationError = '';
  let invitationNotice = '';
  let generatingInvitation = false;
  let inviteMaxUses = 1;
  let copiedCode = '';
  let hardwareConfig = [];
  let hwEditing = false;
  let hwSaving = false;
  let hwError = '';
  let hwNotice = '';
  let hwRows = [];
  let hwEditRow = null;
  let hwEditIndex = -1;
  let hwDialog;
  const hwCategories = ['CPU', 'GPU', '主板', '内存', '存储', '显示器', '键盘', '鼠标', '耳机', '其他'];

  const genderLabels = { male: '男', female: '女', nonbinary: '非二元', private: '不愿透露' };
  const genderOptions = Object.entries(genderLabels);
  const currentStatusLabels = { busy: '忙碌', free: '空闲' };
  const contactTypes = [
    { value: 'email', label: '邮箱' },
    { value: 'wechat', label: '微信' },
    { value: 'qq', label: 'QQ' },
    { value: 'discord', label: 'Discord' },
    { value: 'phone', label: '电话' },
    { value: 'other', label: '其他' },
  ];
  const sponsorIdentityNames = new Set(['资金赞助者', '工具提供者', '材料提供者', '场地提供者', '资源联络者', '采购与后勤协作者']);
  const technicalSupportIdentityNames = new Set(['技术顾问', '测试与体验者']);
  function identityGroup(identity) {
    if (sponsorIdentityNames.has(identity.name)) return '赞助类';
    if (identity.category === 'main' || technicalSupportIdentityNames.has(identity.name)) return '技术类';
    return '管理类';
  }
  $: identityGroups = ['技术类', '赞助类', '管理类'].map((group) => ({
    name: group,
    identities: (data?.identityOptions || []).filter((identity) => identityGroup(identity) === group),
  })).filter((group) => group.identities.length);
  const activityScopes = [
    { value: 'domestic', label: '国内' },
    { value: 'hmt', label: '港澳台' },
    { value: 'overseas', label: '海外' },
  ];
  const activityRegions = {
    domestic: ['北京市', '天津市', '河北省', '山西省', '内蒙古自治区', '辽宁省', '吉林省', '黑龙江省', '上海市', '江苏省', '浙江省', '安徽省', '福建省', '江西省', '山东省', '河南省', '湖北省', '湖南省', '广东省', '广西壮族自治区', '海南省', '重庆市', '四川省', '贵州省', '云南省', '西藏自治区', '陕西省', '甘肃省', '青海省', '宁夏回族自治区', '新疆维吾尔自治区'],
    hmt: ['香港', '澳门', '台湾'],
    overseas: ['亚洲', '欧洲', '北美洲', '南美洲', '非洲', '大洋洲'],
  };
  $: activityRegionOptions = (activityRegions[activityScope] || []).map((region) => ({ value: region, label: region }));
  $: profileTitleOptions = [{ value: '', label: $t('不佩戴头衔') }, ...(data?.profile?.officialTitles || []).map((title) => ({ value: title.id, label: `${title.name}${!title.isOfficial && title.issuedByName ? ` · ${$t('授予者')}：${title.issuedByName}` : ''}` }))];
  $: featuredIdentities = (data?.identities || []).filter((identity) => identity.featured);
  $: completedProjects = [...(data?.projects?.completed || []), ...(data?.projects?.assisted || [])];
  $: difficultyTags = ['S', 'A', 'B', 'C', 'D'].map((level) => ({ level, count: completedProjects.filter((project) => (project.difficulty || '').toUpperCase() === level).length })).filter((tag) => tag.count > 0);
  $: completedDifficulties = difficultyTags;
  let showAllRatings = false;
  function sliceRatingRuns(runs, budget) {
    const out = [];
    let left = budget;
    for (const run of runs) {
      if (left <= 0) break;
      out.push({ level: run.level, text: run.text.slice(0, left) });
      left -= run.text.length;
    }
    return out;
  }
  $: ratingRuns = completedDifficulties.map((tag) => ({ level: tag.level, text: tag.level.repeat(tag.count) }));
  $: ratingLetters = ratingRuns.reduce((sum, run) => sum + run.text.length, 0);
  $: collapsedRatings = !showAllRatings && ratingLetters > 8;
  $: shownRatingRuns = collapsedRatings ? sliceRatingRuns(ratingRuns, 8) : ratingRuns;
  $: totalLevel = (data?.badges || []).reduce((sum, badge) => sum + (badge.level || 0), 0);
  let skillsExpanded = false;

  const rewardLabels = { money: '资金', product: '实物', title: '头衔', credit: '署名', learning: '学习', none: '无', other: '其他' };
  const statusLabels = { not_started: '未开始', ongoing: '已启动', long_term: '长期项目', paused: '暂停中', terminated: '已终止', completed: '已完成' };
  const recognizedLabels = { 1: '有点东西', 2: '他不对劲', 3: '他真会啊', 4: '他太强了！', 5: '他是超人！' };
  const projectStatusFilters = ['not_started', 'ongoing', 'long_term', 'paused', 'completed', 'terminated'];
  let projectTab = 'initiated';
  let projectStatusFilter = '';
  function projectItems(key) {
    const items = data?.projects?.[key] || [];
    return projectStatusFilter ? items.filter((project) => project.status === projectStatusFilter) : items;
  }

  async function loadAvatar(url) {
    if (!url) return '';
    try {
      const response = await fetch(url, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
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

  function medalLevel(level) {
    return Math.floor((level || 0) / 10) * 10;
  }

  function startEdit() {
    if (data?.profile?.canEditProfile === false) return;
    formName = data?.profile?.name || '';
    formBio = data?.profile?.bio || '';
    formPublic = !!data?.profile?.profilePublic;
    formGender = data?.profile?.gender || '';
    formAge = data?.profile?.age || '';
    formBirthday = data?.profile?.birthday || '';
    formProfileTitle = data?.profile?.profileTitleId || '';
    formCurrentStatus = data?.profile?.currentStatus || '';
    formContacts = (data?.profile?.contacts || []).map((contact) => ({ ...contact }));
    formActivityAreas = [...(data?.profile?.activityAreas || [])];
    formIdentityIds = (data?.identities || []).filter((identity) => identity.featured).map((identity) => identity.id);
    activityScope = '';
    activityRegion = '';
    removeAvatar = false;
    avatarFile = null;
    avatarThumb = null;
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
    const region = activityRegion.trim();
    if (!activityScope || !region) return;
    if (formActivityAreas.length >= 2) {
      formError = '最多选择 2 个活动范围。';
      return;
    }
    const value = `${activityScope === 'domestic' ? '国内' : activityScope === 'hmt' ? '港澳台' : '海外'} · ${region}`;
    if (!formActivityAreas.includes(value)) formActivityAreas = [...formActivityAreas, value];
    activityRegion = '';
  }

  function removeActivityArea(index) {
    formActivityAreas = formActivityAreas.filter((_, itemIndex) => itemIndex !== index);
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
    avatarThumb = null;
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatarPreview = '';
  }

  function selectAvatar(event) {
    avatarFile = event.detail.avatar;
    avatarThumb = event.detail.thumb;
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatarPreview = URL.createObjectURL(avatarThumb);
    removeAvatar = false;
  }

  async function saveProfile() {
    if (cropPending) return;
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
      body.set('gender', formGender);
      body.set('age', String(formAge || 0));
      body.set('birthday', formBirthday);
      body.set('profileTitle', formProfileTitle);
      body.set('currentStatus', formCurrentStatus);
      body.set('contacts', JSON.stringify(formContacts.filter((contact) => contact.value.trim())));
      body.set('activityAreas', JSON.stringify(formActivityAreas));
      body.set('identityIds', JSON.stringify(formIdentityIds));
      if (avatarFile && !removeAvatar) { body.set('avatar', avatarFile); body.set('avatarThumb', avatarThumb); }
      if (removeAvatar) body.set('removeAvatar', 'true');
      const response = await fetch('/api/hpahub/me/center/profile', {
        method: 'PATCH',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body,
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || `请求失败（${response.status}）`);
      data = result;
      dispatch('profileSaved', { name: result.profile?.name });
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
        body: JSON.stringify({ count: 1, maxUses: Number(inviteMaxUses) }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.message === 'You can hold at most 10 unused invitations.' ? $t('最多持有 10 个未使用的邀请码。') : body.message || $t('生成失败，请重试。'));
      await loadInvitations();
      invitationNotice = $t('已生成 {count} 个邀请码。', { count: 1 });
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
    hwError = '';
    hwNotice = '';
    hwEditing = true;
  }

  async function editHwRow(index = hwRows.length) {
    if (index >= 50) return;
    hwEditIndex = index;
    hwEditRow = { ...(hwRows[index] || { category: '其他', name: '', detail: '' }) };
    hwError = '';
    await tick();
    hwDialog?.showModal();
  }

  function saveHwRow() {
    if (hwEditRow.category === '其他' && !hwEditRow.name.trim()) {
      hwError = '请填写硬件名称。';
      return;
    }
    const row = { ...hwEditRow, name: hwEditRow.category === '其他' ? hwEditRow.name.trim() : hwEditRow.category };
    hwRows = hwEditIndex === hwRows.length ? [...hwRows, row] : hwRows.map((item, index) => index === hwEditIndex ? row : item);
    hwEditRow = null;
    hwDialog?.close();
  }

  function removeHwRow(index) {
    hwRows = hwRows.filter((_, i) => i !== index);
  }

  async function saveHwConfig() {
    hwSaving = true;
    hwError = '';
    try {
      const items = hwRows.map((r) => ({ ...r, name: r.category === '其他' ? r.name.trim() : r.category })).filter((r) => r.name);
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
        <h1 id="center-name">{data.profile.name || $t('未设置昵称')}</h1>
        {#if data.profile.officialTitles?.length}<div class="profile-meta">{#each data.profile.officialTitles as title}<span class="profile-badge title-badge">{$t(title.isOfficial ? '官方头衔' : '项目头衔')} · {title.name}{#if !title.isOfficial && title.issuedByName} · {$t('授予者')}：{title.issuedByName}{/if}</span>{/each}</div>{/if}
        {#if data.profile.bio}<p class="center-bio">{data.profile.bio}</p>{/if}
        {#if data.profile.currentStatus || data.profile.gender || data.profile.age || data.profile.birthday || data.profile.profileTitle || data.profile.activityAreas?.length}
          <div class="profile-meta" aria-label={$t('个人资料')}>
            {#if data.profile.currentStatus}<span class={`profile-badge status-badge status-${data.profile.currentStatus}`}>{$t('当前状态')} · {$t(currentStatusLabels[data.profile.currentStatus] || data.profile.currentStatus)}</span>{/if}
            {#if data.profile.gender}<span class={`profile-badge gender-badge gender-${data.profile.gender}`}>{$t(genderLabels[data.profile.gender] || data.profile.gender)}</span>{/if}
            {#if data.profile.age}<span class="profile-badge profile-fact">{$t('年龄')} · {data.profile.age}</span>{/if}
            {#if data.profile.birthday}<span class="profile-badge profile-fact">{$t('生日')} · {data.profile.birthday}</span>{/if}
            {#if data.profile.profileTitle}<span class="profile-badge profile-fact">{$t('佩戴头衔')} · {data.profile.profileTitle}</span>{/if}
            {#if data.profile.activityAreas?.length}<span class="profile-badge profile-fact">{$t('活动范围')} · {data.profile.activityAreas.join('、')}</span>{/if}
          </div>
        {/if}
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
            {#if !editing}
              {#if data.profile.canEditProfile}<button type="button" class="link-button" on:click={startEdit}>{$t('编辑资料')}</button>
              {:else}<span class="profile-edit-lock">{$t('个人资料每7天只能修改一次')} · {new Date(data.profile.nextProfileEditAt).toLocaleString()}</span>{/if}
            {/if}
            <a class="text-link" href="/settings">{$t('修改个人名片')}</a>
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
          <div><h2>{$t('编辑个人资料')}</h2><p>{$t('每7天可以修改一次个人资料。')}</p></div>
        </div>
        {#if formError}<p class="message error" role="alert">{$t(formError)}</p>{/if}
        <div class="edit-profile-top">
          <div class="edit-avatar">
            {#if avatarPreview && !removeAvatar}
              <img class="avatar" src={avatarPreview} alt={$t('新头像预览')} />
            {:else if avatarSrc && !removeAvatar}
              <img class="avatar" src={avatarSrc} alt={$t('当前头像')} />
            {:else}
              <span class="avatar avatar-default" aria-hidden="true"></span>
            {/if}
            <div class="avatar-controls">
              <AvatarCrop on:crop={selectAvatar} on:pending={(event) => cropPending = event.detail} />
              {#if avatarSrc || avatarPreview}
                <label class="check-label"><input type="checkbox" bind:checked={removeAvatar} />{$t('删除头像')}</label>
              {/if}
            </div>
          </div>
          <label class="edit-field">{$t('昵称')}<input bind:value={formName} maxlength="80" required /></label>
        </div>
        <label class="edit-field">{$t('简介')}<textarea bind:value={formBio} maxlength="600" rows="4"></textarea></label>
        <div class="edit-grid">
          <div class="edit-field"><span>{$t('性别')}</span><Select bind:value={formGender} ariaLabel={$t('性别')} options={[{ value: '', label: $t('未填写') }, ...genderOptions.map(([value, label]) => ({ value, label: $t(label) }))]} /></div>
          <label class="edit-field">{$t('年龄')}<input type="number" min="1" max="120" bind:value={formAge} placeholder={$t('可不填')} /></label>
        </div>
        <div class="edit-grid">
          <label class="edit-field">{$t('生日')}<input type="date" bind:value={formBirthday} /></label>
          <div class="edit-field"><span>{$t('佩戴头衔')}</span><Select bind:value={formProfileTitle} ariaLabel={$t('佩戴头衔')} options={profileTitleOptions} /><small>{$t('只能选择已获得的头衔；新头衔由管理员授予。')}</small></div>
        </div>
        <div class="edit-field"><span>{$t('当前状态')}</span><Select bind:value={formCurrentStatus} ariaLabel={$t('当前状态')} options={[{ value: '', label: $t('未设置') }, { value: 'busy', label: $t('忙碌') }, { value: 'free', label: $t('空闲') }]} /></div>
        <fieldset class="edit-block">
          <legend>{$t('联系方式（最多 5 项）')}</legend>
          {#each formContacts as contact, index}
            <div class="contact-row">
              <Select bind:value={contact.type} ariaLabel={`${$t('联系方式')} ${index + 1}`} options={contactTypes.map((type) => ({ value: type.value, label: $t(type.label) }))} />
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
              <Select bind:value={activityScope} ariaLabel={$t('活动地区类别')} options={[{ value: '', label: $t('选择地区类别') }, ...activityScopes.map((scope) => ({ value: scope.value, label: $t(scope.label) }))]} on:value={() => activityRegion = ''} />
              <Select bind:value={activityRegion} ariaLabel={$t('活动地区')} options={[{ value: '', label: $t('选择具体地区') }, ...activityRegionOptions]} disabled={!activityScope} />
              <button type="button" class="small" disabled={!activityScope || !activityRegion} on:click={addActivityArea}>{$t('添加')}</button>
            </div>
          {/if}
          <p class="activity-area-hint">{$t('国内按省级地区选择，港澳台按地区选择，海外按大洲选择。')}</p>
        </fieldset>
        <fieldset class="edit-block">
          <legend>{$t('想要展示的职业（最多 3 个）')}</legend>
          {#each identityGroups as group}
            <div class="identity-group">
              <h4>{$t(group.name)}</h4>
              <div class="identity-picker">
                {#each group.identities as identity}
                  <label class:checked={formIdentityIds.includes(identity.id)}>
                    <input type="checkbox" checked={formIdentityIds.includes(identity.id)} on:change={() => toggleIdentity(identity.id)} />
                    <span>{identity.name}</span>
                  </label>
                {/each}
              </div>
            </div>
          {/each}
        </fieldset>
        <div class="edit-actions">
          <button class="primary" type="submit" disabled={saving || cropPending}>{saving ? $t('正在保存…') : $t('保存资料')}</button>
          <button type="button" on:click={cancelEdit} disabled={saving}>{$t('取消')}</button>
        </div>
      </form>
      <Editors {token} cardsOnly={true} />
    {/if}

    {#if !data.profile.member}
      <p class="message">{$t('当前为平台超级管理员账号，个人中心不包含会员资料；请使用会员账号查看完整信息。')}</p>
    {/if}

    <div class="stat-grid">
      <article class="stat">
        <p class="stat-title">{$t('完成项目')}</p>
        <span>{completedProjects.length}</span>
        {#if completedDifficulties.length}
          <div class="stat-tags">
            {#each shownRatingRuns as run}<span class="stat-tag"><span class={`tag-level rank rank-text rank-${difficultyRank(run.level)}`}>{run.text}</span></span>{/each}
            {#if collapsedRatings}<button type="button" class="show-all" on:click={() => showAllRatings = true}>{$t('显示全部')}</button>{/if}
          </div>
        {/if}
      </article>
      <article class="stat">
        <p class="stat-title">{$t('职业徽章')}</p>
        <span>{data.badges.length}</span>
        <p class="stat-note">{$t('总等级')} {totalLevel}</p>
        {#if featuredIdentities.length}
          <div class="stat-roles">
            {#each featuredIdentities as identity}
              <span class={`rank rank-pill rank-${identity.category === 'main' ? 4 : 3}`} title={$t(identity.category === 'main' ? '主力职业' : '辅助职业')}>{identity.name}</span>
            {/each}
          </div>
        {/if}
      </article>
      <article class="stat">
        <p class="stat-title">{$t('技能展示')}</p>
        <span>{data.skills.length}</span>
        {#if data.skills.length}
          <div class="stat-skills">
            {#each skillsExpanded ? data.skills : data.skills.slice(0, 3) as skill (skill.id)}
              <span class="chip small">{skill.name}</span>
            {/each}
          </div>
          {#if data.skills.length > 3}
            <button type="button" class="show-all" on:click={() => skillsExpanded = !skillsExpanded}>{skillsExpanded ? $t('收起') : $t('查看全部')}</button>
          {/if}
        {/if}
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
                  {#if project.projectNumber}<small class="project-number">{project.projectNumber}</small>{/if}
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
              <li class="badge-card career-badge">
                <div class="badge-icon" aria-hidden="true">✦</div>
                <div class="badge-body">
                  <strong>{badge.careerName}</strong>
                  <p><span class={`rank rank-text rank-${levelRank(badge.level)}`}>{badge.levelTitle || `Lv.${badge.level}`}</span> · {$t('累计经验')} {badge.totalXp}</p>
                  {#if badge.nextLevelXp}
                    <div class="progress" role="img" aria-label={`${badge.totalXp} / ${badge.nextLevelXp} XP`}><span style={`width:${badgeProgress(badge)}%`}></span></div>
                    <small>{badge.totalXp} / {badge.nextLevelXp} XP · {$t('下一等级')} {badge.nextLevelTitle}</small>
                  {:else}
                    <small>{$t('已达到当前最高等级')}</small>
                  {/if}
                  {#if medalLevel(badge.level) >= 10}
                    <div class="milestones" aria-label={$t('徽章占位')}>
                      <span class={`rank rank-disc rank-${levelRank(badge.level)}`} title={`Lv.${badge.level} ${$t('徽章占位')}`}><span class="flash-layer" aria-hidden="true"></span>{medalLevel(badge.level)}</span>
                    </div>
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
          <div><h2 id="center-skills">{$t('技能展示')}</h2><p>{$t('技能为本人自述，作品与贡献作为依据。')}</p><p class="skill-hint">{$t('认可由其他成员点赞产生，不能给自己点赞。')}</p></div>
          {#if data.profile.member}<a class="link-button" href="/me/skills">{$t('编辑技能')}</a>{/if}
        </div>
        {#if data.skills.length}
          <ul class="skill-list">
            {#each data.skills as skill (skill.id)}
              <li class="skill-card">
                <div class="skill-head"><strong>{skill.name}</strong>{#if skill.category}<span class="chip small">{$t(skill.category)}</span>{/if}{#if skill.featured}<span class="chip small">{$t('公开展示')}</span>{/if}</div>
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
                  <span class="like-count"><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 10v12" /><path d="M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h2.76a2 2 0 0 0 1.79-1.11L12 2a3.13 3.13 0 0 1 3 3.88Z" /></svg> {skill.likeCount}</span>
                  {#if skill.likeTier > 0}<span class={`recognized rank rank-pill rank-${skill.likeTier}`}><span class="flash-layer" aria-hidden="true"></span>{$t(recognizedLabels[skill.likeTier])}</span>{/if}
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
              <div class="hw-item">
                <span class="hw-category">{row.category}</span><strong>{row.name}</strong>{#if row.detail}<span class="hw-detail">{row.detail}</span>{/if}
                <div class="hw-row-actions"><button type="button" class="small" on:click={() => editHwRow(index)}>{$t('编辑')}</button><button type="button" class="small danger" on:click={() => removeHwRow(index)}>{$t('删除')}</button></div>
              </div>
            {/each}
            <div class="hw-actions">
              <button type="button" class="secondary" on:click={() => editHwRow()} disabled={hwRows.length >= 50}>{$t('添加硬件')}</button>
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

      {#if hwEditRow}
        <dialog class="hw-dialog" bind:this={hwDialog} on:cancel|preventDefault={() => { hwEditRow = null; hwDialog?.close(); }}>
          <form class="hw-dialog-form" on:submit|preventDefault={saveHwRow}>
            <div class="dialog-heading"><h2>{hwEditIndex === hwRows.length ? $t('添加硬件') : $t('编辑硬件')}</h2><button type="button" class="dialog-close" aria-label={$t('关闭')} on:click={() => { hwEditRow = null; hwDialog?.close(); }}>×</button></div>
            <div class="hw-dialog-field"><span>{$t('硬件类别')}</span><Select bind:value={hwEditRow.category} ariaLabel={$t('硬件类别')} options={hwCategories.map((category) => ({ value: category, label: category }))} /></div>
            {#if hwEditRow.category === '其他'}<label>{$t('硬件名称')}<input bind:value={hwEditRow.name} maxlength="200" required /></label>{/if}
            <label>{$t('型号或补充说明')}<input bind:value={hwEditRow.detail} maxlength="500" /></label>
            {#if hwError}<p class="message error" role="alert">{$t(hwError)}</p>{/if}
            <div class="hw-actions"><button type="submit" class="primary">{$t('保存')}</button><button type="button" class="secondary" on:click={() => { hwEditRow = null; hwDialog?.close(); }}>{$t('取消')}</button></div>
          </form>
        </dialog>
      {/if}

      <section class="center-section wide" aria-labelledby="center-invitations">
          <div class="section-head">
            <div><h2 id="center-invitations">{$t('我的邀请码')}</h2><p>{$t('邀请码有效 24 小时，最多持有 10 个未使用的邀请码。')}</p></div>
            <div class="invite-controls">
              <div class="invite-count-select"><span>{$t('使用次数')}</span><Select bind:value={inviteMaxUses} ariaLabel={$t('使用次数')} options={[1, 5, 10].map((value) => ({ value, label: String(value) }))} /></div>
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
                    <button type="button" class="small copy-button" on:click={() => copyInvitation(invitation.code)}>{copiedCode === invitation.code ? $t('已复制') : $t('复制')}</button>
                  </div>
                  <div class="invitation-status-line"><span class:used={invitation.used} class="invitation-status">{$t(invitationStatus(invitation))}{#if invitation.maxUses > 1} · {$t('已用')} {invitation.useCount}/{invitation.maxUses}{/if}</span></div>
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
          </div>
        {/if}
      </section>
    </div>
  {/if}
</main>

<style>
  .personal-center { width: min(1200px, 88%); }
  .center-profile { align-items: flex-start; border-bottom: 1px solid var(--border-subtle); padding-bottom: 28px; }
  .center-profile h1 { margin: 0 0 6px; font-size: clamp(28px, 3vw, 40px); line-height: 1.2; letter-spacing: -0.035em; font-weight: 700; }
  .center-bio { margin: 0 0 14px; max-width: 70ch; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--ink-secondary); }
  .profile-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 7px; margin: 0 0 10px; font-size: 12px; }
  .center-contacts { display: flex; flex-wrap: wrap; gap: 6px 14px; margin: 0 0 10px; color: var(--ink-muted); font-size: 13px; }
  .center-contacts strong { color: var(--accent-deep); }
  .project-categories { display: grid; gap: 10px; }
  .project-tabs { display: flex; flex-wrap: wrap; gap: 6px; padding-bottom: 10px; border-bottom: 1px solid var(--border-subtle); }
  .project-tabs button { border: 1px solid transparent; border-radius: var(--radius-sm); background: none; padding: 7px 12px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), color var(--t-fast) var(--ease-out); }
  .project-tabs button:hover { background: var(--surface-tint); color: var(--accent-deep); }
  .project-tabs button.active { background: linear-gradient(135deg, var(--accent), var(--accent-deep)); color: #fff; box-shadow: var(--shadow-glow); }
  .project-tabs button span { margin-left: 5px; opacity: 0.78; font-family: var(--font-mono); font-size: 11.5px; }
  .status-filters { display: flex; flex-wrap: wrap; gap: 8px; }
  .status-filters button { padding: 4px 12px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-elevated); color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out); }
  .status-filters button:hover { background: var(--surface-tint); border-color: var(--border-strong); }
  .status-filters button.active { background: var(--accent); border-color: var(--accent); color: #fff; }
  .project-mini-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 10px; }
  .project-mini-card { display: grid; gap: 4px; padding: 12px 14px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-elevated); text-decoration: none; color: inherit; transition: transform var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .project-mini-card:hover { transform: translateY(-2px); border-color: var(--border-strong); box-shadow: var(--shadow-xs); }
  .project-mini-card strong { font-size: 13px; color: var(--accent-deep); }
  .mini-status { font-size: 10.5px; color: var(--ink-muted); font-weight: 500; }
  .mini-status.done { color: var(--accent); }
  .mini-empty { color: var(--ink-subtle); font-size: 12px; margin: 0; }
  .hw-edit { display: grid; gap: 10px; }
  .hw-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  .hw-display { display: grid; gap: 8px; }
  .hw-item { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; padding: 10px 12px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-elevated); }
  .hw-category { display: inline-block; padding: 2px 9px; border-radius: 999px; background: var(--surface-tint); color: var(--accent-deep); font-size: 11px; font-weight: 600; min-width: 48px; text-align: center; }
  .hw-item strong { font-size: 13px; color: var(--ink-primary); }
  .hw-detail { color: var(--ink-muted); font-size: 12px; }
  .preset-chip { padding: 4px 12px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-elevated); color: var(--ink-secondary); font-size: 12px; cursor: pointer; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out); }
  .preset-chip:hover { border-color: var(--border-strong); background: var(--surface-tint); }
  .preset-chip.active { border-color: var(--accent); background: var(--surface-tint); color: var(--accent-deep); font-weight: 600; }
  .chips { display: flex; flex-wrap: wrap; gap: 8px; }
  .chip { display: inline-block; padding: 4px 10px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-tint); color: var(--accent-deep); font-size: 12px; font-weight: 500; }
  .chip.small { padding: 2px 8px; font-size: 11px; }
  .profile-actions { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 16px; font-size: 13px; }
  .profile-edit-lock { color: var(--ink-subtle); font-size: 13px; }
  .text-link { color: var(--accent-deep); text-underline-offset: 3px; font-weight: 600; }
  .link-button { padding: 0; border: 0; background: none; color: var(--accent-deep); font: inherit; font-weight: 600; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .profile-edit { display: grid; gap: 16px; max-width: 760px; margin: 0 0 26px; padding: 24px; border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-sm); }
  .profile-edit .section-head { margin-bottom: 0; }
  .profile-edit .message { margin: 0; }
  .edit-avatar { display: flex; flex-wrap: wrap; align-items: center; gap: 18px; }
  .edit-profile-top { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 18px; max-width: 620px; }
  .avatar-controls { display: flex; flex-direction: column; gap: 8px; font-size: 13px; }
  .file-label { display: inline-flex; align-items: center; gap: 8px; color: var(--accent-deep); font-size: 13px; font-weight: 600; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .edit-field { display: grid; gap: 6px; max-width: 620px; color: var(--ink-secondary); font-size: 13px; font-weight: 500; }
  .edit-field input, .edit-field textarea { width: 100%; padding: 10px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; transition: border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .edit-field input:focus, .edit-field textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  .edit-field textarea { resize: vertical; }
  .edit-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; max-width: 620px; }
  .edit-block { display: grid; gap: 10px; margin: 0; padding: 14px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-elevated); }
  .edit-block legend { padding: 0 6px; color: var(--ink-secondary); font-size: 13px; font-weight: 600; }
  .contact-row { display: grid; grid-template-columns: 120px minmax(0, 1fr) auto; gap: 8px; align-items: center; }
  .contact-row input { width: 100%; padding: 8px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); font: inherit; }
  .tag-list { display: flex; flex-wrap: wrap; gap: 8px; }
  .chip button { margin-left: 6px; padding: 0; border: 0; background: none; color: inherit; cursor: pointer; font-size: 14px; line-height: 1; }
  .tag-input { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: 8px; }
  .identity-picker { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 6px; max-height: 260px; overflow: auto; }
  .identity-group { display: grid; gap: 8px; }
  .identity-group h4 { margin: 0; color: var(--ink-subtle); font-size: 12px; font-weight: 600; }
  .identity-picker label { display: flex; align-items: center; gap: 6px; padding: 6px 8px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-soft); font-size: 13px; cursor: pointer; }
  .identity-picker label.checked { border-color: var(--accent); background: var(--surface-tint); color: var(--accent-deep); font-weight: 500; }
  .edit-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  .stat-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin: 26px 0; }
  .stat { border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); padding: 18px 20px; box-shadow: var(--shadow-xs); transition: transform var(--t-med) var(--ease-out), border-color var(--t-med) var(--ease-out); }
  .stat:hover { transform: translateY(-2px); border-color: var(--border-strong); }
  .stat span { display: block; font-size: 30px; font-weight: 700; letter-spacing: -0.03em; font-family: var(--font-mono); color: var(--accent-deep); }
  .stat p { margin: 4px 0 0; color: var(--ink-muted); font-size: 13px; }
  .stat .stat-title { margin: 0 0 4px; }
  .stat p.stat-note { margin-top: 6px; font-size: 12px; overflow-wrap: anywhere; }
  .stat-roles { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
  .stat-roles .rank-pill { font-size: 11px; padding: 2px 7px; }
  .stat-skills { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
  .stat-tags { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; align-items: baseline; }
  .stat-tag { display: inline-flex; align-items: baseline; position: relative; }
  .tag-level { display: inline-block; background-size: 220% 100%; animation: rating-flow 3.4s linear infinite; }
  .stat .tag-level { color: transparent; }
  .tag-level.rank-5 { filter: brightness(1.25) drop-shadow(0 0 3px #78a0ffcc) drop-shadow(0 0 10px #62ff9b66); }
  .tag-level.rank-4 { filter: brightness(2.4) drop-shadow(0 0 4px #ffd76a99); }
  .tag-level.rank-3 { filter: brightness(2.2) drop-shadow(0 0 3px #cfd8e080); }
  .tag-level.rank-2 { filter: brightness(2.2) drop-shadow(0 0 3px #d59a6a80); }
  .tag-level.rank-1 { filter: brightness(2.0) drop-shadow(0 0 3px #c9ced380); }
  @keyframes rating-flow {
    from { background-position: 0% 50%; }
    to { background-position: -220% 50%; }
  }
  @media (prefers-reduced-motion: reduce) {
    .tag-level { animation: none; }
  }
  .show-all { border: 0; background: none; padding: 0; color: var(--ink-subtle); font-size: 11px; text-decoration: underline; cursor: pointer; white-space: nowrap; }
  .center-grid { grid-template-columns: minmax(0, 1fr); align-items: start; }
  .center-section { min-width: 0; padding: 24px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-sm); }
  .center-section .section-head { margin-bottom: 16px; }
  .center-section h2 { margin: 0; font-size: 18px; }
  .center-list, .badge-list, .skill-list { display: grid; gap: 12px; margin: 0; padding: 0; list-style: none; }
  .meta { color: var(--ink-subtle); font-size: 12px; }
  .status-pill { flex: none; height: fit-content; padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 999px; background: var(--surface-soft); color: var(--ink-secondary); font-size: 12px; white-space: nowrap; }
  .status-pill.done { background: var(--surface-tint); border-color: var(--border-strong); color: var(--accent-deep); }
  .badge-card { display: flex; gap: 14px; padding-top: 12px; border-top: 1px solid var(--border-subtle); }
  .badge-card:first-child { padding-top: 0; border-top: 0; }
  .career-badge .rank { animation: none; background-image: none; color: var(--accent-deep); filter: none; }
  .badge-icon { display: grid; flex: none; width: 42px; height: 42px; place-items: center; border-radius: 50%; background: var(--surface-tint); border: 1px solid var(--border-default); color: var(--accent-deep); }
  .badge-body { flex: 1; min-width: 0; }
  .badge-body p { margin: 2px 0 6px; color: var(--ink-muted); font-size: 13px; }
  .badge-body small { color: var(--ink-subtle); font-size: 12px; }
  .badge-hint { margin: 6px 0 0 !important; font-size: 12px !important; color: var(--ink-subtle) !important; }
  .milestones { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
  .progress { height: 7px; margin: 8px 0 4px; border-radius: 999px; overflow: hidden; background: var(--surface-tint); }
  .progress span { display: block; height: 100%; border-radius: 999px; background: linear-gradient(90deg, var(--accent), var(--accent-bright)); }
  .skill-card { padding-top: 12px; border-top: 1px solid var(--border-subtle); }
  .skill-card:first-child { padding-top: 0; border-top: 0; }
  .skill-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
  .skill-card p { margin: 6px 0 0; color: var(--ink-muted); font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .skill-images { display: grid; grid-template-columns: repeat(auto-fit, minmax(80px, 1fr)); gap: 8px; margin-top: 10px; }
  .skill-images img { display: block; width: 100%; height: 90px; object-fit: cover; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); }
  .skill-links { margin: 8px 0 0; padding-left: 16px; font-size: 12px; }
  .skill-links li { margin: 2px 0; overflow-wrap: anywhere; }
  .skill-links a { color: var(--accent-deep); }
  .skill-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 8px; }
  .skill-hint { margin: 6px 0 0 !important; font-size: 12px !important; color: var(--ink-subtle) !important; }
  .like-count { display: inline-flex; align-items: center; gap: 4px; color: var(--ink-muted); font-size: 12px; }
  .like-count svg { flex: none; }
  .invitation-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; }
  .invite-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
  .invitation-code { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-family: var(--font-mono); }
  .copy-button { padding: 3px 9px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--accent-deep); font-size: 12px; }
  .copy-button:hover { background: var(--surface-tint); }
  .unlimited-tag { display: inline-block; padding: 1px 8px; border-radius: 999px; background: var(--warn-bg); color: var(--warn-ink); font-size: 11px; font-weight: 600; }
  .invitation-card { display: grid; gap: 4px; padding: 12px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-elevated); }
  .invitation-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .invitation-status-line { display: flex; align-items: center; }
  .invitation-status { color: var(--accent-deep); font-size: 12px; font-weight: 500; }
  .invitation-status.used { color: var(--ink-subtle); }
  .invitation-card small { color: var(--ink-muted); }
  .recognized { display: inline-flex; align-items: center; gap: 4px; padding: 2px 9px; border: 1px solid transparent; border-radius: 999px; font-size: 11px; font-weight: 600; letter-spacing: 0.02em; }
  .recognized::before { content: '✦'; }
  @media (max-width: 600px) { .edit-profile-top, .edit-grid, .contact-row, .tag-input { grid-template-columns: 1fr; } }
  .reward-note { margin: 12px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
  .center-empty { color: var(--ink-subtle); font-size: 13px; }
  .center-empty p { margin: 0 0 6px; }
  .wide { grid-column: 1 / -1; }
  @media (max-width: 900px) {
    .stat-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
  @media (max-width: 800px) {
    .center-grid { grid-template-columns: 1fr; }
    .center-profile { flex-direction: column; gap: 18px; }
  }
</style>
