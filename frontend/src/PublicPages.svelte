<script>
  import { onMount } from 'svelte';
  import { t } from './i18n.js';
  export let pathname;
  let data = null;
  let error = '';
  let loading = true;
  let errorParams = {};
  const parts = pathname.split('/').filter(Boolean);
  const section = parts[0];
  const id = parts[1];
  onMount(async () => {
    if (!['community', 'showcase'].includes(section)) { loading = false; return; }
    try {
      const endpoint = section === 'community' ? 'members' : 'showcase';
      const response = await fetch(`/api/hpahub/${endpoint}${id ? '/' + encodeURIComponent(id) : ''}`, { cache: 'no-store' });
      if (!response.ok) { error = response.status === 404 ? '内容不存在或未公开。' : '加载失败（{status}）'; errorParams = { status: response.status }; return; }
      data = await response.json();
    } catch (e) { error = e.message; } finally { loading = false; }
  });
  $: items = Array.isArray(data) ? data : data?.items || [];
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
    {:else}
      <h1>{$t('成员')}</h1><p>{$t('认识 HPA 的成员。')}</p>{#if items.length}<div class="public-grid">{#each items as member}<article class="public-card member-card">{#if member.avatarUrl}<img class="avatar" src={member.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<h2>{#if member.id}<a href={'/community/' + encodeURIComponent(member.id)}>{member.name}</a>{:else}{member.name}{/if}</h2></article>{/each}</div>{:else}<p>{$t('暂无成员。')}</p>{/if}
    {/if}
  {:else if section === 'showcase'}
    {#if id}<a href="/showcase">{$t('返回项目列表')}</a><h1>{data.title}</h1><p>{data.text}</p><div class="image-grid">{#each data.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={data.title} /></a>{/each}</div>
    {:else}<h1>{$t('项目展示')}</h1>{#if items.length}<div class="public-grid">{#each items as project}<article class="public-card"><h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2><p>{project.text}</p>{#if project.imageUrls?.length}<img class="project-cover" src={project.imageUrls[0]} alt="" loading="lazy" />{/if}</article>{/each}</div>{:else}<p>{$t('暂无公开项目。')}</p>{/if}{/if}
  {:else}<h1>{$t('页面不存在')}</h1><a href="/">{$t('返回首页')}</a>{/if}
</main>
