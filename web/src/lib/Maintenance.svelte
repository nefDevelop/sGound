<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { showToast } from './stores.js';
  import { fade } from 'svelte/transition';
  import LogViewer from './LogViewer.svelte';

  let { onfixTags, subview = 'overview', onviewchange, onedit, refresh = $bindable() } = $props();
  
  let health = $state({
    missingTags: 0,
    duplicateCount: 0,
    orphanCount: 0,
    lowQualityCount: 0
  });
  
  let currentList = $state([]);
  let currentView = $derived(subview); // 'overview', 'duplicates', 'orphans', 'low-quality'
  let loading = $state(false);
  
  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    track: null,
    items: []
  });

  // Re-fetch details when subview changes via navigation
  $effect(() => {
    if (subview !== 'overview' && subview !== 'logs') {
      fetchDetail(subview);
    }
  });

  onMount(fetchHealth);

  async function fetchHealth() {
    loading = true;
    try {
      const res = await fetch('/api/library/health');
      health = (await res.json()) || health;
    } catch (e) {
      showToast("Could not source library health", "error");
    } finally {
      loading = false;
    }
  }

  async function loadDetail(view) {
    onviewchange?.(view);
  }

  async function fetchDetail(view) {
    loading = true;
    currentList = [];
    try {
      const res = await fetch(`/api/library/${view}`);
      currentList = (await res.json()) || [];
    } catch (e) {
      showToast("Detail sourcing failed", "error");
    } finally {
      loading = false;
    }
  }

  async function runCleanup() {
    if (!confirm(`Purge ${health.orphanCount} orphan entries from database?`)) return;
    try {
      const res = await fetch('/api/library/cleanup', { method: 'POST' });
      const data = await res.json();
      showToast(`Purged ${data.removed} stale entries`, "success");
      fetchHealth();
      if (currentView === 'orphans') currentList = [];
    } catch (e) {
      showToast("Cleanup failed", "error");
    }
  }

  async function deleteTrack(track) {
    if (!confirm(`Permanently delete "${track.title}" from library?`)) return;
    try {
      const res = await fetch(`/api/tracks/${track.id}`, { method: 'DELETE' });
      if (res.ok) {
        if (subview === 'duplicates') {
          // Remove from group and filter groups that no longer have duplicates
          currentList = currentList
            .map(group => group.filter(t => t.id !== track.id))
            .filter(group => group.length > 1);
        } else {
          currentList = currentList.filter(t => t.id !== track.id);
        }
        fetchHealth();
        showToast("Asset deleted successfully", "success");
      }
    } catch (e) {
      showToast("Deletion failed", "error");
    }
  }
  function formatDuration(seconds) {
    if (!seconds) return '--:--';
    const min = Math.floor(seconds / 60);
    const sec = Math.floor(seconds % 60);
    return `${min}:${sec.toString().padStart(2, '0')}`;
  }

  function syncScroll(e, groupIndex) {
    const scrollLeft = e.target.scrollLeft;
    const groupScrollers = document.querySelectorAll(`.scroller-group-${groupIndex}`);
    groupScrollers.forEach(s => {
      if (s !== e.target) s.scrollLeft = scrollLeft;
    });
  }

  function showTrackMenu(track, e) {
    if (e) e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      track,
      items: [
        { label: 'Editar Tags', icon: 'edit', action: () => onedit?.(track) },
        { divider: true },
        { label: 'Borrar pista', icon: 'trash', color: 'danger', action: () => deleteTrack(track) }
      ]
    };
  }

  async function renameTrack(track) {
    const newTitle = prompt("Enter new track title:", track.title);
    if (!newTitle || newTitle === track.title) return;

    try {
      const res = await fetch(`/api/tracks/tags?id=${encodeURIComponent(track.id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: newTitle })
      });
      if (res.ok) {
        showToast("Track renamed successfully", "success");
        if (currentView !== 'overview') fetchDetail(currentView);
      } else {
        throw new Error("Failed to rename");
      }
    } catch (e) {
      showToast("Error renaming: " + e.message, "error");
    }
  }

  function dragToScroll(node) {
    let isDown = false;
    let startX;
    let scrollLeft;

    const onMouseDown = (e) => {
      isDown = true;
      startX = e.pageX - node.offsetLeft;
      scrollLeft = node.scrollLeft;
    };

    const onMouseLeave = () => { isDown = false; };
    const onMouseUp = () => { isDown = false; };

    const onMouseMove = (e) => {
      if (!isDown) return;
      e.preventDefault();
      const x = e.pageX - node.offsetLeft;
      const walk = (x - startX) * 2;
      node.scrollLeft = scrollLeft - walk;
    };

    node.addEventListener('mousedown', onMouseDown);
    node.addEventListener('mouseleave', onMouseLeave);
    node.addEventListener('mouseup', onMouseUp);
    node.addEventListener('mousemove', onMouseMove);

    return {
      destroy() {
        node.removeEventListener('mousedown', onMouseDown);
        node.removeEventListener('mouseleave', onMouseLeave);
        node.removeEventListener('mouseup', onMouseUp);
        node.removeEventListener('mousemove', onMouseMove);
      }
    };
  }
  // Expose refresh function to parent
  refresh = () => {
    fetchHealth();
    if (currentView !== 'overview') fetchDetail(currentView);
  };
</script>

<div class="maintenance-view" in:fade>
  <header class="section-header">
    <div class="texts">
       <div class="title-row">
        {#if currentView !== 'overview'}
          <button class="icon-btn back-btn" onclick={() => onviewchange?.('overview')} aria-label="Back to overview">
            <Icon name="arrowLeft" size={24} />
          </button>
        {/if}
        <h1>Library Health</h1>
      </div>
      {#if currentView === 'overview'}
        <p class="subtitle">System tools for library optimization and integrity checks.</p>
      {/if}
    </div>
  </header>

  {#if currentView === 'overview'}
    <div class="bento-grid">
      <div class="bento-card" class:warning={health.duplicateCount > 0}>
        <div class="card-header">
          <Icon name="copy" size={32} />
          <span class="lbl">Duplicates</span>
        </div>
        <span class="val">{health.duplicateCount}</span>
        <button class="bento-btn" onclick={() => loadDetail('duplicates')}>Inspect Detail</button>
      </div>

      <div class="bento-card" class:danger={health.orphanCount > 0}>
        <div class="card-header">
          <Icon name="trash" size={32} />
          <span class="lbl">Orphans</span>
        </div>
        <span class="val">{health.orphanCount}</span>
        {#if health.orphanCount > 0}
          <button class="bento-btn danger" onclick={runCleanup}>Purge All</button>
        {:else}
          <button class="bento-btn" onclick={() => loadDetail('orphans')}>Inspect Detail</button>
        {/if}
      </div>

      <div class="bento-card" class:warning={health.lowQualityCount > 0}>
        <div class="card-header">
          <Icon name="trendingDown" size={32} />
          <span class="lbl">Low Quality</span>
        </div>
        <span class="val">{health.lowQualityCount}</span>
        <button class="bento-btn" onclick={() => loadDetail('low-quality')}>Inspect Detail</button>
      </div>

      <div class="bento-card highlight">
        <div class="card-header">
          <Icon name="tag" size={32} />
          <span class="lbl">Missing Metadata</span>
        </div>
        <span class="val">{health.missingTags}</span>
        <button class="bento-btn primary" onclick={() => onfixTags?.()}>Batch Fix</button>
      </div>

      <div class="bento-card highlight terminal">
        <div class="card-header">
          <Icon name="terminal" size={32} />
          <span class="lbl">Server Activity</span>
        </div>
        <div class="terminal-preview">
          Monitoring real-time events...
        </div>
        <button class="bento-btn primary" onclick={() => loadDetail('logs')}>View Logs</button>
      </div>
    </div>
  {:else}
    <div class="detail-container">
      <div class="detail-header">
        <div class="title-wrap">
          <h2>{currentView.split('-').map(s => s.charAt(0).toUpperCase() + s.slice(1)).join(' ')}</h2>
          <p class="desc">{currentView === 'duplicates' ? 'Tracks grouped by content similarity.' : 'Isolated database entries with no physical files.'}</p>
        </div>
        {#if loading}
          <div class="loading-spinner"></div>
        {/if}
      </div>
      
      {#if !loading && currentList.length === 0}
        <div class="empty-state">
          <Icon name="checkCircle" size={48} color="var(--primary)" />
          <p>This section is perfectly optimized.</p>
        </div>
      {:else}
        <div class="scroll-wrapper">
          {#if currentView === 'duplicates'}
             <div class="groups-container">
               {#each currentList as group}
                 {@const best = group.reduce((acc, curr) => {
                    const accScore = (acc.format === 'flac' ? 10 : 0) + (acc.bit_rate || 0);
                    const currScore = (curr.format === 'flac' ? 10 : 0) + (curr.bit_rate || 0);
                    return currScore > accScore ? curr : acc;
                 }, group[0])}
                 
                 <div class="duplicate-group" in:fade>
                   <div class="group-header">
                     <Icon name="copy" size={18} />
                     <span>{group[0].title} — {group.length} versions found</span>
                   </div>
                   
                   <table class="m-table">
                     <tbody>
                        {#each group as track, i}
                          <tr 
                            class:is-best={track.id === best.id} 
                            class="duplicate-row {track.id === best.id ? 'best-row' : ''} row-{i%2 === 0 ? 'even' : 'odd'}"
                            oncontextmenu={(e) => showTrackMenu(track, e)}
                          >
                            <td class="main-info-col">
                              <div class="title-cell">
                                <span class="track-sub">
                                  {#if track.id === best.id}
                                    <Icon name="award" size={14} color="var(--primary)" />
                                  {/if}
                                  {track.artist}
                                </span>
                                
                                <div class="attr-badges">
                                  <span class="format-badge-mini" title={track.format?.toUpperCase()}>
                                     <Icon name="file" size={12} />
                                     {track.format?.toUpperCase()}
                                  </span>
                                  <span class="quality-badge" title="Bitrate">
                                    <Icon name="activity" size={12} />
                                    {track.bit_rate ? Math.round(track.bit_rate / 1000) : '---'}k
                                  </span>
                                  <span class="time-badge">
                                    <Icon name="clock" size={12} />
                                    {formatDuration(track.duration)}
                                  </span>
                                  {#if track.fileHash}
                                    <span class="hash-badge" title="Physical match verified">
                                      <Icon name="link" size={12} />
                                      MATCH
                                    </span>
                                  {/if}
                                </div>
                              </div>
                            </td>
                            <td class="path-col">
                              <div class="path-outer">
                                <div 
                                  use:dragToScroll 
                                  class="path-scroller scroller-group-{(group[0].id).slice(0,8)}" 
                                  onscroll={(e) => syncScroll(e, (group[0].id).slice(0,8))} 
                                  title={track.path}
                                >
                                  <code>{track.path}</code>
                                </div>
                              </div>
                            </td>
                            <td class="actions-col">
                              <button 
                                class="icon-btn danger-hover" 
                                onclick={() => deleteTrack(track)} 
                                title={track.id === best.id ? "Keep this one" : "Delete redundant copy"}
                              >
                                <Icon name="trash" size={18} />
                              </button>
                            </td>
                          </tr>
                        {/each}
                     </tbody>
                   </table>
                 </div>
               {/each}
             </div>
          {:else if currentView === 'logs'}
             <LogViewer />
          {:else}
            <table class="m-table">
              <thead>
                <tr>
                  <th>Composition</th>
                  <th>Attributes</th>
                  <th>Path Integrity</th>
                  <th class="actions-col"></th>
                </tr>
              </thead>
              <tbody>
                {#each currentList as track}
                  <tr>
                    <td>
                      <div class="title-cell">
                        <span class="track-title">{track.title}</span>
                        <span class="track-sub">{track.artist} • {track.album}</span>
                      </div>
                    </td>
                    <td>
                      <div class="attr-badges">
                        {#if track.bit_rate}
                          <span class="quality-badge">
                            <Icon name="activity" size={12} />
                            {Math.round(track.bit_rate / 1000)}k
                          </span>
                        {/if}
                        <span class="format-badge-mini">{track.format?.toUpperCase() || 'AUDIO'}</span>
                      </div>
                    </td>
                    <td>
                       <code class="path-snippet" title={track.path}>{track.path}</code>
                    </td>
                    <td class="actions-col">
                      <button class="icon-btn danger-hover" onclick={() => deleteTrack(track)} title="Delete asset">
                        <Icon name="trash" size={18} />
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      {/if}
    </div>
  {/if}

  <ContextMenu 
    bind:visible={contextMenu.visible} 
    x={contextMenu.x} 
    y={contextMenu.y} 
    items={contextMenu.items} 
    onclose={() => contextMenu.visible = false}
  />
</div>

<style>
  .maintenance-view { padding: 0; width: 100%; max-width: 1400px; margin: 0 auto; }
  
  .subtitle { color: var(--text-muted); font-size: 1.2rem; font-weight: 500; opacity: 0.8; margin-top: 0.5rem; }

  .bento-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 2rem;
  }

  .bento-card {
    background: var(--surface-container);
    border: 1px solid var(--border-color);
    padding: 2.5rem;
    border-radius: 40px;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    transition: all 0.4s cubic-bezier(0.2, 0, 0, 1);
    position: relative;
    overflow: hidden;
  }
  .bento-card:hover { transform: translateY(-8px); border-color: var(--primary-40); box-shadow: 0 20px 40px rgba(0,0,0,0.2); }
  
  .bento-card.warning { border-left: 6px solid var(--tertiary); }
  .bento-card.danger { border-left: 6px solid #ff4444; }
  .bento-card.highlight { background: var(--surface-container-high); border: 2px solid var(--primary-20); }

  .card-header { display: flex; align-items: center; gap: 1.2rem; color: var(--text-muted); }
  .card-header .lbl { text-transform: uppercase; font-size: 0.9rem; font-weight: 800; letter-spacing: 0.1em; }

  .bento-card.terminal { background: #0d1117; border-color: #30363d; }
  .terminal-preview { flex: 1; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; color: #8b949e; opacity: 0.8; }

  .bento-card .val { font-size: 4rem; font-weight: 950; color: var(--text-main); line-height: 1; letter-spacing: -0.05em; }

  .bento-btn {
    width: 100%;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 1.2rem;
    border-radius: 24px;
    font-weight: 850;
    cursor: pointer;
    transition: all 0.3s;
    font-size: 1rem;
  }
  .bento-btn:hover { background: var(--text-main); color: var(--surface-container); }
  .bento-btn.primary { background: var(--primary); color: white; border: none; }
  .bento-btn.danger { background: #ff444415; color: #ff4444; border-color: #ff444430; }
  .bento-btn.danger:hover { background: #ff4444; color: white; }

  .detail-container { 
    background: var(--surface-container); 
    border-radius: 48px; 
    padding: 3rem; 
    border: 1px solid var(--border-color); 
    animation: slideUp 0.4s ease-out;
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .title-wrap { display: flex; flex-direction: column; gap: 0.5rem; }

  .groups-container { display: flex; flex-direction: column; gap: 3rem; }

  .duplicate-group {
    background: var(--surface-container-high);
    border-radius: 32px;
    padding: 2rem;
    border: 1px solid var(--border-color);
  }

  .group-header {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    margin-bottom: 1.5rem;
    color: var(--primary);
    font-weight: 900;
    text-transform: uppercase;
    font-size: 0.85rem;
    letter-spacing: 1px;
  }

  /* best-row handles this now */

  .m-table { width: 100%; border-collapse: collapse; table-layout: fixed; margin-top: 1rem; }
  .m-table th { text-align: left; padding: 1rem 1.5rem; color: var(--text-muted); font-size: 0.85rem; font-weight: 800; text-transform: uppercase; letter-spacing: 0.05em; }
  .m-table td { padding: 1.5rem; border-top: 1px solid rgba(255,255,255,0.03); vertical-align: middle; }
  .m-table tr:first-child td { border-top: none; }

  .duplicate-row { transition: all 0.2s; }
  .attr-badges { display: flex; align-items: center; gap: 0.4rem; white-space: nowrap; margin-top: 0.4rem; }
  
  .format-badge-mini {
    background: var(--surface-container-highest);
    color: var(--text-muted);
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    font-size: 0.7rem;
    font-weight: 950;
    display: flex;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid rgba(255,255,255,0.05);
    min-width: 70px;
    justify-content: center;
  }

  .time-badge {
    background: rgba(255,255,255,0.03);
    color: var(--text-muted);
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    font-size: 0.7rem;
    font-weight: 800;
    display: flex;
    align-items: center;
    gap: 0.3rem;
    min-width: 65px;
    justify-content: center;
  }

  .hash-badge {
    background: #10b98115;
    color: #10b981;
    font-size: 0.65rem;
    font-weight: 900;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .quality-badge { 
    background: var(--surface-container-highest);
    color: var(--primary); 
    padding: 0.3rem 0.6rem; 
    border-radius: 6px; 
    font-size: 0.7rem; 
    font-weight: 950; 
    min-width: 65px; 
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    border: 1px solid rgba(255,255,255,0.08);
  }

  .attr-badges :global(.lucide-icon) { flex-shrink: 0; }

  /* Zebra Stripes refined */
  .m-table tr.row-even td { background: rgba(255,255,255,0.02); }
  .m-table tr.row-odd td { background: transparent; }
  
  .m-table tr.best-row td { background: var(--primary-05) !important; }
  .m-table tr.best-row td:first-child { border-left: none; }

  .path-outer { display: flex; align-items: center; gap: 0.2rem; width: 100%; }

  .path-scroller {
    flex: 1;
    overflow-x: auto;
    background: rgba(0,0,0,0.25);
    padding: 0.7rem 0.9rem;
    border-radius: 12px;
    scrollbar-width: thin;
    scrollbar-color: var(--primary-20) transparent;
    border: 1px solid rgba(255,255,255,0.03);
    cursor: grab;
    user-select: none;
  }
  .path-scroller:active { cursor: grabbing; }
  
  .path-scroller::-webkit-scrollbar { height: 4px; }
  .path-scroller::-webkit-scrollbar-thumb { background: var(--primary-20); border-radius: 4px; }

  .path-scroller code {
    white-space: nowrap;
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.68rem;
    color: var(--text-main);
    opacity: 0.6;
  }

  .sep { opacity: 0.3; margin: 0 0.1rem; }
  .main-info-col { width: 320px; }
  .duplicate-group .tech-col { display: none; }
  .tech-col { width: 220px; }
  .path-col { width: auto; }
  .actions-col { width: 80px; text-align: right; }

  .icon-btn { border: none; background: var(--surface-container-highest); color: var(--text-muted); width: 44px; height: 44px; border-radius: 14px; display: flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.2s; }
  .icon-btn:hover { background: var(--surface-container-highest-hover); color: var(--text-main); transform: scale(1.05); }
  .icon-btn.danger-hover:hover { background: #ff444415; color: #ff4444; }

  .title-cell { display: flex; flex-direction: column; gap: 0.3rem; }
  .track-sub { font-size: 0.9rem; color: var(--text-muted); font-weight: 600; opacity: 0.7; display: flex; align-items: center; gap: 0.5rem; }

  @keyframes slideUp { from { transform: translateY(20px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }

  @media (max-width: 900px) {
    .bento-grid { grid-template-columns: repeat(2, 1fr); gap: 1rem; }
    .bento-card { padding: 1.5rem; border-radius: 24px; gap: 1rem; }
    .bento-card .val { font-size: 2.5rem; }
    .detail-container { padding: 1.5rem; border-radius: 32px; }
    .m-table th { display: none; }
    .m-table td { display: block; padding: 1rem; }
    .m-table td:first-child { border-radius: 20px 20px 0 0; border-right: 1px solid var(--white-05); }
    .m-table td:last-child { border-radius: 0 0 20px 20px; border-left: 1px solid var(--white-05); text-align: right; }
    .path-snippet { max-width: 100%; }
  }
</style>
