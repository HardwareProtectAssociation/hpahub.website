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
  let removeAvatar = false;
  let avatarFile = null;
  let avatarPreview = '';
  let skillsEditing = false;
  let skillsSaving = false;
  let skillError = '';
  let skillRows = [];

  const proficiencyOptions = ['初学者', '略懂一二', '比较熟练', '非常熟练', '你可以永远信任我', '我是这个领域的神！'];

  const rewardLabels = { money: '资金', product: '实物', title: '头衔', credit: '署名', learning: '学习', none: '无', other: '其他' };
  const statusLabels = { ongoing: '进行中', paused: '暂停', terminated: '已中止', completed: '已完成' };

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
    if (skillsEditing) cancelSkillEdit();
    formName = data?.profile?.name || '';
    formBio = data?.profile?.bio || '';
    formPublic = !!data?.profile?.profilePublic;
    removeAvatar = false;
    avatarFile = null;
    formError = '';
    notice = '';
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    avatarPreview = '';
    editing = true;
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

  function startSkillEdit() {
    if (editing) cancelEdit();
    skillRows = (data?.skills || []).map((skill) => ({
      name: skill.name,
      proficiency: proficiencyOptions.includes(skill.proficiency) ? skill.proficiency : '初学者',
      abilityNote: skill.abilityNote || '',
    }));
    skillError = '';
    notice = '';
    skillsEditing = true;
  }

  function cancelSkillEdit() {
    skillsEditing = false;
    skillsSaving = false;
    skillError = '';
    skillRows = [];
  }

  function addSkillRow() {
    skillRows = [...skillRows, { name: '', proficiency: '初学者', abilityNote: '' }];
  }

  function removeSkillRow(index) {
    skillRows = skillRows.filter((_, itemIndex) => itemIndex !== index);
  }

  async function saveSkills() {
    const names = new Set();
    for (const row of skillRows) {
      const name = row.name.trim();
      if (!name) {
        skillError = '请填写技能名称。';
        return;
      }
      if (names.has(name.toLowerCase())) {
        skillError = '技能名称重复。';
        return;
      }
      names.add(name.toLowerCase());
    }
    skillsSaving = true;
    skillError = '';
    notice = '';
    try {
      const response = await fetch('/api/hpahub/me/center/skills', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({
          skills: skillRows.map((row) => ({ name: row.name.trim(), proficiency: row.proficiency, abilityNote: row.abilityNote })),
        }),
      });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.message || `请求失败（${response.status}）`);
      data = result;
      skillsEditing = false;
      skillRows = [];
      notice = '技能已保存。';
    } catch (err) {
      skillError = err.message || String(err);
    } finally {
      skillsSaving = false;
    }
  }

  onMount(load);
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
        {#if data.identities.length}
          <div class="chips">
            {#each data.identities as identity}
              <span class="chip">{$t(identity.category === 'main' ? '主力职业' : '辅助职业')} · {identity.name}</span>
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

      <section class="center-section" class:wide={skillsEditing} aria-labelledby="center-skills">
        <div class="section-head">
          <div><h2 id="center-skills">{$t('技能展示')}</h2><p>{$t('技能为本人自述，作品与贡献作为依据。')}</p></div>
          {#if !skillsEditing}<button type="button" class="link-button" on:click={startSkillEdit}>{$t('编辑技能')}</button>{/if}
        </div>
        {#if skillsEditing}
          <div class="skill-edit">
            {#if skillError}<p class="message error" role="alert">{$t(skillError)}</p>{/if}
            {#each skillRows as row, index}
              <div class="skill-row">
                <label>{$t('技能名称')}<input bind:value={row.name} maxlength="80" placeholder={$t('输入技能名称')} /></label>
                <label>{$t('熟练程度')}<select bind:value={row.proficiency}>{#each proficiencyOptions as option}<option value={option}>{$t(option)}</option>{/each}</select></label>
                <label class="skill-note">{$t('能力说明')}<textarea bind:value={row.abilityNote} maxlength="500" rows="2" placeholder={$t('简单说明你的经验或作品')}></textarea></label>
                <button type="button" class="small danger" on:click={() => removeSkillRow(index)}>{$t('删除')}</button>
              </div>
            {/each}
            {#if !skillRows.length}<p class="center-empty">{$t('还没有添加技能')}</p>{/if}
            <div class="skill-edit-actions">
              <button type="button" on:click={addSkillRow}>{$t('新增技能')}</button>
              <button class="primary" type="button" on:click={saveSkills} disabled={skillsSaving}>{skillsSaving ? $t('正在保存…') : $t('保存技能')}</button>
              <button type="button" on:click={cancelSkillEdit} disabled={skillsSaving}>{$t('取消')}</button>
            </div>
          </div>
        {:else if data.skills.length}
          <ul class="skill-list">
            {#each data.skills as skill (skill.id)}
              <li class="skill-card">
                <div class="skill-head"><strong>{skill.name}</strong>{#if skill.category}<span class="chip small">{$t(skill.category)}</span>{/if}</div>
                {#if skill.proficiency}<span class="meta">{$t('熟练程度')}：{$t(skill.proficiency)}</span>{/if}
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
  .link-button { padding: 0; border: 0; background: none; color: #345d3d; font: inherit; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .profile-edit { display: grid; gap: 16px; max-width: 760px; margin: 0 0 26px; padding: 24px; border: 1px solid #dcded4; background: #fafbf7; }
  .profile-edit .section-head { margin-bottom: 0; }
  .profile-edit .message { margin: 0; }
  .edit-avatar { display: flex; flex-wrap: wrap; align-items: center; gap: 18px; }
  .avatar-controls { display: flex; flex-direction: column; gap: 8px; font-size: 13px; }
  .file-label { display: inline-flex; align-items: center; gap: 8px; color: #345d3d; font-size: 13px; text-decoration: underline; text-underline-offset: 3px; cursor: pointer; }
  .file-label input { max-width: 220px; }
  .edit-field { display: grid; gap: 6px; max-width: 620px; color: #52695a; font-size: 13px; }
  .edit-field input, .edit-field textarea { width: 100%; padding: 10px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .edit-field textarea { resize: vertical; }
  .edit-actions { display: flex; flex-wrap: wrap; gap: 10px; }
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
  .skill-edit { display: grid; gap: 12px; }
  .skill-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; padding: 12px; border: 1px solid #e6e9df; background: #fff; }
  .skill-row label { display: grid; gap: 4px; color: #52695a; font-size: 12px; }
  .skill-row input, .skill-row select, .skill-row textarea { width: 100%; padding: 8px; border: 1px solid #b9c3b3; background: #fff; color: #252b26; font: inherit; }
  .skill-row .skill-note { grid-column: 1 / -1; }
  .skill-row button { grid-column: 1 / -1; justify-self: start; }
  .skill-edit-actions { display: flex; flex-wrap: wrap; gap: 10px; }
  @media (max-width: 600px) { .skill-row { grid-template-columns: 1fr; } }
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
