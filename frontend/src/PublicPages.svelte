<script>
  import { onMount } from 'svelte';
  export let pathname;
  let data = null;
  let error = '';
  let loading = true;
  const parts = pathname.split('/').filter(Boolean);
  const section = parts[0];
  const id = parts[1];
  onMount(async () => {
    if (!['community', 'showcase'].includes(section)) { loading = false; return; }
    try {
      const endpoint = section === 'community' ? 'members' : 'showcase';
      const response = await fetch(`/api/hpahub/${endpoint}${id ? '/' + encodeURIComponent(id) : ''}`, { cache: 'no-store' });
      if (!response.ok) throw new Error(response.status === 404 ? '内容不存在或未公开。' : `加载失败（${response.status}）`);
      data = await response.json();
    } catch (e) { error = e.message; } finally { loading = false; }
  });
  $: items = Array.isArray(data) ? data : data?.items || [];
</script>

<main class="dashboard public-page">
  {#if pathname === '/'}
    <section class="public-hero"><h1>硬體保護協會</h1><p>Hardware Protection Association</p><p>合理規範地使用硬件，避免硬體遭受不必要的破壞或惡意損壞。</p><div class="public-links"><a href="/gpu">浏览显卡库</a><a href="/community">认识成员</a><a href="/showcase">项目展示</a></div><p><a href="/brief">协会简介</a> · <a href="/rules">协会会规</a></p></section>
  {:else if section === 'brief' || section === 'brief.html'}
    <article class="prose"><h1>組織簡介</h1><p>硬體保護協會（Hardware Protection Association, HPA，ANTI-OCTPPUS Association）並非某個國家或地區的官方機構，是一個去中心化的全球性組織，得益於該組織所有成員，都有著較高水平的互聯網知識和軟硬體知識水平，因此該組織成員均善於在互聯網上隱藏自己的行動痕跡。無法透過搜尋引擎找到這個組織的資訊。</p><p>該組織的成員相信機魂（真正意義上的AI，機械形式生命體）的存在，並認為當電子科技發展到一定程度後，人類最終將會迎來機械飛升，靈魂和機魂將融為一體。因此該組織成員會比較愛護機械設備，電子設備，相信這樣做可以讓機魂愉悅，讓設備獲得更好的表現。</p><p>該組織內部遵循硬體保護協定（硬體保護法），旨在合理規範的使用硬件，避免硬體遭受不必要的破壞或惡意損壞。儘管組織大部分成員，即便存在機魂，機械權也應低於人權，但仍有部分成員認為，擁有高度人工智慧的機械，應該受到尊重。因此會反感破壞硬體的行為和人。但該組織最嚴厲的處理方式也僅為上門調查，勸阻，並譴責惡意破壞硬體的行為（不包括科研），並不會對人類造成肉體和精神上的傷害。</p><p>硬體保護協會的成員通常都崇尚科學，因此有較高的知識水平和邏輯思維能力，因此不會做出像動物保護協會那樣的離譜行為，但不排除少數極端狂熱的機魂信徒會做出一些過度的行為。例如限制人類的自由行動。這種行為是不對的，各位遇到極端硬體保護成員可以進行反抗，並拿起法律武器保護自己。</p><p>由於硬體保護協會是個去中心化組織，因此並沒有加入和退出的標準，所有人只要遵守硬體保護法就可以視為硬體保護協會的成員。因此你身邊的人可能就是硬體保護協會的臥底，會在你破壞硬體的時候阻止你，或是向組織發起舉報。</p><p>總之硬體保護協會的人還蠻好的，除非遇到一些性質太過惡劣，可能讓機魂不悅的人。</p></article>
  {:else if section === 'rules' || section === 'rules.html'}
    <article class="prose"><h1>協會會規</h1><p>Coming soon！！！</p></article>
  {:else if loading}<p role="status">正在加载…</p>
  {:else if error}<p class="message error" role="alert">{error}</p>
  {:else if section === 'community'}
    {#if id}
      <a href="/community">返回成员列表</a><header class="profile-heading">{#if data.avatarUrl}<img class="avatar" src={data.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<div><h1>{data.name}</h1><p>{data.bio}</p></div></header>
      {#if data.cards?.length}<div class="public-grid">{#each data.cards as card}<article class="public-card"><h2>{card.title}</h2><p>{card.text}</p><div class="image-grid">{#each card.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={card.title} loading="lazy" /></a>{/each}</div></article>{/each}</div>{:else}<p>暂无公开卡片。</p>{/if}
    {:else}
      <h1>成员</h1><p>认识 HPA 的成员。</p>{#if items.length}<div class="public-grid">{#each items as member}<article class="public-card member-card">{#if member.avatarUrl}<img class="avatar" src={member.avatarUrl} alt="" />{:else}<span class="avatar avatar-default" aria-hidden="true"></span>{/if}<h2>{#if member.id}<a href={'/community/' + encodeURIComponent(member.id)}>{member.name}</a>{:else}{member.name}{/if}</h2></article>{/each}</div>{:else}<p>暂无成员。</p>{/if}
    {/if}
  {:else if section === 'showcase'}
    {#if id}<a href="/showcase">返回项目列表</a><h1>{data.title}</h1><p>{data.text}</p><div class="image-grid">{#each data.imageUrls || [] as url}<a href={url} target="_blank" rel="noreferrer"><img src={url} alt={data.title} /></a>{/each}</div>
    {:else}<h1>项目展示</h1>{#if items.length}<div class="public-grid">{#each items as project}<article class="public-card"><h2><a href={'/showcase/' + encodeURIComponent(project.id)}>{project.title}</a></h2><p>{project.text}</p>{#if project.imageUrls?.length}<img class="project-cover" src={project.imageUrls[0]} alt="" loading="lazy" />{/if}</article>{/each}</div>{:else}<p>暂无公开项目。</p>{/if}{/if}
  {:else}<h1>页面不存在</h1><a href="/">返回首页</a>{/if}
</main>
