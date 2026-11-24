<script>
  import { onMount } from 'svelte';
  export let token = '';
  export let role = 'guest';
  $: canEdit = !!token && ['member', 'admin', 'octopus'].includes(role);

  const basic = [['model','显卡型号'],['brand','品牌'],['vram_size','显存容量'],['vram_type','显存类型'],['vram_clock','显存频率'],['gpu_clock','核心频率'],['tgp','TGP (W)'],['actual_power','实测功耗 (W)'],['driver_version','驱动版本'],['test_date','测试日期'],['test_platform','测试平台'],['ambient_temp','环境温度']];
  const cooling = [['default_fan_temp','默认转速满载温度'],['max_fan_temp','满转速满载温度'],['temp_45db','45分贝满载温度'],['core_temp','满载核心温度'],['hotspot_temp','热点温度'],['vram_temp','显存温度'],['default_fan_rpm','默认转速 (RPM)'],['max_fan_rpm','满转速 (RPM)'],['rpm_45db','45分贝转速'],['full_load_power','满载功耗 (W)'],['power_45db','45分贝功耗 (W)'],['temp_limit','温度墙'],['power_limit','功耗墙'],['throttling','降频情况']];
  const mark = [['solar_bay','Solar Bay'],['steel_nomad','Steel Nomad'],['steel_nomad_light','Steel Nomad Light'],['speed_way','Speed Way'],['port_royal','Port Royal'],['time_spy','Time Spy'],['fire_strike','Fire Strike'],['dx_raytracing','DX 光追 (FPS)'],['pcie_bandwidth','PCIE 带宽 (GB/s)']];
  const blender = [['monster','Monster'],['junkshop','Junkshop'],['classroom','Classroom']];
  const games = [['game','游戏'],['resolution','分辨率'],['quality','画质'],['ray_tracing','光线追踪'],['render_mode','渲染模式'],['upscale_tech','超分技术'],['upscale_quality','超分档位'],['frame_gen','帧生成'],['avg_fps','平均帧'],['one_percent_low','1% Low'],['min_fps','最低帧数'],['gpu_temp','显卡温度'],['gpu_usage','GPU占用 (%)'],['cpu_usage','CPU占用 (%)'],['ram_usage','内存占用'],['vram_usage','显存占用'],['power','功耗'],['note','备注']];
  const itemGames = games.filter(([k]) => !['render_mode','upscale_tech','upscale_quality','frame_gen','min_fps','cpu_usage','power'].includes(k));
  const sw = [['graphics','图形'],['processor','处理器'],['io','输入/输出'],['realview','RealView 性能'],['simulation','模拟']];
  const codec = [['codec','编码器'],['resolution','分辨率'],['time_sec','耗时'],['power','功耗'],['gpu_usage','GPU占用'],['cpu_usage','CPU占用'],['note','备注']];
  const gpuGroups = [ ['basic','基本参数',basic], ['cooling','散热与功耗',cooling], ['mark3d','3DMark',mark], ['sw','SolidWorks (SW)',sw], ['blender','Blender',blender] ];
  const gpuRows = [['productivity','生产力测试',[['name','项目'],['score','得分'],['unit','单位'],['note','备注']]],['games','游戏测试',games],['encoding','编码测试',codec],['decoding','解码测试',codec]];
  const schemas = {
    cpu: {name:'model', subtitle:['cores','boost_clock'], groups:[['基本参数',[['model','型号'],['brand','品牌'],['socket','插槽'],['cores','核心数'],['threads','线程数'],['p_cores','大核数'],['e_cores','小核数'],['base_clock','基础频率'],['boost_clock','加速频率'],['l3_cache','L3 缓存'],['tdp','TDP (W)'],['process','制程'],['igpu','核显'],['memory','内存支持'],['test_platform','测试平台'],['bios','BIOS / 微码'],['test_date','测试日期'],['ambient_temp','环境温度']]],['跑分',[['cpuz_single','CPU-Z 单核'],['cpuz_multi','CPU-Z 多核'],['cb23_single','Cinebench R23 单核'],['cb23_multi','Cinebench R23 多核'],['cb24_single','Cinebench 2024 单核'],['cb24_multi','Cinebench 2024 多核'],['geekbench_single','Geekbench 单核'],['geekbench_multi','Geekbench 多核'],['timespy_cpu','3DMark CPU Profile'],['cpu_profile_1t','CPU Profile 1T'],['passmark','PassMark'],['ycruncher','y-cruncher']]],['功耗与温度',[['idle_power','待机功耗 (W)'],['full_power','满载功耗 (W)'],['package_power','封装功耗 (W)'],['temp_idle','待机温度 (℃)'],['temp_full','满载温度 (℃)'],['temp_45db','45分贝温度 (℃)'],['temp_limit','温度墙'],['power_limit','功耗墙'],['throttling','降频情况']]],['备注',[['note','备注']]]], rows:[['power_steps','功耗档位测试',[['power','功耗 (W)'],['multi','多核分数'],['single','单核分数'],['temp','温度'],['note','说明']]],['games','游戏测试',itemGames]] },
    device: {name:'name', subtitle:['type','cpu','gpu'], groups:[['基本信息',[['name','设备名称'],['type','类型'],['brand','品牌'],['model','型号'],['cpu','CPU'],['gpu','显卡'],['ram','内存'],['storage','存储'],['os','系统']]],['性能测试',[...mark.filter(([k]) => k !== 'pcie_bandwidth'),['cinebench_multi','Cinebench 多核'],['cinebench_single','Cinebench 单核'],['geekbench_multi','Geekbench 多核'],['pcmark','PCMark']]],['功耗、温度与续航',[['idle_power','待机功耗 (W)'],['full_power','满载功耗 (W)'],['temp_idle','待机温度 (℃)'],['temp_full','满载温度 (℃)'],['fan_noise','满载噪音 (dB)'],['battery_life','续航 (小时)']]],['备注',[['note','备注']]]], rows:[['games','游戏测试',itemGames]] }
  };
  const suggestions = {game:['三角洲大坝','死亡搁浅2','CS2','LOL大乱斗','永劫无间','2077','红色沙漠','绝区零'],resolution:['1080P','2K','4K','1080P60','4K60','8K','8K60'],quality:['最低','非常低','低','中','中等','高','超高','极高','最高','超级','极致','影视级','电影级','原画','性能','均衡','流畅','便携'],ray_tracing:['开启','无','中','超级','变态'],upscale_tech:['DLSS','FSR','XeSS'],upscale_quality:['性能','平衡'],frame_gen:['X2','X3','X4','X5'],codec:['H.264','H.265/HEVC','HEVC','AV1','VP9','VP8','MPEG-2','MPEG-4','VC-1','MJPEG','剪映H264','剪映HEVC','剪映AV1']};
  let gpus = [], items = [], stats = {}, gameOptions = [], error = '', notice = '', loading = false, busy = false;
  let view = 'list', kind = 'gpu', detail = null, id = null, editing = false, search = '', filter = 'all';
  let gameFilter = '', resolutionFilter = '', qualityFilter = '', media = [], mediaPath = '', newGame = '';
  let selected = [], compareDetails = [], metric = 'mark3d.time_spy', compareGame = '', compareResolution = '', compareField = 'avg_fps';
  const labels = {gpu:'显卡',cpu:'CPU',device:'设备'};
  const tabs = [['all','全部'],['gpu-amd','AMD显卡'],['cpu-amd','AMD CPU'],['gpu-intel','英特尔显卡'],['cpu-intel','英特尔 CPU'],['gpu-nvidia','英伟达显卡'],['device','设备']];
  async function request(path, method = 'GET', body) {
    const res = await fetch('/api/hpahub/hardware' + path, {method, headers:{...(token ? {Authorization:`Bearer ${token}`} : {}), ...(body !== undefined ? {'Content-Type':'application/json'} : {})}, ...(body !== undefined ? {body:JSON.stringify(body)} : {})});
    const text = await res.text(); let data;
    try { data = text ? JSON.parse(text) : null; } catch { data = text; }
    if (!res.ok) throw new Error(data?.error || data?.message || `请求失败（${res.status}）`);
    return data;
  }
  async function load() {
    loading = true; error = '';
    try { const [records,options] = await Promise.all([request('/records'),request('/options/games')]); gameOptions = options; gpus = records.filter(r => r.kind === 'gpu').map(r => ({...r.data.fields,...r,model:r.data.fields?.model || r.name})); items = records.filter(r => r.kind !== 'gpu').map(r => ({...r,fields:r.data.fields})); stats = {games:records.reduce((n,r) => n + (r.data.rows?.games?.length || 0),0)}; }
    catch (e) { error = e.message; } finally { loading = false; }
  }
  onMount(load);
  function vendor(type, d) {
    const s = [d.model,d.name,d.brand,d.fields?.model].filter(Boolean).join(' ').toLowerCase();
    if (type === 'cpu') return /amd|ryzen|锐龙|threadripper|epyc/.test(s) ? 'amd' : /intel|酷睿|core\s*(i[3579]|ultra)|xeon|至强|奔腾|赛扬/.test(s) ? 'intel' : '';
    if (/arc|锐炫|(^|[^a-z0-9])[ab]\d{3}([^0-9]|$)/.test(s)) return 'intel';
    if (/rtx|gtx|quadro|tesla|geforce|(^|[^0-9])(20|30|40|50)\d{2}([^0-9]|$)|(^|[^a-z0-9])a(\d{2}|\d{4})([^0-9]|$)/.test(s)) return 'nvidia';
    return /radeon|vega|rx\s?\d|(^|[^a-z0-9])w\d{4}|(^|[^0-9])7\d{3}m?([^0-9]|$)/.test(s) ? 'amd' : '';
  }
  $: all = [...gpus.map(data => ({type:'gpu',data})), ...items.map(data => ({type:data.kind,data}))];
  $: shown = all.filter(({type,data}) => (filter === 'all' || filter === type || filter === `${type}-${vendor(type,data)}`) && JSON.stringify(data).toLowerCase().includes(search.trim().toLowerCase()));
  function blank(k) {
    return normalize({kind:k,name:'',brand:'',subtitle:'',sort_order:0,data:{fields:{},sections:{},rows:{},custom:{}}},k);
  }
  function normalize(record,k) {
    const data = record.data || {};
    const d = {...record,fields:data.fields || {},rows:data.rows || {}};
    if (k === 'gpu') {
      d.gpu = d.fields;
      for (const [key] of gpuGroups) if (key !== 'basic') d[key] = data.sections?.[key] || {};
      d.custom = data.custom || {}; for (const key of ['basic','cooling','mark3d']) { d.custom[key] ||= {}; d.custom[key].hidden ||= []; d.custom[key].extra ||= []; }
      for (const [key] of gpuRows) d[key] = d.rows[key] || [];
    } else { for (const [key] of schemas[k].rows) d.rows[key] ||= []; }
    return d;
  }
  async function open(k, recordId, edit = false) {
    kind = k; id = recordId; editing = edit; error = ''; notice = ''; loading = true; gameFilter = resolutionFilter = qualityFilter = ''; media = [];
    try {
      detail = recordId ? normalize(await request(`/records/${recordId}`),k) : blank(k);
      view = 'detail';
      if (recordId && canEdit) media = (await request(`/records/${recordId}/media`)).paths || [];
    } catch (e) { error = e.message; } finally { loading = false; }
  }
  async function save() {
    if (!canEdit || busy) return; busy = true; error = ''; notice = '';
    try {
      if (kind !== 'gpu') { const sc = schemas[kind]; detail.name = detail.fields[sc.name] || ''; detail.brand = detail.fields.brand || ''; detail.subtitle = sc.subtitle.map(k => detail.fields[k]).filter(Boolean).join(' · '); }
      else { detail.name = detail.gpu.model || ''; detail.brand = detail.gpu.brand || ''; }
      const data = {...detail.data,fields:detail.fields,rows:kind === 'gpu' ? {...detail.rows,...Object.fromEntries(gpuRows.map(([key]) => [key,detail[key]]))} : detail.rows};
      if (kind === 'gpu') { data.sections = {...data.sections,...Object.fromEntries(gpuGroups.filter(([key]) => key !== 'basic').map(([key]) => [key,detail[key]]))}; data.custom = detail.custom; }
      const result = await request(`/records${id ? '/'+id : ''}`, id ? 'PUT' : 'POST', {kind,name:detail.name,brand:detail.brand,subtitle:detail.subtitle,sort_order:detail.sort_order,data});
      id = result.id || id; notice = '已保存';
    } catch (e) { error = e.message; } finally { busy = false; }
  }
  async function action(actionName) {
    if (!canEdit || !id || busy) return;
    if (actionName === 'delete' && !confirm('确定删除此记录？此操作无法撤销。')) return;
    busy = true; error = '';
    try {
      const path = `/records/${id}`;
      if (actionName === 'delete') { await request(path,'DELETE'); view = 'list'; await load(); }
      else { const result = await request(path + '/copy','POST'); await open(kind,result.id); }
    } catch (e) { error = e.message; } finally { busy = false; }
  }
  async function reorder(gpuId, delta) {
    const next = [...gpus], index = next.findIndex(x => x.id === gpuId), target = index + delta;
    if (index < 0 || target < 0 || target >= next.length) return;
    [next[index],next[target]] = [next[target],next[index]];
    try { await request('/reorder','POST',{ids:next.map(x => x.id)}); gpus = next; } catch (e) { error = e.message; }
  }
  function addRow(container,key) { container[key] = [...container[key],{}]; detail = detail; }
  function removeRow(container,key,index) { container[key] = container[key].filter((_,i) => i !== index); detail = detail; }
  function addCustom(key) { const label = prompt('项目名称'); if (label?.trim()) { detail.custom[key].extra = [...detail.custom[key].extra,{key:'c_'+Date.now().toString(36)+Math.random().toString(36).slice(2,6),label:label.trim(),value:''}]; detail = detail; } }
  function hideField(group,key) { detail[group === 'basic' ? 'gpu' : group][key] = ''; detail.custom[group].hidden = [...detail.custom[group].hidden,key]; detail = detail; }
  async function addGame() { if (!newGame.trim()) return; try { await request('/options/games','POST',{value:newGame.trim()}); gameOptions = await request('/options/games'); newGame = ''; notice = '游戏选项已添加'; } catch (e) { error = e.message; } }
  async function changeMedia(actionName,path) { if (!id || (actionName === 'add' && !path.trim())) return; try { const paths = actionName === 'add' ? (media.includes(path.trim()) ? media : [...media,path.trim()]) : media.filter(p => p !== path); media = (await request(`/records/${id}/media`,'PUT',{paths})).paths; mediaPath = ''; } catch (e) { error = e.message; } }
  function mediaUrl(path) { if (typeof path !== 'string') return ''; try { const url = new URL(path, location.origin); return ['http:','https:'].includes(url.protocol) ? url.href : ''; } catch { return ''; } }
  const unique = values => [...new Set(values.map(v => String(v ?? '')))];
  $: gameRows = detail ? (kind === 'gpu' ? detail.games : detail.rows?.games) || [] : [];
  $: resolutions = unique(gameRows.filter(r => !gameFilter || r.game === gameFilter).map(r => r.resolution));
  $: qualities = unique(gameRows.filter(r => (!gameFilter || r.game === gameFilter) && (!resolutionFilter || r.resolution === resolutionFilter)).map(r => r.quality));
  $: filteredGames = gameRows.filter(r => (!gameFilter || r.game === gameFilter) && (!resolutionFilter || r.resolution === resolutionFilter) && (!qualityFilter || r.quality === qualityFilter));
  async function compare() { if (selected.length < 2) { error = '至少选择两张显卡'; return; } loading = true; error = ''; try { compareDetails = (await Promise.all(selected.map(i => request(`/records/${i}`)))).map(d => normalize(d,'gpu')); view = 'compare'; } catch (e) { error = e.message; } finally { loading = false; } }
  $: compareGames = unique(compareDetails.flatMap(d => (d.games || []).map(r => r.game))).filter(Boolean);
  $: compareResolutions = unique(compareDetails.flatMap(d => (d.games || []).filter(r => r.game === compareGame).map(r => r.resolution)));
  function numeric(v) { const m = String(v ?? '').match(/\d+(?:\.\d+)?/g); return !m ? null : m.length > 1 && /[-~～]/.test(v) ? (Number(m[0])+Number(m[1]))/2 : Number(m[0]); }
  function compareValue(d) {
    if (metric === 'games') { const rows = (d.games || []).filter(r => r.game === compareGame && String(r.resolution || '') === compareResolution && numeric(r[compareField]) !== null); rows.sort((a,b) => numeric(b[compareField])-numeric(a[compareField])); return rows[0]?.[compareField] ?? ''; }
    if (metric.startsWith('productivity.')) return (d.productivity || []).find(r => r.name === metric.slice(13))?.score ?? '';
    if (metric.startsWith('extra.')) return d.custom?.mark3d?.extra?.find(r => r.label === metric.slice(6))?.value ?? '';
    const [group,key] = metric.split('.'); return d[group]?.[key] ?? '';
  }
  $: metricOptions = [...gpuGroups.flatMap(([group,title,fields]) => fields.map(([key,label]) => [`${group === 'basic' ? 'gpu' : group}.${key}`,`${title} · ${label}`])), ...unique(compareDetails.flatMap(d => (d.productivity || []).map(r => r.name))).filter(Boolean).map(n => ['productivity.'+n,'生产力 · '+n]), ...unique(compareDetails.flatMap(d => (d.custom?.mark3d?.extra || []).map(r => r.label))).filter(Boolean).map(n => ['extra.'+n,'3DMark · '+n]), ['games','游戏帧数']];
</script>

<div class="gpu-page">
  <header class="heading"><div><h1>硬件数据库</h1><p>浏览显卡、CPU 与设备的实测数据。</p></div>{#if view !== 'list'}<button on:click={() => { view = 'list'; editing = false; load(); }}>返回列表</button>{/if}</header>
  {#if error}<div class="message error" role="alert">{error}</div>{/if}
  {#if notice}<div class="message" role="status">{notice}</div>{/if}
  {#if loading}<p role="status">正在加载…</p>{/if}
  {#if view === 'list'}
    <div class="summary">显卡 {gpus.length}　·　CPU {items.filter(x => x.kind === 'cpu').length}　·　设备 {items.filter(x => x.kind === 'device').length}　·　游戏记录 {stats.games || 0}</div>
    <nav class="tabs" aria-label="硬件分类">{#each tabs as [key,label]}<button class:active={filter === key} on:click={() => filter = key}>{label}</button>{/each}</nav>
    <div class="toolbar"><label class="search">搜索<input type="search" bind:value={search} placeholder="型号、品牌或参数" /></label><span>{shown.length} 条记录</span><button on:click={compare}>对比所选显卡（{selected.length}）</button>{#if canEdit}{#each ['gpu','cpu','device'] as k}<button on:click={() => open(k,null,true)}>新增{labels[k]}</button>{/each}{/if}</div>
    {#if !loading && !shown.length}<p class="empty">没有匹配的记录。</p>{/if}
    <div class="list">{#each shown as {type,data} (type + '-' + data.id)}<article class="record"><div><span class="badge">{labels[type] || type}</span><button class="record-title" on:click={() => open(type,data.id)}>{type === 'gpu' ? data.model || '未命名显卡' : data.name || '未命名' + labels[type]}</button><p>{type === 'gpu' ? [data.brand,data.vram_size].filter(Boolean).join(' · ') : data.subtitle || ''}</p></div><div class="record-actions">{#if type === 'gpu'}<label><input type="checkbox" value={data.id} bind:group={selected} /> 对比</label>{#if canEdit}<button aria-label="上移显卡" on:click={() => reorder(data.id,-1)}>上移</button><button aria-label="下移显卡" on:click={() => reorder(data.id,1)}>下移</button>{/if}{/if}</div></article>{/each}</div>
  {:else if view === 'compare'}
    <section class="panel"><h2>显卡数据对比</h2><div class="toolbar"><label>对比项目<select bind:value={metric}>{#each metricOptions as [value,label]}<option {value}>{label}</option>{/each}</select></label>{#if metric === 'games'}<label>游戏<select bind:value={compareGame} on:change={() => compareResolution = ''}><option value="">选择游戏</option>{#each compareGames as g}<option value={g}>{g}</option>{/each}</select></label><label>分辨率<select bind:value={compareResolution}>{#each compareResolutions as r}<option value={r}>{r || '未填'}</option>{/each}</select></label><label>帧数指标<select bind:value={compareField}><option value="avg_fps">平均帧</option><option value="one_percent_low">1% Low</option><option value="min_fps">最低帧数</option></select></label>{/if}</div><div class="comparison">{#each compareDetails as d}<div><strong>{d.gpu?.model || '未命名显卡'}</strong><span>{compareValue(d) || '—'}</span></div>{/each}</div><p class="hint">同一游戏和分辨率有多组设置时，显示所选指标的最高值。</p></section>
  {:else if detail}
    <div class="toolbar detail-actions"><h2>{editing ? (id ? '编辑' : '新增') : ''}{labels[kind]} · {kind === 'gpu' ? detail.gpu.model || '未命名' : detail.fields[schemas[kind].name] || detail.name || '未命名'}</h2>{#if canEdit}{#if editing}<button class="primary" disabled={busy} on:click={save}>保存</button><button on:click={() => id ? open(kind,id) : (view = 'list')}>取消</button>{:else}<button class="primary" on:click={() => editing = true}>编辑</button><button disabled={busy} on:click={() => action('copy')}>复制一份</button><button class="danger" disabled={busy} on:click={() => action('delete')}>删除</button>{/if}{/if}</div>
    {#if kind === 'gpu'}
      {#each gpuGroups as [group,title,fields]}
        {@const obj = detail[group === 'basic' ? 'gpu' : group]}
        {@const custom = detail.custom[group]}
        <section class="panel"><h3>{title}</h3><div class="fields">{#each fields as [key,label]}{#if !custom?.hidden?.includes(key)}<div class="field"><label>{label}{#if editing}<input bind:value={detail[group === 'basic' ? 'gpu' : group][key]} />{:else}<span>{obj[key] || '—'}</span>{/if}</label>{#if editing && custom}<button class="small" aria-label={'删除' + label} on:click={() => hideField(group,key)}>删除</button>{/if}</div>{/if}{/each}{#each custom?.extra || [] as extra, index}<div class="field"><label>{extra.label || extra.key}{#if editing}<input value={extra.value} on:input={(event) => { detail.custom[group].extra[index].value = event.currentTarget.value; }} />{:else}<span>{extra.value || '—'}</span>{/if}</label>{#if editing}<button class="small" on:click={() => { detail.custom[group].extra = custom.extra.filter((_,i) => i !== index); }}>删除</button>{/if}</div>{/each}</div>{#if editing && custom}<div class="toolbar"><button on:click={() => addCustom(group)}>添加项目</button>{#if custom.hidden.length}<button on:click={() => { detail.custom[group].hidden = []; }}>恢复内置项目</button>{/if}</div>{/if}{#if group === 'basic'}<label class="note">备注{#if editing}<textarea bind:value={detail.gpu.note}></textarea>{:else}<span>{detail.gpu.note || '—'}</span>{/if}</label>{/if}</section>
      {/each}
    {:else}
      {#each schemas[kind].groups as [title,fields]}<section class="panel"><h3>{title}</h3><div class="fields">{#each fields as [key,label]}<label>{label}{#if editing}<input bind:value={detail.fields[key]} />{:else}<span>{detail.fields[key] || '—'}</span>{/if}</label>{/each}</div></section>{/each}
    {/if}
    {#each kind === 'gpu' ? gpuRows : schemas[kind].rows as [key,title,columns]}
      {@const container = kind === 'gpu' ? detail : detail.rows}
      {@const rows = key === 'games' && !editing ? filteredGames : container[key]}
      <section class="panel"><h3>{title} <small>{rows.length} 条</small></h3>
        {#if key === 'games' && !editing}<div class="toolbar"><label>游戏<select bind:value={gameFilter} on:change={() => { resolutionFilter = ''; qualityFilter = ''; }}><option value="">全部</option>{#each unique(gameRows.map(r => r.game)).filter(Boolean) as g}<option value={g}>{g}</option>{/each}</select></label><label>分辨率<select bind:value={resolutionFilter} on:change={() => qualityFilter = ''}><option value="">全部</option>{#each resolutions.filter(Boolean) as r}<option value={r}>{r}</option>{/each}</select></label><label>画质<select bind:value={qualityFilter}><option value="">全部</option>{#each qualities.filter(Boolean) as q}<option value={q}>{q}</option>{/each}</select></label></div>{/if}
        <div class="table-wrap"><table><thead><tr>{#each columns as [,label]}<th>{label}</th>{/each}{#if editing}<th>操作</th>{/if}</tr></thead><tbody>{#each rows as row,index}<tr>{#each columns as [column,label]}<td>{#if editing}<input aria-label={label} list={suggestions[column] ? 'gpu-option-'+column : undefined} value={row[column]} on:input={(event) => { (kind === 'gpu' ? detail : detail.rows)[key][index][column] = event.currentTarget.value; detail = detail; }} />{:else}{row[column] || '—'}{/if}</td>{/each}{#if editing}<td><button on:click={() => removeRow(container,key,index)}>删除</button></td>{/if}</tr>{/each}</tbody></table></div>{#if !rows.length}<p class="empty">暂无数据</p>{/if}{#if editing}<button on:click={() => addRow(container,key)}>添加一行</button>{/if}
      </section>
    {/each}
    {#if editing}<section class="panel"><h3>游戏选项</h3><div class="toolbar"><label>新游戏名称<input bind:value={newGame} /></label><button on:click={addGame}>添加游戏选项</button></div></section>{/if}
    {#if id && canEdit}<section class="panel"><h3>相关媒体</h3>{#if media.length}<ul>{#each media as path}<li>{#if mediaUrl(path)}<a href={mediaUrl(path)} target="_blank" rel="noopener noreferrer">{path}</a>{:else}{typeof path === 'string' ? path : JSON.stringify(path)}{/if}<button class="small" on:click={() => changeMedia('remove',path)}>移除</button></li>{/each}</ul>{:else}<p class="empty">暂无相关媒体</p>{/if}<div class="toolbar"><label>媒体路径或链接<input bind:value={mediaPath} /></label><button on:click={() => changeMedia('add',mediaPath)}>添加媒体</button></div></section>{/if}
    {#if editing}<button class="primary" disabled={busy} on:click={save}>保存所有更改</button>{/if}
  {/if}
  {#each Object.entries(suggestions) as [key,values]}<datalist id={'gpu-option-'+key}>{#each key === 'game' ? unique([...values,...gameOptions]) : values as value}<option {value}></option>{/each}</datalist>{/each}
</div>

<style>
  .gpu-page{box-sizing:border-box;width:calc(100% - 2rem)}
  .gpu-page{color:#253a30;max-width:1400px;margin:auto;padding:1.5rem 0 3rem}.heading,.toolbar,.detail-actions,.record,.record-actions{display:flex;align-items:center;gap:.75rem;flex-wrap:wrap}.heading{justify-content:space-between;margin-bottom:1.5rem}.heading h1,.heading p,.record p{margin:0}.heading p,.hint,.empty,.record p{color:#5d7065}.heading p{margin-top:.4rem}.summary{padding:1rem 0;border-top:1px solid #d9e3dc;border-bottom:1px solid #d9e3dc;color:#52695a}.tabs{display:flex;gap:.4rem;overflow-x:auto;margin:1.2rem 0}.tabs button{white-space:nowrap}.tabs .active,.primary{background:#315d43;color:white;border-color:#315d43}.toolbar{margin:1rem 0}.toolbar h2{margin:0 auto 0 0}.toolbar label,.search{display:flex;align-items:center;gap:.5rem}.search input{min-width:15rem}.list{border-top:1px solid #d9e3dc}.record{justify-content:space-between;padding:1rem 0;border-bottom:1px solid #d9e3dc}.record-title{display:block;border:0;padding:.35rem 0;background:none;font-size:1.1rem;font-weight:700;text-align:left;color:#203d2c}.badge{font-size:.75rem;font-weight:700;color:#426f51}.record p{font-size:.9rem}.panel{border:1px solid #d9e3dc;background:#fff;border-radius:.5rem;padding:1.1rem;margin:1rem 0}.panel h3{margin:0 0 1rem;font-size:1rem}.panel h3 small{font-weight:400;color:#5d7065}.fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem}.fields label,.note{display:flex;flex-direction:column;gap:.35rem;color:#52695a;font-size:.85rem}.fields label span,.note span{font-size:1rem;color:#20372a;overflow-wrap:anywhere}.field{display:flex;align-items:end;gap:.3rem}.field label{flex:1;min-width:0}.note{margin-top:1rem}input,select,textarea,button{font:inherit}input:not([type=checkbox]),select,textarea{box-sizing:border-box;border:1px solid #bbcbbf;border-radius:.3rem;background:white;color:#20372a;padding:.55rem;max-width:100%}textarea{min-height:5rem}button{cursor:pointer;border:1px solid #b7c9bc;border-radius:.3rem;background:#fff;color:#294d35;padding:.5rem .7rem}button:hover{border-color:#315d43}button:focus-visible,input:focus-visible,select:focus-visible,textarea:focus-visible,a:focus-visible{outline:3px solid #7caa88;outline-offset:2px}button:disabled{opacity:.55;cursor:wait}.primary{background:#315d43;color:#fff}.danger{color:#a03232}.small{font-size:.75rem;padding:.3rem}.table-wrap{overflow-x:auto}table{border-collapse:collapse;width:100%;font-size:.85rem}th,td{border-bottom:1px solid #e1e9e3;padding:.6rem;text-align:left;white-space:nowrap}th{color:#52695a}td input{width:9rem}.empty{font-size:.9rem}.message{padding:.8rem;border:1px solid #aac9b2;background:#eff8f1;margin:1rem 0}.error{border-color:#e2b6b6;background:#fff1f1;color:#8e2929}.comparison{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:.75rem}.comparison div{display:flex;flex-direction:column;gap:.5rem;border:1px solid #d9e3dc;padding:1rem}.comparison span{font-size:1.4rem;font-weight:700}.hint{font-size:.85rem}li{margin:.5rem 0;overflow-wrap:anywhere}li button{margin-left:.5rem}@media(max-width:600px){.gpu-page{padding:1rem 0 2rem}.toolbar{align-items:stretch}.toolbar label{width:100%;justify-content:space-between}.toolbar label input,.toolbar label select{flex:1}.fields{grid-template-columns:repeat(2,minmax(0,1fr))}.record-actions{width:100%}}
</style>
