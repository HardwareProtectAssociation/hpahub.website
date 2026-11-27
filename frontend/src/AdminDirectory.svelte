<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  import { difficultyRank } from './tiers.js';

  export let token;

  let members = [];
  let applications = [];
  let membersMore = false;
  let applicationsMore = false;
  let moreBusy = false;
  let projects = [];
  let loading = true;
  let error = '';
  let notice = '';
  let applicationBusy = '';

  const roleLabels = { admin: '管理员', member: '成员' };
  const statusLabels = { active: '正常', suspended: '已暂停' };
  const projectStatusLabels = { not_started: '未开始', ongoing: '已启动', long_term: '长期项目', paused: '暂停中', terminated: '已中止', completed: '已完成' };
  const reviewLabels = { pending: '待审核', approved: '已通过', rejected: '已拒绝' };
  const sourceLabels = { invitation: '邀请注册', seed: '种子账号' };

  function formatDate(value) {
    if (!value) return '—';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString();
  }

  async function request(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: { Authorization: `Bearer ${token}`, ...(options.headers || {}) },
      cache: 'no-store',
    });
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new Error(body.message || `请求失败（${response.status}）`);
    }
    return response.status === 204 ? null : response.json();
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [memberData, applicationData, projectData] = await Promise.all([
        request('/api/hpahub/manage/members'),
        request('/api/hpahub/manage/applications'),
        request('/api/hpahub/manage/projects'),
      ]);
      members = memberData.items || [];
      applications = applicationData.items || [];
      membersMore = memberData.hasMore;
      applicationsMore = applicationData.hasMore;
      projects = projectData.items || [];
    } catch (err) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  }

  async function loadMore(kind) {
    moreBusy = true;
    error = '';
    try {
      const current = kind === 'members' ? members : applications;
      const data = await request(`/api/hpahub/manage/${kind}?offset=${current.length}`);
      if (kind === 'members') {
        members = [...members, ...data.items];
        membersMore = data.hasMore;
      } else {
        applications = [...applications, ...data.items];
        applicationsMore = data.hasMore;
      }
    } catch (err) {
      error = err.message || String(err);
    } finally {
      moreBusy = false;
    }
  }

  async function handleApplication(application, status) {
    applicationBusy = application.id;
    error = '';
    notice = '';
    try {
      await request(`/api/hpahub/manage/applications/${encodeURIComponent(application.id)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status }),
      });
      notice = status === 'accepted' ? '申请已通过。' : '申请已拒绝。';
      await load();
    } catch (err) {
      error = err.message || String(err);
    } finally {
      applicationBusy = '';
    }
  }

  onMount(load);
</script>

<section class="admin-directory">
  <div class="section-head">
    <div>
      <h2>{$t('成员与项目')}</h2>
      <p>{$t('查看全部成员资料、待审批申请和项目状态。')}</p>
    </div>
    <button type="button" class="secondary" on:click={load} disabled={loading}>{loading ? $t('正在加载…') : $t('刷新')}</button>
  </div>

  {#if error}<p class="message error" role="alert">{$t(error)}</p>{/if}
  {#if notice}<p class="message success" role="status">{$t(notice)}</p>{/if}

  <section class="admin-block">
    <h3>{$t('成员信息')} <span class="count">{members.length}</span></h3>
    {#if members.length}
      <div class="table-wrap">
        <table>
          <thead><tr><th>{$t('昵称')}</th><th>{$t('邮箱')}</th><th>{$t('角色')}</th><th>{$t('账号状态')}</th><th>{$t('注册来源')}</th><th>{$t('活动范围')}</th><th>{$t('展示职业')}</th><th>{$t('加入时间')}</th></tr></thead>
          <tbody>
            {#each members as member}
              <tr>
                <td><strong>{member.name || '—'}</strong>{#if member.profilePublic}<span class="tag">{$t('公开')}</span>{/if}</td>
                <td>{member.email || '—'}</td>
                <td>{$t(roleLabels[member.role] || member.role)}</td>
                <td>{$t(statusLabels[member.accountStatus] || member.accountStatus || '正常')}</td>
                <td>{$t(sourceLabels[member.registrationSource] || member.registrationSource || '—')}</td>
                <td>{member.activityAreas?.join('、') || '—'}</td>
                <td>{member.professions?.join('、') || '—'}</td>
                <td>{formatDate(member.created)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p>{$t('暂无成员。')}</p>
    {/if}
    {#if membersMore}<button type="button" class="secondary" on:click={() => loadMore('members')} disabled={loading || moreBusy}>{$t('加载更多')}</button>{/if}
  </section>

  <section class="admin-block">
    <h3>{$t('待审批申请')} <span class="count">{applications.length}</span></h3>
    {#if applications.length}
      <div class="application-list">
        {#each applications as application}
          <article class="application-card">
            <div class="application-head">
              <div>
                <strong>{application.userName || application.userEmail || '—'}</strong>
                <span class="muted">{$t(application.source === 'improvement' ? '申请改进' : '申请加入')} · {application.projectTitle || application.projectId}</span>
              </div>
              <time>{formatDate(application.created)}</time>
            </div>
            {#if application.message}<p>{application.message}</p>{/if}
            <div class="application-actions">
              <button type="button" class="primary" on:click={() => handleApplication(application, 'accepted')} disabled={applicationBusy === application.id}>{$t('通过')}</button>
              <button type="button" class="danger" on:click={() => handleApplication(application, 'rejected')} disabled={applicationBusy === application.id}>{$t('拒绝')}</button>
            </div>
          </article>
        {/each}
      </div>
    {:else}
      <p>{$t('暂无待审批申请。')}</p>
    {/if}
    {#if applicationsMore}<button type="button" class="secondary" on:click={() => loadMore('applications')} disabled={loading || moreBusy}>{$t('加载更多')}</button>{/if}
  </section>

  <section class="admin-block">
    <h3>{$t('全部项目')} <span class="count">{projects.length}</span></h3>
    {#if projects.length}
      <div class="table-wrap">
        <table>
          <thead><tr><th>{$t('项目')}</th><th>{$t('负责人')}</th><th>{$t('执行状态')}</th><th>{$t('审核状态')}</th><th>{$t('难度')}</th><th>{$t('更新时间')}</th></tr></thead>
          <tbody>
            {#each projects as project}
              <tr>
                <td><strong>{project.title}</strong>{#if project.summary}<div class="muted">{project.summary}</div>{/if}</td>
                <td>{project.ownerName || '—'}</td>
                <td>{$t(projectStatusLabels[project.status] || project.status || '—')}</td>
                <td>{$t(reviewLabels[project.reviewStatus] || project.reviewStatus || '—')}</td>
                <td>{#if project.reviewStatus === 'approved' && project.difficulty}<span class={`rank rank-pill difficulty-badge rank-${difficultyRank(project.difficulty)}`}>{project.difficulty}</span>{:else}<span class="rank-pending">{$t('待定')}</span>{/if}</td>
                <td>{formatDate(project.updated)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p>{$t('暂无项目。')}</p>
    {/if}
  </section>
</section>

<style>
  .admin-directory { margin: 32px 0; }
  .admin-directory .section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
  .admin-directory .section-head .secondary { width: auto; min-height: 42px; padding: 0 16px; justify-content: center; gap: 0; white-space: nowrap; }
  .admin-directory h3 { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; color: var(--ink-primary); }
  .count { display: inline-flex; min-width: 24px; height: 22px; align-items: center; justify-content: center; padding: 0 8px; border-radius: 999px; background: var(--surface-tint); border: 1px solid var(--border-subtle); color: var(--accent-deep); font-family: var(--font-mono); font-size: 11.5px; }
  .admin-block { margin-top: 26px; padding: 22px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-sm); }
  .table-wrap { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; min-width: 760px; }
  th, td { padding: 10px 12px; border-bottom: 1px solid var(--border-subtle); text-align: left; vertical-align: top; font-size: 13px; }
  th { color: var(--ink-muted); font-weight: 600; white-space: nowrap; background: var(--surface-soft); }
  .muted { color: var(--ink-subtle); font-size: 12px; }
  .tag { display: inline-block; margin-left: 6px; padding: 1px 7px; border-radius: 999px; background: var(--surface-tint); color: var(--accent-deep); font-size: 11px; font-weight: 600; }
  .application-list { display: grid; gap: 12px; }
  .application-card { padding: 16px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-elevated); transition: border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  .application-card:hover { border-color: var(--border-strong); box-shadow: var(--shadow-xs); }
  .application-head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 8px; }
  .application-head span { margin-left: 8px; }
  .application-card p { margin: 8px 0; color: var(--ink-secondary); }
  .application-actions { display: flex; gap: 8px; margin-top: 10px; }
  .application-actions .danger { min-height: 44px; padding: 0 16px; border: 1px solid var(--danger-border); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--danger); font-weight: 600; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out); }
  .application-actions .danger:hover:not(:disabled) { background: var(--danger-bg); border-color: var(--danger); }
</style>
