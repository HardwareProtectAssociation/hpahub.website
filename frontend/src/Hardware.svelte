<script>
  import { onMount, tick } from 'svelte';
  import { get } from 'svelte/store';
  import { t } from './i18n.js';
  export let token = '';
  export let role = 'guest';
  $: canEdit = !!token && ['member', 'admin', 'octopus'].includes(role);
  const suggestions = {game:['三角洲大坝','死亡搁浅2','CS2','LOL大乱斗','永劫无间','2077','红色沙漠','绝区零'],resolution:['1080P','2K','4K','1080P60','4K60','8K','8K60'],quality:['最低','非常低','低','中','中等','高','超高','极高','最高','超级','极致','影视级','电影级','原画','性能','均衡','流畅','便携'],ray_tracing:['开启','无','中','超级','变态'],upscale_tech:['DLSS','FSR','XeSS'],upscale_quality:['性能','平衡'],frame_gen:['X2','X3','X4','X5'],codec:['H.264','H.265/HEVC','HEVC','AV1','VP9','VP8','MPEG-2','MPEG-4','VC-1','MJPEG','剪映H264','剪映HEVC','剪映AV1']};
  let types = [], records = [], gameOptions = [], error = '', notice = '', loading = false, busy = false;
  let view = 'list', kind = 'general', detail = null, id = null, editing = false, search = '', filter = 'all', brandFilter = '';
  let gameFilter = '', resolutionFilter = '', qualityFilter = '', media = [], mediaPath = '', newGame = '', submitKind = 'general';
  let selected = [], compareDetails = [], metric = '', compareGame = '', compareResolution = '', compareField = 'avg_fps';
  const typeOf = k => types.find(x => x.key === k);
  const typeLabel = k => typeOf(k)?.label || k;
  const recordName = (d, k) => d.fields?.[typeOf(k)?.name_field] || d.name || '';
  async function request(path, method = 'GET', body) {
    const res = await fetch('/api/hpahub/hardware' + path, {method, headers:{...(token ? {Authorization:`Bearer ${token}`} : {}), ...(body !== undefined ? {'Content-Type':'application/json'} : {})}, ...(body !== undefined ? {body:JSON.stringify(body)} : {})});
    const text = await res.text(); let data;
    try { data = text ? JSON.parse(text) : null; } catch { data = text; }
    if (!res.ok) throw new Error(data?.error || data?.message || get(t)('请求失败（{status}）', {status: res.status}));
    return data;
  }
  async function load() {
    loading = true; error = '';
    try { const [schema,data,options] = await Promise.all([request('/schema'),request('/records'),request('/options/games')]); types = schema.types || []; if (!types.some(x => x.key === submitKind) && types.length) submitKind = types[0].key; records = data.map(enrich); gameOptions = options; }
    catch (e) { error = e.message; } finally { loading = false; }
  }
  onMount(load);
  function enrich(record) { return {...record, fields: record.data?.fields || {}, rows: record.data?.rows || {}, custom: record.data?.custom || {}}; }
  function blank(k) { return normalize({kind:k, name:'', brand:'', subtitle:'', sort_order:0, data:{fields:{},rows:{},custom:{}}}, k); }
  function normalize(record, k) {
    const d = enrich(record);
    for (const group of typeOf(k)?.groups || []) d.custom[group.key] ||= {hidden: [], extra: []};
    for (const table of typeOf(k)?.tables || []) d.rows[table.key] ||= [];
    return d;
  }
  $: gameCount = records.reduce((n, r) => n + (r.rows?.games?.length || 0), 0);
  $: summary = [...types.map(x => $t(x.label) + ' ' + records.filter(r => r.kind === x.key).length), $t('游戏记录') + ' ' + gameCount].join('　·　');
  $: brands = unique(records.map(r => r.brand)).filter(Boolean);
  $: shown = records.filter(r => (filter === 'all' || filter === r.kind) && (!brandFilter || r.brand === brandFilter) && JSON.stringify(r).toLowerCase().includes(search.trim().toLowerCase()));
  $: hasGames = (typeOf(kind)?.tables || []).some(x => x.key === 'games');
  async function open(k, recordId, edit = false) {
    kind = k; id = recordId; editing = edit; error = ''; notice = ''; loading = true; gameFilter = resolutionFilter = qualityFilter = ''; media = []; reviewTitle = ''; reviewContent = '';
    try {
      detail = recordId ? normalize(await request('/records/' + recordId), k) : blank(k);
      view = 'detail';
      if (recordId && canEdit) media = (await request('/records/' + recordId + '/media')).paths || [];
      if (recordId) await loadReviews();
    } catch (e) { error = e.message; } finally { loading = false; }
  }
  async function save() {
    if (!canEdit || busy) return; busy = true; error = ''; notice = '';
    try {
      const sc = typeOf(kind);
      detail.name = detail.fields[sc?.name_field] || '';
      detail.brand = detail.fields.brand || '';
      detail.subtitle = (sc?.subtitle_fields || '').split(',').map(key => detail.fields[key.trim()]).filter(Boolean).join(' · ');
      const data = {fields: detail.fields, rows: detail.rows, custom: detail.custom};
      const result = await request('/records' + (id ? '/' + id : ''), id ? 'PUT' : 'POST', {kind, name:detail.name, brand:detail.brand, subtitle:detail.subtitle, sort_order:detail.sort_order, data});
      id = result.id || id; notice = '已保存';
    } catch (e) { error = e.message; } finally { busy = false; }
  }
  async function action(actionName) {
    if (!canEdit || !id || busy) return;
    if (actionName === 'delete' && !confirm(get(t)('确定删除此记录？此操作无法撤销。'))) return;
    busy = true; error = '';
    try {
      const path = '/records/' + id;
      if (actionName === 'delete') { await request(path,'DELETE'); view = 'list'; await load(); }
      else { const result = await request(path + '/copy','POST'); await open(kind,result.id); }
    } catch (e) { error = e.message; } finally { busy = false; }
  }
  async function reorder(recordId, delta) {
    const next = [...records], index = next.findIndex(x => x.id === recordId), target = index + delta;
    if (index < 0 || target < 0 || target >= next.length) return;
    [next[index],next[target]] = [next[target],next[index]];
    try { await request('/reorder','POST',{ids:next.map(x => x.id)}); records = next; } catch (e) { error = e.message; }
  }
  function addRow(container,key) { container[key] = [...container[key],{}]; detail = detail; }
  function removeRow(container,key,index) { container[key] = container[key].filter((_,i) => i !== index); detail = detail; }
  function addCustom(groupKey) { const label = prompt(get(t)('项目名称')); if (label?.trim()) { detail.custom[groupKey].extra = [...detail.custom[groupKey].extra,{key:'c_'+Date.now().toString(36)+Math.random().toString(36).slice(2,6),label:label.trim(),value:''}]; detail = detail; } }
  function hideField(groupKey, key) { detail.fields[key] = ''; detail.custom[groupKey].hidden = [...detail.custom[groupKey].hidden, key]; detail = detail; }
  async function addGame() { if (!newGame.trim()) return; try { await request('/options/games','POST',{value:newGame.trim()}); gameOptions = await request('/options/games'); newGame = ''; notice = '游戏选项已添加'; } catch (e) { error = e.message; } }
  async function changeMedia(actionName,path) { if (!id || (actionName === 'add' && !path.trim())) return; try { const paths = actionName === 'add' ? (media.includes(path.trim()) ? media : [...media,path.trim()]) : media.filter(p => p !== path); media = (await request('/records/' + id + '/media','PUT',{paths})).paths; mediaPath = ''; } catch (e) { error = e.message; } }
  function mediaUrl(path) { if (typeof path !== 'string') return ''; try { const url = new URL(path, location.origin); return ['http:','https:'].includes(url.protocol) ? url.href : ''; } catch { return ''; } }
  let reviews = [], reviewTitle = '', reviewContent = '', reviewBusy = false;
  async function loadReviews() {
    if (!id) { reviews = []; return; }
    try { reviews = (await request('/records/' + id + '/reviews')).items || []; } catch { reviews = []; }
  }
  async function submitReview() {
    if (!id || reviewBusy || !reviewTitle.trim() || !reviewContent.trim()) return;
    reviewBusy = true;
    try {
      await request('/records/' + id + '/reviews', 'POST', { title: reviewTitle.trim(), content: reviewContent.trim() });
      reviewTitle = ''; reviewContent = '';
      await loadReviews();
    } catch (e) { error = e.message; } finally { reviewBusy = false; }
  }
  async function likeReview(review) {
    try {
      const result = await request('/reviews/' + review.id + '/like', 'POST');
      reviews = reviews.map((r) => (r.id === review.id ? { ...r, likedByMe: result.liked, likeCount: result.likeCount } : r));
    } catch (e) { error = e.message; }
  }
  async function deleteReview(review) {
    if (!confirm(get(t)('确定删除此评测？'))) return;
    try {
      await request('/reviews/' + review.id, 'DELETE');
      reviews = reviews.filter((r) => r.id !== review.id);
    } catch (e) { error = e.message; }
  }
  const unique = values => [...new Set(values.map(v => String(v ?? '')))];
  $: gameRows = detail ? detail.rows?.games || [] : [];
  $: resolutions = unique(gameRows.filter(r => !gameFilter || r.game === gameFilter).map(r => r.resolution));
  $: qualities = unique(gameRows.filter(r => (!gameFilter || r.game === gameFilter) && (!resolutionFilter || r.resolution === resolutionFilter)).map(r => r.quality));
  $: filteredGames = gameRows.filter(r => (!gameFilter || r.game === gameFilter) && (!resolutionFilter || r.resolution === resolutionFilter) && (!qualityFilter || r.quality === qualityFilter));
  async function compare() {
    if (selected.length < 2) { error = '至少选择两条记录'; return; }
    loading = true; error = '';
    try {
      compareDetails = (await Promise.all(selected.map(i => request('/records/' + i)))).map(d => normalize(d, d.kind));
      await tick();
      if (!metricOptions.some(([value]) => value === metric)) metric = metricOptions[0]?.[0] || '';
      view = 'compare';
    } catch (e) { error = e.message; } finally { loading = false; }
  }
  $: compareGames = unique(compareDetails.flatMap(d => (d.rows?.games || []).map(r => r.game))).filter(Boolean);
  $: compareResolutions = unique(compareDetails.flatMap(d => (d.rows?.games || []).filter(r => r.game === compareGame).map(r => r.resolution)));
  $: compareFieldOptions = (typeOf(compareDetails[0]?.kind)?.tables || []).find(x => x.key === 'games')?.columns?.filter(c => !['game','resolution','quality','note'].includes(c.key)) || [];
  function numeric(v) { const m = String(v ?? '').match(/\d+(?:\.\d+)?/g); return !m ? null : m.length > 1 && /[-~～]/.test(v) ? (Number(m[0])+Number(m[1]))/2 : Number(m[0]); }
  function compareValue(d) {
    if (metric === 'games') { const rows = (d.rows?.games || []).filter(r => r.game === compareGame && String(r.resolution || '') === compareResolution && numeric(r[compareField]) !== null); rows.sort((a,b) => numeric(b[compareField])-numeric(a[compareField])); return rows[0]?.[compareField] ?? ''; }
    if (metric.startsWith('fields.')) return d.fields[metric.slice(7)] ?? '';
    if (metric.startsWith('row.')) { const parts = metric.split('.'); return (d.rows[parts[1]] || []).find(r => r.name === parts.slice(2).join('.'))?.score ?? ''; }
    if (metric.startsWith('extra.')) { const label = metric.slice(6); for (const custom of Object.values(d.custom || {})) { const hit = (custom.extra || []).find(x => x.label === label); if (hit) return hit.value ?? ''; } }
    return '';
  }
  function metricOptionsFor(details, tr) {
    const options = [], seen = new Set();
    const add = (value, label) => { if (!seen.has(value)) { seen.add(value); options.push([value, label]); } };
    for (const d of details) {
      for (const group of typeOf(d.kind)?.groups || []) for (const field of group.fields) add('fields.' + field.key, tr(group.key) + ' · ' + tr(field.label));
      for (const table of typeOf(d.kind)?.tables || []) {
        const keys = (table.columns || []).map(c => c.key);
        if (keys.includes('name') && keys.includes('score')) for (const row of d.rows[table.key] || []) if (row.name) add('row.' + table.key + '.' + row.name, tr(table.title) + ' · ' + row.name);
      }
      for (const custom of Object.values(d.custom || {})) for (const extra of custom.extra || []) if (extra.label) add('extra.' + extra.label, extra.label);
    }
    if (details.some(d => (d.rows?.games || []).length)) add('games', tr('游戏帧数'));
    return options;
  }
  $: metricOptions = metricOptionsFor(compareDetails, $t);
</script>

<div class="gpu-page">
  <header class="heading"><div><h1>{$t('硬件数据库')}</h1><p>{$t('浏览显卡、CPU 与设备的实测数据。')}</p></div>{#if view !== 'list'}<button on:click={() => { view = 'list'; editing = false; load(); }}>{$t('返回列表')}</button>{/if}</header>
  {#if error}<div class="message error" role="alert">{$t(error)}</div>{/if}
  {#if notice}<div class="message" role="status">{$t(notice)}</div>{/if}
  {#if loading}<p role="status">{$t('正在加载…')}</p>{/if}
  {#if view === 'list'}
    <div class="summary">{summary}</div>
    <nav class="tabs" aria-label={$t('硬件分类')}><button class:active={filter === 'all'} on:click={() => filter = 'all'}>{$t('全部')}</button>{#each types as item}<button class:active={filter === item.key} on:click={() => filter = item.key}>{$t(item.label)}</button>{/each}</nav>
    <div class="toolbar"><label class="search">{$t('搜索')}<input type="search" bind:value={search} placeholder={$t('型号、品牌或参数')} /></label><label>{$t('品牌')}<select bind:value={brandFilter}><option value="">{$t('全部')}</option>{#each brands as b}<option value={b}>{b}</option>{/each}</select></label><span>{$t('{count} 条记录', {count:shown.length})}</span><button on:click={compare}>{$t('对比所选记录（{count}）', {count:selected.length})}</button>{#if canEdit}<label>{$t('硬件类型')}<select bind:value={submitKind}>{#each types as item}<option value={item.key}>{$t(item.label)}</option>{/each}</select></label><button class="primary" on:click={() => open(submitKind,null,true)}>{$t('提交硬件')}</button>{/if}</div>
    {#if !loading && !shown.length}<p class="empty">{$t('没有匹配的记录。')}</p>{/if}
    <div class="list">{#each shown as data (data.id)}<article class="record"><div><span class="badge">{$t(typeLabel(data.kind))}</span><button class="record-title" on:click={() => open(data.kind,data.id)}>{recordName(data, data.kind) || $t('未命名{type}', {type:$t(typeLabel(data.kind))})}</button><p>{data.subtitle || ''}</p></div><div class="record-actions"><label><input type="checkbox" value={data.id} bind:group={selected} /> {$t('对比')}</label>{#if canEdit}<button aria-label={$t('上移')} on:click={() => reorder(data.id,-1)}>{$t('上移')}</button><button aria-label={$t('下移')} on:click={() => reorder(data.id,1)}>{$t('下移')}</button>{/if}</div></article>{/each}</div>
  {:else if view === 'compare'}
    <section class="panel"><h2>{$t('数据对比')}</h2><div class="toolbar"><label>{$t('对比项目')}<select bind:value={metric}>{#each metricOptions as [value,label]}<option {value}>{label}</option>{/each}</select></label>{#if metric === 'games'}<label>{$t('游戏')}<select bind:value={compareGame} on:change={() => compareResolution = ''}><option value="">{$t('选择游戏')}</option>{#each compareGames as g}<option value={g}>{g}</option>{/each}</select></label><label>{$t('分辨率')}<select bind:value={compareResolution}>{#each compareResolutions as r}<option value={r}>{r || $t('未填')}</option>{/each}</select></label><label>{$t('帧数指标')}<select bind:value={compareField}>{#each compareFieldOptions as c}<option value={c.key}>{$t(c.label)}</option>{/each}</select></label>{/if}</div><div class="comparison">{#each compareDetails as d}<div><strong>{recordName(d, d.kind) || $t('未命名')}</strong><span>{compareValue(d) || '—'}</span></div>{/each}</div><p class="hint">{$t('同一游戏和分辨率有多组设置时，显示所选指标的最高值。')}</p></section>
  {:else if detail}
    <div class="toolbar detail-actions"><h2>{editing ? $t(id ? '编辑' : '新增') : ''}{$t(typeLabel(kind))} · {recordName(detail, kind) || $t('未命名')}</h2>{#if canEdit}{#if editing}<button class="primary" disabled={busy} on:click={save}>{$t('保存')}</button><button on:click={() => id ? open(kind,id) : (view = 'list')}>{$t('取消')}</button>{:else}<button class="primary" on:click={() => editing = true}>{$t('编辑')}</button><button disabled={busy} on:click={() => action('copy')}>{$t('复制一份')}</button><button class="danger" disabled={busy} on:click={() => action('delete')}>{$t('删除')}</button>{/if}{/if}</div>
    {#each typeOf(kind)?.groups || [] as group}
      {@const custom = detail.custom[group.key]}
      <section class="panel"><h3>{$t(group.key)}</h3><div class="fields">{#each group.fields as field}{#if !custom?.hidden?.includes(field.key)}<div class="field"><label>{$t(field.label)}{#if editing}{#if field.key === 'note'}<textarea bind:value={detail.fields[field.key]}></textarea>{:else}<input bind:value={detail.fields[field.key]} />{/if}{:else}<span>{detail.fields[field.key] || '—'}</span>{/if}</label>{#if editing && custom}<button class="small" aria-label={$t('删除{label}', {label:$t(field.label)})} on:click={() => hideField(group.key,field.key)}>{$t('删除')}</button>{/if}</div>{/if}{/each}{#each custom?.extra || [] as extra, index}<div class="field"><label>{extra.label || extra.key}{#if editing}<input value={extra.value} on:input={(event) => { detail.custom[group.key].extra[index].value = event.currentTarget.value; }} />{:else}<span>{extra.value || '—'}</span>{/if}</label>{#if editing}<button class="small" on:click={() => { detail.custom[group.key].extra = custom.extra.filter((_,i) => i !== index); }}>{$t('删除')}</button>{/if}</div>{/each}</div>{#if editing && custom}<div class="toolbar"><button on:click={() => addCustom(group.key)}>{$t('添加项目')}</button>{#if custom.hidden.length}<button on:click={() => { detail.custom[group.key].hidden = []; }}>{$t('恢复内置项目')}</button>{/if}</div>{/if}</section>
    {/each}
    {#each typeOf(kind)?.tables || [] as table}
      {@const rows = table.key === 'games' && !editing ? filteredGames : detail.rows[table.key] || []}
      <section class="panel"><h3>{$t(table.title)} <small>{$t('{count} 条', {count:rows.length})}</small></h3>
        {#if table.key === 'games' && !editing}<div class="toolbar"><label>{$t('游戏')}<select bind:value={gameFilter} on:change={() => { resolutionFilter = ''; qualityFilter = ''; }}><option value="">{$t('全部')}</option>{#each unique(gameRows.map(r => r.game)).filter(Boolean) as g}<option value={g}>{g}</option>{/each}</select></label><label>{$t('分辨率')}<select bind:value={resolutionFilter} on:change={() => qualityFilter = ''}><option value="">{$t('全部')}</option>{#each resolutions.filter(Boolean) as r}<option value={r}>{r}</option>{/each}</select></label><label>{$t('画质')}<select bind:value={qualityFilter}><option value="">{$t('全部')}</option>{#each qualities.filter(Boolean) as q}<option value={q}>{q}</option>{/each}</select></label></div>{/if}
        <div class="table-wrap"><table><thead><tr>{#each table.columns as column}<th>{$t(column.label)}</th>{/each}{#if editing}<th>{$t('操作')}</th>{/if}</tr></thead><tbody>{#each rows as row,index}<tr>{#each table.columns as column}<td>{#if editing}<input aria-label={$t(column.label)} list={suggestions[column.key] ? 'gpu-option-'+column.key : undefined} value={row[column.key]} on:input={(event) => { detail.rows[table.key][index][column.key] = event.currentTarget.value; detail = detail; }} />{:else}{row[column.key] || '—'}{/if}</td>{/each}{#if editing}<td><button on:click={() => removeRow(detail.rows,table.key,index)}>{$t('删除')}</button></td>{/if}</tr>{/each}</tbody></table></div>{#if !rows.length}<p class="empty">{$t('暂无数据')}</p>{/if}{#if editing}<button on:click={() => addRow(detail.rows,table.key)}>{$t('添加一行')}</button>{/if}
      </section>
    {/each}
    {#if editing && hasGames}<section class="panel"><h3>{$t('游戏选项')}</h3><div class="toolbar"><label>{$t('新游戏名称')}<input bind:value={newGame} /></label><button on:click={addGame}>{$t('添加游戏选项')}</button></div></section>{/if}
    {#if id && canEdit}<section class="panel"><h3>{$t('相关媒体')}</h3>{#if media.length}<ul>{#each media as path}<li>{#if mediaUrl(path)}<a href={mediaUrl(path)} target="_blank" rel="noopener noreferrer">{path}</a>{:else}{typeof path === 'string' ? path : JSON.stringify(path)}{/if}<button class="small" on:click={() => changeMedia('remove',path)}>{$t('移除')}</button></li>{/each}</ul>{:else}<p class="empty">{$t('暂无相关媒体')}</p>{/if}<div class="toolbar"><label>{$t('媒体路径或链接')}<input bind:value={mediaPath} /></label><button on:click={() => changeMedia('add',mediaPath)}>{$t('添加媒体')}</button></div></section>{/if}
    {#if id && !editing}<section class="panel"><h3>{$t('硬件评测')}</h3>
      {#if reviews.length}
        <ul class="review-list">
          {#each reviews as review (review.id)}
            <li class="review-item">
              <strong>{review.title}</strong>
              <p>{review.content}</p>
              <div class="review-foot">
                <span class="hint">{review.authorName || '—'}{#if review.created} · {new Date(review.created).toLocaleDateString()}{/if}</span>
                {#if review.canLike}<button class="small" on:click={() => likeReview(review)}>{review.likedByMe ? $t('取消点赞') : $t('点赞')} {review.likeCount}</button>{:else}<span class="hint">♥ {review.likeCount}</span>{/if}
                {#if review.canDelete}<button class="small" on:click={() => deleteReview(review)}>{$t('删除')}</button>{/if}
              </div>
            </li>
          {/each}
        </ul>
      {:else}<p class="empty">{$t('暂无评测')}</p>{/if}
      {#if token}
        <div class="review-form">
          <label>{$t('评测标题')}<input bind:value={reviewTitle} maxlength="160" /></label>
          <label>{$t('评测内容')}<textarea bind:value={reviewContent} rows="3" maxlength="5000"></textarea></label>
          <button class="primary" disabled={reviewBusy} on:click={submitReview}>{$t('发布评测')}</button>
        </div>
      {:else}<p class="hint">{$t('登录后可发布评测。')}</p>{/if}
    </section>{/if}
    {#if editing}<button class="primary" disabled={busy} on:click={save}>{$t('保存所有更改')}</button>{/if}
  {/if}
  {#each Object.entries(suggestions) as [key,values]}<datalist id={'gpu-option-'+key}>{#each key === 'game' ? unique([...values,...gameOptions]) : values as value}<option {value}></option>{/each}</datalist>{/each}
</div>

<style>
  .gpu-page { box-sizing: border-box; width: min(1320px, 88%); color: var(--ink-primary); margin: 0 auto; padding: 36px 0 72px; animation: mint-enter 260ms var(--ease-out) both; }
  .heading, .toolbar, .detail-actions, .record, .record-actions { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }
  .heading { justify-content: space-between; margin-bottom: 1.5rem; }
  .heading h1 { font-size: clamp(28px, 3.2vw, 42px); letter-spacing: -0.03em; line-height: 1.2; margin: 0; }
  .heading p, .record p { margin: 0; }
  .heading p, .hint, .empty, .record p { color: var(--ink-muted); }
  .heading p { margin-top: 0.45rem; font-size: 14px; }
  .summary { padding: 14px 18px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); color: var(--ink-secondary); font-size: 13px; font-family: var(--font-mono); box-shadow: var(--shadow-xs); }
  .tabs { display: flex; gap: 0.45rem; overflow-x: auto; margin: 1.2rem 0; padding-bottom: 4px; }
  .tabs button { white-space: nowrap; border-radius: 999px; padding: 7px 14px; font-size: 13px; font-weight: 500; }
  .tabs .active, .primary { background: linear-gradient(135deg, var(--accent), var(--accent-deep)); color: #fff; border-color: transparent; box-shadow: var(--shadow-glow); }
  .toolbar { margin: 1rem 0; }
  .toolbar h2 { margin: 0 auto 0 0; }
  .toolbar label, .search { display: flex; align-items: center; gap: 0.5rem; color: var(--ink-secondary); font-size: 13px; }
  .search input { min-width: 15rem; }
  .list { display: grid; gap: 10px; margin-top: 12px; }
  .record { justify-content: space-between; padding: 16px 18px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--surface); box-shadow: var(--shadow-xs); transition: transform var(--t-med) var(--ease-out), border-color var(--t-med) var(--ease-out), box-shadow var(--t-med) var(--ease-out); }
  .record:hover { transform: translateY(-2px); border-color: var(--border-strong); box-shadow: var(--shadow-sm); }
  .record-title { display: block; border: 0; padding: 0.35rem 0; background: none; font-size: 1.08rem; font-weight: 700; text-align: left; color: var(--ink-primary); transition: color var(--t-fast) var(--ease-out); }
  .record-title:hover { color: var(--accent-deep); }
  .badge { display: inline-block; padding: 2px 9px; border-radius: 999px; background: var(--surface-tint); color: var(--accent-deep); font-size: 0.72rem; font-weight: 600; letter-spacing: 0.03em; }
  .record p { font-size: 0.88rem; }
  .panel { border: 1px solid var(--border-subtle); background: var(--surface); border-radius: var(--radius-md); padding: 1.25rem; margin: 1rem 0; box-shadow: var(--shadow-sm); }
  .panel h3 { margin: 0 0 1rem; font-size: 1rem; color: var(--ink-primary); }
  .panel h3 small { font-weight: 500; color: var(--ink-subtle); font-family: var(--font-mono); }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; }
  .fields label { display: flex; flex-direction: column; gap: 0.35rem; color: var(--ink-muted); font-size: 0.82rem; font-weight: 500; }
  .fields label span { font-size: 0.98rem; color: var(--ink-primary); font-weight: 500; overflow-wrap: anywhere; }
  .field { display: flex; align-items: flex-end; gap: 0.35rem; }
  .field label { flex: 1; min-width: 0; }
  input, select, textarea, button { font: inherit; }
  input:not([type=checkbox]), select, textarea { box-sizing: border-box; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-primary); padding: 0.55rem 0.7rem; max-width: 100%; transition: border-color var(--t-fast) var(--ease-out), box-shadow var(--t-fast) var(--ease-out); }
  input:not([type=checkbox]):focus, select:focus, textarea:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); outline: none; }
  textarea { min-height: 5rem; }
  button { cursor: pointer; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--surface-elevated); color: var(--ink-secondary); padding: 0.5rem 0.8rem; font-weight: 500; transition: background-color var(--t-fast) var(--ease-out), border-color var(--t-fast) var(--ease-out), transform var(--t-fast) var(--ease-out); }
  button:hover:not(:disabled) { border-color: var(--border-strong); background: var(--surface-tint); }
  button:active:not(:disabled) { transform: scale(0.985); }
  button:focus-visible, input:focus-visible, select:focus-visible, textarea:focus-visible, a:focus-visible { outline: 2px solid var(--accent-bright); outline-offset: 2px; }
  button:disabled { opacity: 0.55; cursor: wait; }
  .primary { background: linear-gradient(135deg, var(--accent), var(--accent-deep)); color: #fff; border-color: transparent; }
  .primary:hover:not(:disabled) { background: linear-gradient(135deg, var(--accent-bright), var(--accent)); transform: translateY(-1px); }
  .danger { color: var(--danger); border-color: var(--danger-border); }
  .danger:hover:not(:disabled) { background: var(--danger-bg); border-color: var(--danger); }
  .small { font-size: 0.75rem; padding: 0.3rem 0.6rem; }
  .table-wrap { overflow-x: auto; }
  table { border-collapse: collapse; width: 100%; font-size: 0.85rem; }
  th, td { border-bottom: 1px solid var(--border-subtle); padding: 0.65rem 0.7rem; text-align: left; white-space: nowrap; }
  th { color: var(--ink-muted); font-weight: 600; background: var(--surface-soft); }
  tbody tr:hover td { background: rgba(227, 242, 233, 0.42); }
  td input { width: 9rem; }
  .empty { font-size: 0.9rem; padding: 24px 0; }
  .message { padding: 0.85rem 1rem; border: 1px solid var(--border-strong); border-radius: var(--radius-sm); background: var(--surface-tint); color: var(--accent-deep); margin: 1rem 0; }
  .error { border-color: var(--danger-border); background: var(--danger-bg); color: var(--danger); }
  .comparison { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 0.85rem; }
  .comparison div { display: flex; flex-direction: column; gap: 0.5rem; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--surface-soft); padding: 1rem; }
  .comparison span { font-size: 1.45rem; font-weight: 700; font-family: var(--font-mono); color: var(--accent-deep); }
  .hint { font-size: 0.85rem; }
  li { margin: 0.5rem 0; overflow-wrap: anywhere; }
  li button { margin-left: 0.5rem; }
  .review-list { list-style: none; margin: 0 0 1rem; padding: 0; display: grid; gap: 0.85rem; }
  .review-item { border-bottom: 1px solid var(--border-subtle); padding-bottom: 0.85rem; }
  .review-item p { margin: 0.35rem 0; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--ink-secondary); }
  .review-foot { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }
  .review-form { display: grid; gap: 0.75rem; max-width: 650px; }
  .review-form label { display: grid; gap: 0.35rem; color: var(--ink-muted); font-size: 0.85rem; }
  @media (max-width: 600px) {
    .gpu-page { width: 90%; padding: 1.2rem 0 2.4rem; }
    .toolbar { align-items: stretch; }
    .toolbar label { width: 100%; justify-content: space-between; }
    .toolbar label input, .toolbar label select { flex: 1; }
    .fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .record-actions { width: 100%; }
  }
</style>
