<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  import { fade } from 'svelte/transition';

  let { onfixTags } = $props();
  
  let health = $state({
    missingTags: 0,
    duplicateCount: 0,
    orphanCount: 0,
    lowQualityCount: 0
  });
  
  let currentList = $state([]);
  let currentView = $state('overview'); // 'overview', 'duplicates', 'orphans', 'low-quality'
  let loading = $state(false);

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
    currentView = view;
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
        currentList = currentList.filter(t => t.id !== track.id);
        fetchHealth();
        showToast("Asset deleted successfully", "success");
      }
    } catch (e) {
      showToast("Deletion failed", "error");
    }
  }
</script>

<div class="maintenance-view" in:fade>
  <header class="section-header">
    <div class="texts">
       <div class="title-row">
        {#if currentView !== 'overview'}
          <button class="icon-btn back-btn" onclick={() => currentView = 'overview'} aria-label="Back to overview">
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
                       {#each group as track}
                         <tr class:is-best={track.id === best.id}>
                           <td>
                             <div class="title-cell">
                               <div class="top">
                                 <span class="track-title">{track.title}</span>
                                 {#if track.id === best.id}
                                   <span class="best-badge"><Icon name="award" size={12} /> BEST QUALITY</span>
                                 {/if}
                               </div>
                               <span class="track-sub">{track.artist} inner tags</span>
                             </div>
                           </td>
                           <td>
                             <div class="attr-badges">
                               {#if track.bit_rate}
                                 <span class="quality-badge">{Math.round(track.bit_rate / 1000)}kbps</span>
                               {/if}
                               <span class="format-badge">{track.format?.toUpperCase()}</span>
                               {#if track.fileHash}
                                 <span class="hash-badge" title="Physical match verified">HASH MATCH</span>
                               {/if}
                             </div>
                           </td>
                           <td>
                             <code class="path-snippet" title={track.path}>{track.path}</code>
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
                      {#if track.bit_rate}
                        <span class="quality-badge">{Math.round(track.bit_rate / 1000)}kbps</span>
                      {/if}
                      <span class="format-badge">{track.format?.toUpperCase() || 'AUDIO'}</span>
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

  tr.is-best td { background: var(--surface-container-highest); border-color: var(--primary-20); }

  .top { display: flex; align-items: center; gap: 1rem; }
  
  .best-badge {
    background: var(--primary);
    color: white;
    font-size: 0.65rem;
    font-weight: 900;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .attr-badges { display: flex; align-items: center; gap: 0.5rem; }
  .hash-badge {
    background: #10b98120;
    color: #10b981;
    font-size: 0.7rem;
    font-weight: 900;
    padding: 0.4rem 0.8rem;
    border-radius: 10px;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2.5rem;
    gap: 2rem;
  }

  h1 { 
    font-size: 2.5rem; 
    font-weight: 950; 
    margin: 0; 
    letter-spacing: -1.5px;
    line-height: 1;
    color: var(--text-main);
  }

  .title-row { display: flex; align-items: center; gap: 1rem; }

  .detail-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 2.5rem; }
  .detail-header h2 { font-size: 2.2rem; font-weight: 900; margin: 0; letter-spacing: -0.02em; }

  .scroll-wrapper { overflow-x: auto; margin: 0 -1rem; padding: 0 1rem; }

  .m-table { width: 100%; border-collapse: separate; border-spacing: 0 0.8rem; }
  .m-table th { text-align: left; padding: 1rem 1.5rem; color: var(--text-muted); font-size: 0.85rem; font-weight: 800; text-transform: uppercase; letter-spacing: 0.05em; }
  .m-table td { background: var(--surface-container-high); padding: 1.5rem; border-top: 1px solid var(--white-05); border-bottom: 1px solid var(--white-05); }
  .m-table td:first-child { border-radius: 20px 0 0 20px; border-left: 1px solid var(--white-05); }
  .m-table td:last-child { border-radius: 0 20px 20px 0; border-right: 1px solid var(--white-05); }

  .title-cell { display: flex; flex-direction: column; gap: 0.3rem; }
  .track-title { font-weight: 800; font-size: 1.1rem; color: var(--text-main); }
  .track-sub { font-size: 0.9rem; color: var(--text-muted); font-weight: 600; opacity: 0.7; }

  .quality-badge { background: var(--primary-15); color: var(--primary); padding: 0.4rem 0.8rem; border-radius: 10px; font-size: 0.8rem; font-weight: 900; margin-right: 0.5rem; }
  .format-badge { background: var(--surface-container-highest); color: var(--text-muted); padding: 0.4rem 0.8rem; border-radius: 10px; font-size: 0.8rem; font-weight: 800; }

  .path-snippet { font-family: 'JetBrains Mono', monospace; font-size: 0.75rem; color: var(--text-muted); opacity: 0.6; max-width: 400px; display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .icon-btn { border: none; background: var(--surface-container-highest); color: var(--text-muted); width: 44px; height: 44px; border-radius: 14px; display: flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.2s; }
  .icon-btn:hover { background: var(--surface-container-highest-hover); color: var(--text-main); transform: scale(1.05); }
  .icon-btn.danger-hover:hover { background: #ff444415; color: #ff4444; }

  .actions-col { width: 60px; text-align: center; }

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
