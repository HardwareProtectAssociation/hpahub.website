<script>
  import { onMount } from 'svelte';

  export let token = '';
  export let role = 'guest';

  const base = '/api/hpahub/warehouse';
  const categories = { images: '图片', videos: '视频', docs: '文档', others: '其他' };
  const textExtensions = ['txt', 'csv', 'json', 'md', 'log', 'xml', 'html', 'css', 'js', 'py', 'ini', 'yaml', 'yml', 'sql', 'conf', 'sh', 'bat'];
  let projects = [];
  let storage = { disks: [], categories: {}, allocations: {} };
  let project = '';
  let files = {};
  let filter = 'all';
  let search = '';
  let selected = [];
  let pending = [];
  let preview = null;
  let previewText = '';
  let allocations = {};
  let editingStorage = false;
  let loading = false;
  let busy = false;
  let progress = 0;
  let error = '';
  let notice = '';
  let mediaReady = false;
  let timer;
  $: allowed = ['member', 'admin', 'octopus'].includes(role);
  $: allFiles = Object.keys(categories).flatMap(key => files[key] || []);
  $: visibleFiles = allFiles.filter(file => (filter === 'all' || file.category === filter) && file.name.toLowerCase().includes(search.trim().toLowerCase()));
  $: selectedFiles = allFiles.filter(file => selected.includes(fileKey(file)));

  function fileKey(file) { return `${file.category}/${file.name}`; }
  function pathPart(value) { return encodeURIComponent(value); }
  function projectPath(name = project) { return `/projects/${pathPart(name)}`; }
  function filePath(file) { return `${projectPath()}/files/${pathPart(file.category)}/${pathPart(file.name)}`; }
  function mediaUrl(kind, file) { return `${base}/media/${kind}/${pathPart(project)}/${pathPart(file.category)}/${pathPart(file.name)}`; }
  function size(bytes) {
    if (!bytes) return '0 B';
    const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 4);
    return `${(bytes / 1024 ** unit).toFixed(unit ? 1 : 0)} ${['B', 'KB', 'MB', 'GB', 'TB'][unit]}`;
  }
  async function api(path, options = {}) {
    const response = await fetch(base + path, {
      ...options,
      headers: { Authorization: `Bearer ${token}`, ...(options.body && !(options.body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}), ...options.headers },
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok || data.ok === false) throw new Error(data.error || data.message || `请求失败（${response.status}）`);
    return data;
  }
  async function issueMediaSession() {
    try {
      await api('/media-session', { method: 'POST' });
      mediaReady = true;
    } catch (err) {
      mediaReady = false;
      error = `媒体会话创建失败：${err.message}`;
    }
  }
  async function loadProjects() {
    const [list, capacity] = await Promise.all([api('/projects'), api('/storage')]);
    projects = list.projects || [];
    storage = capacity;
  }
  async function loadFiles() {
    const name = project;
    const data = await api(projectPath(name));
    if (project === name) files = data.files || {};
  }
  async function refresh() {
    loading = true;
    error = '';
    try { await loadProjects(); if (project) await loadFiles(); }
    catch (err) { error = err.message; }
    finally { loading = false; }
  }
  async function openProject(name) {
    project = name;
    files = {};
    selected = [];
    filter = 'all';
    search = '';
    preview = null;
    await refresh();
  }
  async function createProject() {
    const name = prompt('请输入项目名称');
    if (!name || !name.trim()) return;
    await mutate(async () => { await api('/projects', { method: 'POST', body: JSON.stringify({ name: name.trim() }) }); await loadProjects(); }, '项目已创建');
  }
  async function renameProject(name) {
    const next = prompt('请输入新项目名称', name);
    if (!next || !next.trim() || next.trim() === name) return;
    await mutate(async () => {
      const result = await api(projectPath(name), { method: 'PUT', body: JSON.stringify({ name: next.trim() }) });
      if (project === name) project = result.project.name;
      await loadProjects();
    }, '项目已重命名');
  }
  async function deleteProject(name) {
    if (!confirm(`确定删除项目「${name}」及其所有文件吗？此操作不可恢复。`)) return;
    await mutate(async () => { await api(projectPath(name), { method: 'DELETE' }); if (project === name) { project = ''; preview = null; } await loadProjects(); }, '项目已删除');
  }
  async function mutate(action, message) {
    busy = true; error = ''; notice = '';
    try { await action(); notice = message; }
    catch (err) { error = err.message; }
    finally { busy = false; }
  }
  async function renameFile(file) {
    const name = prompt('请输入新文件名', file.name);
    if (!name || !name.trim() || name.trim() === file.name) return;
    await mutate(async () => { await api(filePath(file), { method: 'PUT', body: JSON.stringify({ name: name.trim() }) }); preview = null; selected = []; await loadFiles(); }, '文件已重命名');
  }
  async function deleteFiles(items) {
    if (!items.length || !confirm(`确定删除选中的 ${items.length} 个文件吗？此操作不可恢复。`)) return;
    busy = true; error = ''; notice = '';
    const results = await Promise.allSettled(items.map(file => api(filePath(file), { method: 'DELETE' })));
    const failed = results.filter(result => result.status === 'rejected');
    try { await Promise.all([loadFiles(), loadProjects()]); }
    catch (err) { error = err.message; }
    selected = []; preview = null; busy = false;
    if (failed.length) error = `删除失败 ${failed.length} 个：${failed[0].reason.message}`;
    else notice = `已删除 ${items.length} 个文件`;
  }
  function chooseFiles(list) { pending = Array.from(list).map(file => ({ file, name: file.name })); }
  function upload() {
    if (!pending.length) return;
    const data = new FormData();
    for (const item of pending) { data.append('files', item.file, item.file.name); data.append('names', item.name); }
    busy = true; progress = 0; error = ''; notice = '';
    const xhr = new XMLHttpRequest();
    xhr.open('POST', base + projectPath() + '/files');
    xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    xhr.upload.onprogress = event => { if (event.lengthComputable) progress = Math.round(event.loaded / event.total * 100); };
    xhr.onload = async () => {
      let result = {};
      try { result = JSON.parse(xhr.responseText); } catch (_) { /* Display HTTP error below. */ }
      if (xhr.status >= 200 && xhr.status < 300 && result.ok !== false) {
        pending = []; notice = `已上传 ${(result.files || []).length} 个文件`;
        try { await Promise.all([loadFiles(), loadProjects()]); } catch (err) { error = err.message; }
      } else error = result.error || result.message || `上传失败（${xhr.status}）`;
      busy = false;
    };
    xhr.onerror = () => { error = '上传失败：网络错误'; busy = false; };
    xhr.send(data);
  }
  function toggle(file) {
    const key = fileKey(file);
    selected = selected.includes(key) ? selected.filter(item => item !== key) : [...selected, key];
  }
  function download(file) {
    const link = document.createElement('a');
    link.href = mediaUrl('file', file); link.download = file.name; link.click();
  }
  async function openPreview(file) {
    preview = file; previewText = '';
    const ext = file.name.split('.').pop().toLowerCase();
    if (textExtensions.includes(ext)) {
      previewText = '加载中…';
      try {
        const response = await fetch(mediaUrl('file', file));
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const text = await response.text();
        if (preview === file) previewText = text.slice(0, 500000);
      } catch (err) { if (preview === file) previewText = `加载失败：${err.message}`; }
    }
  }
  function editAllocations() {
    allocations = Object.fromEntries(storage.disks.map(disk => [disk.mountpoint, (storage.allocations[disk.mountpoint] || 0) / 1073741824]));
    editingStorage = true;
  }
  async function saveAllocations() {
    const values = Object.fromEntries(Object.entries(allocations).filter(([, value]) => Number(value) > 0).map(([mount, value]) => [mount, Math.round(Number(value) * 1073741824)]));
    await mutate(async () => { await api('/storage', { method: 'PUT', body: JSON.stringify({ allocations: values }) }); editingStorage = false; await loadProjects(); }, '容量分配已保存');
  }
  onMount(() => {
    if (allowed) {
      refresh();
      issueMediaSession();
      timer = setInterval(issueMediaSession, 5 * 60 * 1000);
    }
    return () => clearInterval(timer);
  });
</script>

<section class="warehouse">
  <header class="heading">
    <div><h1>测试数据储存仓库</h1><p>管理项目、测试文件和存储容量</p></div>
    {#if allowed}<button on:click={refresh} disabled={loading || busy}>刷新</button>{/if}
  </header>
  {#if !allowed}
    <p class="empty">此页面仅供内部成员访问。</p>
  {:else}
    {#if error}<p class="message error" role="alert">{error}</p>{/if}
    {#if notice}<p class="message" role="status">{notice}</p>{/if}
    {#if !project}
      <section class="capacity" aria-label="存储容量">
        <div class="section-heading"><h2>存储容量</h2>{#if role === 'octopus'}<button on:click={editAllocations} disabled={busy}>容量分配</button>{/if}</div>
        {#each storage.disks || [] as disk}
          <div class="disk"><div class="disk-label"><strong>{disk.mountpoint} · {disk.device}</strong><span>已用 {size(disk.used)} / {size(disk.total)} · 可用 {size(disk.free)}{storage.allocations[disk.mountpoint] ? ` · 分配 ${size(storage.allocations[disk.mountpoint])}` : ''}</span></div><progress max={disk.total || 1} value={disk.used}>已用 {size(disk.used)}</progress></div>
        {:else}<p class="muted">暂无容量信息</p>{/each}
        <p class="muted">仓库分类用量：{Object.entries(categories).map(([key, label]) => `${label} ${size(storage.categories[key])}`).join(' · ')}</p>
        {#if editingStorage}
          <form class="allocation" on:submit|preventDefault={saveAllocations}>
            <h3>容量分配</h3><p class="muted">单位 GB；0 表示不限制。</p>
            {#each storage.disks as disk}
              <label>{disk.mountpoint}（可用 {size(disk.free)}）<input type="number" min="0" max={Math.floor(disk.free / 1073741824)} step="1" bind:value={allocations[disk.mountpoint]} /></label>
            {/each}
            <div class="actions"><button type="submit" class="primary" disabled={busy}>保存</button><button type="button" on:click={() => editingStorage = false}>取消</button></div>
          </form>
        {/if}
      </section>
      <div class="section-heading"><h2>项目 <span class="muted">{projects.length}</span></h2><button class="primary" on:click={createProject} disabled={busy}>新建项目</button></div>
      <div class="project-list">
        {#each projects as item}
          <article class="project-card">
            <button class="project-open" on:click={() => openProject(item.name)}><strong>{item.display_name || item.name}</strong><span>{item.total_files} 个文件 · {size(item.size_bytes)}{item.creator_name ? ` · ${item.creator_name}` : ''}</span><span>{Object.entries(categories).map(([key, label]) => `${label} ${item.counts[key] || 0}`).join(' · ')}</span></button>
            <div class="actions"><button on:click={() => renameProject(item.name)} disabled={busy}>重命名</button><button on:click={() => deleteProject(item.name)} disabled={busy}>删除</button></div>
          </article>
        {:else}<p class="empty">{loading ? '正在加载项目…' : '暂无项目，请创建项目。'}</p>{/each}
      </div>
    {:else}
      <div class="section-heading"><div><button on:click={() => { project = ''; preview = null; selected = []; refresh(); }}>返回项目</button><h2 class="project-title">{project}</h2></div><div class="actions"><button on:click={() => renameProject(project)} disabled={busy}>重命名项目</button><button on:click={() => deleteProject(project)} disabled={busy}>删除项目</button></div></div>
      <div class="upload" on:dragover|preventDefault on:drop|preventDefault={(event) => chooseFiles(event.dataTransfer.files)} role="group" aria-label="上传文件">
        <label>选择或拖放文件上传<input type="file" multiple on:change={(event) => { chooseFiles(event.currentTarget.files); event.currentTarget.value = ''; }} disabled={busy} /></label>
        <p class="muted">文件按扩展名自动分类。</p>
      </div>
      {#if pending.length}
        <form class="pending" on:submit|preventDefault={upload}>
          <h3>待上传文件</h3>
          {#each pending as item, index}
            <div class="pending-row"><label>文件名 <input required maxlength="255" bind:value={item.name} /></label><span>{size(item.file.size)}</span><button type="button" disabled={busy} on:click={() => pending = pending.filter((_, i) => i !== index)}>移除</button></div>
          {/each}
          {#if busy}<progress max="100" value={progress}>上传 {progress}%</progress><span>上传 {progress}%</span>{/if}
          <div class="actions"><button class="primary" disabled={busy} type="submit">上传 {pending.length} 个文件</button><button type="button" disabled={busy} on:click={() => pending = []}>取消</button></div>
        </form>
      {/if}
      <div class="filters"><button class:active={filter === 'all'} on:click={() => filter = 'all'}>全部 {allFiles.length}</button>{#each Object.entries(categories) as [key, label]}<button class:active={filter === key} on:click={() => filter = key}>{label} {(files[key] || []).length}</button>{/each}<label class="search">搜索文件 <input type="search" bind:value={search} /></label></div>
      <div class="batch"><label><input type="checkbox" checked={visibleFiles.length > 0 && visibleFiles.every(file => selected.includes(fileKey(file)))} on:change={(event) => { const keys = visibleFiles.map(fileKey); selected = event.currentTarget.checked ? [...new Set([...selected, ...keys])] : selected.filter(key => !keys.includes(key)); }} /> 全选当前结果</label><span>已选 {selected.length} 个</span><button disabled={!selected.length || !mediaReady} on:click={() => selectedFiles.forEach(download)}>批量下载</button><button disabled={!selected.length || busy} on:click={() => deleteFiles(selectedFiles)}>批量删除</button><button disabled={!selected.length} on:click={() => selected = []}>清空选择</button></div>
      <div class="file-list">
        {#each visibleFiles as file (fileKey(file))}
          <article class="file-card"><label class="select"><input type="checkbox" checked={selected.includes(fileKey(file))} on:change={() => toggle(file)} aria-label={`选择 ${file.name}`} /></label><button class="file-preview" on:click={() => openPreview(file)} disabled={!mediaReady} aria-label={`预览 ${file.name}`}>
            {#if mediaReady && file.category === 'images'}<img src={mediaUrl('file', file)} alt="" loading="lazy" />{:else if mediaReady && file.category === 'videos' && file.thumb}<img src={mediaUrl('thumb', file)} alt="" loading="lazy" />{:else}<span>{categories[file.category]}</span>{/if}
          </button><div class="file-info"><strong title={file.name}>{file.name}</strong><span>{categories[file.category]} · {size(file.size)}</span></div><div class="actions"><button on:click={() => download(file)} disabled={!mediaReady}>下载</button><button on:click={() => renameFile(file)} disabled={busy}>重命名</button><button on:click={() => deleteFiles([file])} disabled={busy}>删除</button></div></article>
        {:else}<p class="empty">{loading ? '正在加载文件…' : '没有符合条件的文件。'}</p>{/each}
      </div>
    {/if}
    {#if preview}
      <div class="preview" role="dialog" aria-modal="true" aria-label={`预览 ${preview.name}`}><div class="preview-head"><strong>{preview.name}</strong><div class="actions"><button on:click={() => download(preview)}>下载</button><button on:click={() => preview = null}>关闭预览</button></div></div><div class="preview-body">
        {#if preview.category === 'images'}<img src={mediaUrl('file', preview)} alt={preview.name} />
        {:else if preview.category === 'videos'}<video controls playsinline src={mediaUrl(preview.web_url ? 'web' : preview.preview_url ? 'preview' : 'file', preview)}></video>
        {:else if textExtensions.includes(preview.name.split('.').pop().toLowerCase())}<pre>{previewText}</pre>
        {:else}<p>此文件无法预览，请下载后查看。</p>{/if}
      </div></div>
    {/if}
  {/if}
</section>

<style>
  .warehouse { color: #252b26; max-width: 1500px; margin: auto; padding: 24px; }
  h1, h2, h3, p { margin: 0; } h1 { font-size: 1.6rem; } h2 { font-size: 1.2rem; } h3 { font-size: 1rem; }
  .heading, .section-heading, .disk-label, .actions, .filters, .batch, .preview-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .heading { margin-bottom: 26px; } .heading p, .muted { color: #56655a; font-size: .88rem; } .heading p { margin-top: 5px; }
  button, input { font: inherit; } button { color: #263a2c; background: #f8f8f3; border: 1px solid #b8c5b2; border-radius: 2px; padding: 8px 12px; cursor: pointer; } button:hover:not(:disabled), button.active { background: #e2ecda; border-color: #739265; } button:disabled { opacity: .5; cursor: not-allowed; } button:focus-visible, input:focus-visible { outline: 3px solid #739265; outline-offset: 2px; } .primary, button.active { background: #537451; color: #fff; border-color: #537451; } .primary:hover:not(:disabled), button.active:hover:not(:disabled) { background: #3e603e; color: #fff; }
  input:not([type='checkbox']):not([type='file']) { background: #fff; color: #252b26; border: 1px solid #aab9a5; border-radius: 2px; padding: 8px; } input[type='checkbox'] { accent-color: #537451; }
  .capacity, .upload, .pending, .allocation { background: #f8f8f3; border: 1px solid #d0d8cc; padding: 18px; margin-bottom: 24px; } .capacity .section-heading { margin-bottom: 14px; } .disk { margin: 12px 0; } .disk-label { font-size: .88rem; margin-bottom: 5px; } progress { display: block; width: 100%; height: 10px; accent-color: #739265; } .allocation { margin: 18px 0 0; } .allocation label { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 12px 0; } .allocation input { width: 110px; }
  .project-list, .file-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 280px), 1fr)); gap: 14px; margin-top: 16px; } .project-card, .file-card { background: #f8f8f3; border: 1px solid #d0d8cc; min-width: 0; } .project-card { padding: 16px; } .project-open { display: flex; flex-direction: column; gap: 8px; align-items: flex-start; text-align: left; width: 100%; border: 0; padding: 0 0 16px; background: transparent; } .project-open strong { font-size: 1.05rem; overflow-wrap: anywhere; } .project-open span, .file-info span { color: #56655a; font-size: .85rem; } .actions { justify-content: flex-start; gap: 6px; } .actions button { font-size: .85rem; }
  .project-title { display: inline-block; margin-left: 12px; overflow-wrap: anywhere; } .upload { margin-top: 18px; border-style: dashed; } .upload label { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; font-weight: 600; } .upload p { margin-top: 8px; } .pending-row { display: flex; align-items: end; gap: 10px; margin: 12px 0; flex-wrap: wrap; } .pending-row label { display: grid; gap: 4px; flex: 1; min-width: 180px; } .pending progress { margin: 10px 0; }
  .filters { justify-content: flex-start; margin-bottom: 12px; } .search { display: flex; align-items: center; gap: 8px; margin-left: auto; } .batch { justify-content: flex-start; background: #e2ecda; padding: 10px; } .batch label { display: flex; align-items: center; gap: 6px; } .batch span { margin-right: auto; }
  .file-card { position: relative; } .select { position: absolute; top: 8px; left: 8px; z-index: 1; background: #f8f8f3; padding: 6px; } .file-preview { width: 100%; height: 160px; display: flex; justify-content: center; align-items: center; border: 0; background: #e2ecda; color: #56655a; padding: 0; } .file-preview img { width: 100%; height: 100%; object-fit: cover; } .file-info { display: flex; flex-direction: column; gap: 4px; padding: 12px; } .file-info strong { overflow-wrap: anywhere; } .file-card .actions { padding: 0 12px 12px; }
  .empty { color: #56655a; padding: 24px 0; } .message { padding: 12px; border: 1px solid #739265; margin-bottom: 16px; } .error { border-color: #a64c42; color: #8b3029; }
  .preview { position: fixed; inset: 0; z-index: 1000; background: #f3f2ec; color: #252b26; display: flex; flex-direction: column; padding: 20px; } .preview-head { border-bottom: 1px solid #d0d8cc; padding-bottom: 12px; } .preview-head strong { overflow-wrap: anywhere; } .preview-body { flex: 1; min-height: 0; display: flex; align-items: center; justify-content: center; padding-top: 16px; } .preview-body img, .preview-body video { max-width: 100%; max-height: 100%; } .preview-body pre { align-self: stretch; width: 100%; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; color: #252b26; }
  @media (max-width: 600px) { .warehouse { padding: 16px; } .search { margin-left: 0; width: 100%; } .search input { flex: 1; min-width: 0; } .disk-label { align-items: flex-start; flex-direction: column; gap: 3px; } .preview { padding: 12px; } }
</style>
