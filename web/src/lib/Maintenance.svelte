<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { showToast } from './stores.js';
  import { fade } from 'svelte/transition';

  const dispatch = createEventDispatcher();
  
  let health = {
    missingTags: 0,
    duplicateCount: 0,
    orphanCount: 0,
    lowQualityCount: 0
  };
  
  let currentList = [];
  let currentView = 'overview'; // 'overview', 'duplicates', 'orphans', 'low-quality'
  let loading = false;

  onMount(fetchHealth);

  async function fetchHealth() {
    loading = true;
    try {
      const res = await fetch('/api/library/health');
      health = await res.json();
    } catch (e) {
      showToast("Failed to fetch health stats", "error");
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
      currentList = await res.json();
    } catch (e) {
      showToast("Failed to load details", "error");
    } finally {
      loading = false;
    }
  }

  async function runCleanup() {
    if (!confirm(`This will remove ${health.orphanCount} entries from database. Continue?`)) return;
    try {
      const res = await fetch('/api/library/cleanup', { method: 'POST' });
      const data = await res.json();
      showToast(`Cleaned up ${data.removed} tracks`, "success");
      fetchHealth();
      if (currentView === 'orphans') currentList = [];
    } catch (e) {
      showToast("Cleanup failed", "error");
    }
  }

  async function deleteTrack(track) {
    if (!confirm(`Delete "${track.title}"?`)) return;
    try {
      const res = await fetch(`/api/tracks/${track.id}`, { method: 'DELETE' });
      if (res.ok) {
        currentList = currentList.filter(t => t.id !== track.id);
        fetchHealth();
        showToast("Track deleted", "success");
      }
    } catch (e) {
      showToast("Failed to delete", "error");
    }
  }
</script>

<div class="maintenance" in:fade>
  <header class="view-header">
    {#if currentView !== 'overview'}
      <button class="back-btn" on:click={() => currentView = 'overview'}>
        <Icon name="arrowLeft" size={20} />
      </button>
    {/if}
    <h1>Library Maintenance</h1>
  </header>

  {#if currentView === 'overview'}
    <div class="stats-grid">
      <div class="stat-card" class:warning={health.duplicateCount > 0}>
        <Icon name="copy" size={32} />
        <div class="info">
          <span class="val">{health.duplicateCount}</span>
          <span class="lbl">Duplicates</span>
        </div>
        <button class="action-btn" on:click={() => loadDetail('duplicates')}>Review</button>
      </div>

      <div class="stat-card" class:danger={health.orphanCount > 0}>
        <Icon name="trash" size={32} />
        <div class="info">
          <span class="val">{health.orphanCount}</span>
          <span class="lbl">Orphans</span>
        </div>
        {#if health.orphanCount > 0}
          <button class="action-btn danger" on:click={runCleanup}>Clean All</button>
        {:else}
          <button class="action-btn" on:click={() => loadDetail('orphans')}>Review</button>
        {/if}
      </div>

      <div class="stat-card" class:warning={health.lowQualityCount > 0}>
        <Icon name="trendingDown" size={32} />
        <div class="info">
          <span class="val">{health.lowQualityCount}</span>
          <span class="lbl">Low Quality</span>
        </div>
        <button class="action-btn" on:click={() => loadDetail('low-quality')}>Review</button>
      </div>

      <div class="stat-card">
        <Icon name="tag" size={32} />
        <div class="info">
          <span class="val">{health.missingTags}</span>
          <span class="lbl">Missing Tags</span>
        </div>
        <button class="action-btn" on:click={() => dispatch('fixTags')}>Fix Now</button>
      </div>
    </div>
  {:else}
    <div class="detail-view">
      <h3>{currentView.charAt(0).toUpperCase() + currentView.slice(1).replace('-', ' ')}</h3>
      
      {#if loading}
        <p>Loading list...</p>
      {:else if currentList.length === 0}
        <p class="empty">No issues found in this category.</p>
      {:else}
        <table class="m-table">
          <thead>
            <tr>
              <th>Title</th>
              <th>Artist / Album</th>
              <th>Details</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each currentList as track}
              <tr>
                <td>{track.title}</td>
                <td class="sub">{track.artist}<br/>{track.album}</td>
                <td>
                  {#if track.bit_rate}
                    <span class="badge">{Math.round(track.bit_rate / 1000)}k</span>
                  {/if}
                  <span class="path">{track.path}</span>
                </td>
                <td>
                  <button class="icon-btn danger" on:click={() => deleteTrack(track)}>
                    <Icon name="trash" size={16} />
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

<style>
  .maintenance { padding: 1rem 0; }
  .view-header { display: flex; align-items: center; gap: 1rem; margin-bottom: 2rem; }
  h1 { font-family: var(--font-headline); margin: 0; }

  .back-btn { background: rgba(255,255,255,0.05); border: none; border-radius: 50%; width: 32px; height: 32px; display: flex; align-items: center; justify-content: center; color: white; cursor: pointer; }

  .stats-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 1.5rem; }
  
  .stat-card {
    background: var(--surface-container-high);
    border: 1px solid var(--border-color);
    padding: 1.5rem;
    border-radius: var(--radius);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
    text-align: center;
    transition: transform 0.2s;
  }
  .stat-card:hover { transform: translateY(-5px); border-color: var(--primary); }
  .stat-card.warning { border-color: var(--tertiary); }
  .stat-card.danger { border-color: var(--secondary); }

  .stat-card .val { font-size: 2.5rem; font-weight: 900; color: var(--primary); font-family: var(--font-headline); display: block; }
  .stat-card .lbl { color: var(--text-muted); font-weight: 700; text-transform: uppercase; font-size: 0.8rem; }

  .action-btn {
    width: 100%;
    background: rgba(255,255,255,0.05);
    border: 1px solid var(--border-color);
    color: white;
    padding: 0.6rem;
    border-radius: 100px;
    font-weight: 700;
    cursor: pointer;
  }
  .action-btn:hover { background: var(--primary); color: white; border-color: var(--primary); }
  .action-btn.danger:hover { background: var(--secondary); border-color: var(--secondary); }

  .detail-view { background: var(--surface-container-high); border-radius: var(--radius); padding: 2rem; border: 1px solid var(--border-color); }
  
  .m-table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
  .m-table th { text-align: left; padding: 1rem; border-bottom: 1px solid var(--border-color); color: var(--text-muted); font-size: 0.8rem; text-transform: uppercase; }
  .m-table td { padding: 1rem; border-bottom: 1px solid rgba(255,255,255,0.02); }
  .m-table .sub { font-size: 0.85rem; color: var(--text-muted); line-height: 1.4; }
  .badge { background: var(--primary-10); color: var(--primary); padding: 0.2rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 800; }
  .path { font-size: 0.7rem; color: var(--text-muted); display: block; margin-top: 0.4rem; opacity: 0.5; max-width: 300px; overflow: hidden; text-overflow: ellipsis; }

  .icon-btn { background: transparent; border: none; padding: 0.5rem; border-radius: 8px; cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.2s; }
  .icon-btn.danger { color: #f87171; }
  .icon-btn.danger:hover { background: rgba(248, 113, 113, 0.1); }
</style>
