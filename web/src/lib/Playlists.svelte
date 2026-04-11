<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { showToast } from './stores.js';
  import PlaylistModal from './PlaylistModal.svelte';
  
  const dispatch = createEventDispatcher();
  let playlists = $state([]);
  let loading = $state(true);
  let isCreateModalOpen = $state(false);
  let smartCounts = $state({});

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    playlist: null,
    items: []
  });

  onMount(() => {
    fetchPlaylists();
    fetchSmartCounts();
  });

  async function fetchSmartCounts() {
    const types = ['recently-played', 'high-quality', 'recently-added', 'decade', 'genre'];
    for (const type of types) {
      try {
        const res = await fetch(`/api/smart-playlists/${type}`);
        const data = await res.json();
        smartCounts[type] = data.length;
      } catch (e) {}
    }
  }

  async function fetchPlaylists() {
    try {
      const res = await fetch('/api/playlists');
      playlists = await res.json() || [];
    } catch (e) {
      console.error("Failed to fetch playlists", e);
    } finally {
      loading = false;
    }
  }

  function showContextMenu(playlist, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      playlist,
      items: [
        { label: 'Play', icon: 'play', action: () => dispatch('selectPlaylist', playlist) },
        { divider: true },
        { label: 'Delete playlist', icon: 'trash', color: 'danger', action: () => deletePlaylist(playlist.id) }
      ]
    };
  }

  async function deletePlaylist(id) {
    if (!confirm("Delete this playlist?")) return;
    try {
      const res = await fetch(`/api/playlists/${id}`, { method: 'DELETE' });
      if (res.ok) {
        showToast("Playlist deleted", "success");
        fetchPlaylists();
      }
    } catch (e) {
      showToast("Delete failed", "error");
    }
  }

  function handleMouseMove(e) {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    e.currentTarget.style.setProperty('--mouse-x', `${x}px`);
    e.currentTarget.style.setProperty('--mouse-y', `${y}px`);
  }

  function createPlaylist() {
    isCreateModalOpen = true;
  }
</script>

<div class="playlists-view">
  <header class="section-header">
    <h1>Playlists</h1>
    <button class="create-btn" onclick={createPlaylist}>
      <Icon name="plus" size={18} />
      <span>New Playlist</span>
    </button>
  </header>

  <section class="smart-playlists">
    <h2>Smart Collections</h2>
    <div class="smart-grid">
      <button 
        class="smart-card" 
        onmousemove={handleMouseMove}
        onclick={() => dispatch('selectSmart', { type: 'recently-played', name: 'Recently Played', icon: 'clock' })}
      >
        <div class="smart-icon" style="background: var(--primary-10); color: var(--primary);"><Icon name="clock" size={24} /></div>
        <div class="smart-info">
          <span>Recently Played</span>
          <p class="count">{smartCounts['recently-played'] || 0} tracks</p>
        </div>
      </button>
      <button 
        class="smart-card" 
        onmousemove={handleMouseMove}
        onclick={() => dispatch('selectSmart', { type: 'high-quality', name: 'High Fidelity', icon: 'award' })}
      >
        <div class="smart-icon" style="background: rgba(214, 184, 92, 0.1); color: #d4b85c;"><Icon name="award" size={24} /></div>
        <div class="smart-info">
          <span>High Fidelity</span>
          <p class="count">{smartCounts['high-quality'] || 0} tracks</p>
        </div>
      </button>
      <button 
        class="smart-card" 
        onmousemove={handleMouseMove}
        onclick={() => dispatch('selectSmart', { type: 'recently-added', name: 'New Additions', icon: 'plus' })}
      >
        <div class="smart-icon" style="background: var(--secondary-10); color: var(--secondary);"><Icon name="plus" size={24} /></div>
        <div class="smart-info">
          <span>New Additions</span>
          <p class="count">{smartCounts['recently-added'] || 0} tracks</p>
        </div>
      </button>
      <button 
        class="smart-card" 
        onmousemove={handleMouseMove}
        onclick={() => dispatch('selectSmart', { type: 'decade', name: 'Decades', icon: 'calendar' })}
      >
        <div class="smart-icon" style="background: rgba(194, 112, 133, 0.1); color: #c27085;"><Icon name="calendar" size={24} /></div>
        <div class="smart-info">
          <span>Decades</span>
          <p class="count">{smartCounts['decade'] || 0} tracks</p>
        </div>
      </button>
      <button 
        class="smart-card" 
        onmousemove={handleMouseMove}
        onclick={() => dispatch('selectSmart', { type: 'genre', name: 'By Genre', icon: 'layers' })}
      >
        <div class="smart-icon" style="background: rgba(16, 185, 129, 0.1); color: #10b981;"><Icon name="layers" size={24} /></div>
        <div class="smart-info">
          <span>By Genre</span>
          <p class="count">{smartCounts['genre'] || 0} tracks</p>
        </div>
      </button>
    </div>
  </section>

  {#if loading}
    <div class="status">Sourcing playlists...</div>
  {:else if playlists.length === 0}
    <div class="empty-state">
      <div class="empty-icon"><Icon name="list" size={64} /></div>
      <p>Your collection is empty. Use the New Playlist button above to get started.</p>
    </div>
  {:else}
    <div class="playlists-grid">
      {#each playlists as playlist}
        <button 
          class="playlist-card" 
          onclick={() => dispatch('selectPlaylist', playlist)}
          oncontextmenu={(e) => showContextMenu(playlist, e)}
          onmousemove={handleMouseMove}
        >
          <div class="smart-icon playlist-icon">
            <Icon name="music" size={24} />
            <div class="play-overlay">
              <Icon name="play" size={16} color="black" />
            </div>
          </div>
          <div class="playlist-info">
            <h3>{playlist.name}</h3>
            <p>{playlist.trackCount || 0} tracks</p>
          </div>
        </button>
      {/each}
    </div>
  {/if}

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    onclose={() => contextMenu.visible = false}
  />

  <PlaylistModal 
    bind:isOpen={isCreateModalOpen} 
    onCreated={fetchPlaylists} 
  />
</div>

<style>
  .playlists-view { width: 100%; padding: 0 0 3rem 0; animation: fadeIn 0.4s ease-out; }
  
  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2.5rem;
  }

  h1 { font-size: 2.5rem; font-weight: 950; letter-spacing: -1.5px; margin: 0; color: var(--text-main); }

  .smart-playlists { margin-bottom: 4rem; }
  .smart-playlists h2 {
    font-size: 1.1rem;
    font-weight: 800;
    color: var(--text-muted);
    margin-bottom: 1.5rem;
    text-transform: uppercase;
    letter-spacing: 2px;
  }

  .smart-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 1.5rem;
  }

  .smart-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.2rem;
    padding: 2.5rem 1rem;
    background: var(--surface-container);
    border-radius: 32px;
    border: 1px solid var(--border-color);
    transition: all 0.5s cubic-bezier(0.16, 1, 0.3, 1);
    cursor: pointer;
    position: relative;
    overflow: hidden;
    isolation: isolate;
  }

  .smart-card:hover { 
    transform: translateY(-12px) scale(1.02); 
    background: var(--surface-container-high); 
    border-color: var(--primary-40);
    box-shadow: 0 20px 40px rgba(0,0,0,0.3);
  }

  /* Shared glow logic for smart cards too */
  .smart-card::before {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: 32px;
    background: radial-gradient(400px circle at var(--mouse-x, 50%) var(--mouse-y, 50%), rgba(var(--primary-rgb), 0.05), transparent 40%);
    opacity: 0;
    transition: opacity 0.5s;
    z-index: -1;
  }
  .smart-card:hover::before { opacity: 1; }

  .smart-card span { font-weight: 800; font-size: 0.95rem; color: var(--text-main); }
  .smart-info { display: flex; flex-direction: column; gap: 0.2rem; align-items: center; }
  .smart-info .count { margin: 0; font-size: 0.75rem; color: var(--text-muted); font-weight: 700; text-transform: none; letter-spacing: 0; }

  .smart-icon {
    width: 64px; height: 64px;
    border-radius: 20px;
    display: flex; align-items: center; justify-content: center;
    box-shadow: 0 8px 20px rgba(0,0,0,0.2);
  }

  .smart-card span { font-weight: 800; font-size: 0.95rem; color: var(--text-main); }

  .create-btn {
    background: var(--primary);
    color: white;
    padding: 1rem 2rem;
    border-radius: 100px;
    font-weight: 900;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    box-shadow: 0 10px 30px var(--primary-20);
    transition: all 0.3s ease;
    cursor: pointer;
    border: none;
  }
  .create-btn:hover { 
    transform: translateY(-6px) scale(1.05); 
    box-shadow: 0 15px 40px var(--primary-40);
    filter: brightness(1.1);
  }

  .playlists-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 1.5rem;
  }

  .playlist-card {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: flex-start;
    gap: 1.5rem;
    padding: 1.2rem 1.5rem;
    background: var(--surface-container);
    border-radius: 24px;
    transition: all 0.5s cubic-bezier(0.16, 1, 0.3, 1);
    border: 1px solid var(--border-color);
    cursor: pointer;
    text-align: left;
    position: relative;
    overflow: hidden;
    isolation: isolate;
  }

  .playlist-card:hover {
    transform: translateY(-12px) scale(1.02);
    border-color: var(--primary-40);
    background: var(--surface-container-high);
    box-shadow: 
      0 20px 40px rgba(0,0,0,0.3),
      0 0 0 1px var(--primary-20);
  }

  /* Glow effect on hover */
  .playlist-card::before {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: 32px;
    background: radial-gradient(800px circle at var(--mouse-x, 50%) var(--mouse-y, 50%), rgba(var(--primary-rgb), 0.08), transparent 40%);
    opacity: 0;
    transition: opacity 0.5s;
    z-index: -1;
  }
  .playlist-card:hover::before { opacity: 1; }

  .playlist-icon {
    background: var(--primary-10);
    color: var(--primary);
    position: relative;
  }

  .playlist-card:hover .smart-icon {
    transform: scale(1.1);
    background: var(--primary-20);
    box-shadow: 0 15px 30px rgba(0,0,0,0.3);
  }

  .play-overlay {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    background: var(--primary);
    border-radius: inherit;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transform: scale(0.8);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
  }

  .playlist-card:hover .play-overlay { 
    opacity: 1; 
    transform: scale(1);
  }

  .playlist-info h3 { margin: 0; font-size: 1.1rem; font-weight: 800; color: var(--text-main); }
  .playlist-info p { margin: 0.1rem 0 0; color: var(--text-muted); font-size: 0.8rem; font-weight: 600; }

  .status { text-align: center; padding: 4rem; color: var(--text-muted); }
  .empty-state { text-align: center; padding: 5rem 0; color: var(--text-muted); }

  @media (max-width: 768px) {
    h1 { font-size: 2rem; }
    .smart-grid { grid-template-columns: repeat(2, 1fr); gap: 1rem; }
    .playlists-grid { grid-template-columns: repeat(2, 1fr); gap: 1rem; }
    .smart-card { padding: 1.5rem 0.5rem; border-radius: 24px; }
    .playlist-card { border-radius: 24px; }
    .create-btn span { display: none; }
    .create-btn { padding: 1rem; }
  }
</style>
