<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { showToast } from './stores.js';
  
  const dispatch = createEventDispatcher();
  let playlists = $state([]);
  let loading = $state(true);

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    playlist: null,
    items: []
  });

  onMount(fetchPlaylists);

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

  function createPlaylist() {
    const name = prompt("New playlist name:");
    if (!name) return;
    
    fetch('/api/playlists', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    }).then(res => {
      if (res.ok) fetchPlaylists();
    });
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
      <button class="smart-card" onclick={() => dispatch('selectSmart', { type: 'recently-played', name: 'Recently Played', icon: 'clock' })}>
        <div class="smart-icon" style="background: var(--primary-10); color: var(--primary);"><Icon name="clock" size={24} /></div>
        <span>Recently Played</span>
      </button>
      <button class="smart-card" onclick={() => dispatch('selectSmart', { type: 'high-quality', name: 'High Fidelity', icon: 'award' })}>
        <div class="smart-icon" style="background: rgba(214, 184, 92, 0.1); color: #d4b85c;"><Icon name="award" size={24} /></div>
        <span>High Fidelity</span>
      </button>
      <button class="smart-card" onclick={() => dispatch('selectSmart', { type: 'recently-added', name: 'New Additions', icon: 'plus' })}>
        <div class="smart-icon" style="background: var(--secondary-10); color: var(--secondary);"><Icon name="plus" size={24} /></div>
        <span>New Additions</span>
      </button>
      <button class="smart-card" onclick={() => dispatch('selectSmart', { type: 'decade', name: 'Decades', icon: 'calendar' })}>
        <div class="smart-icon" style="background: rgba(194, 112, 133, 0.1); color: #c27085;"><Icon name="calendar" size={24} /></div>
        <span>Decades</span>
      </button>
      <button class="smart-card" onclick={() => dispatch('selectSmart', { type: 'genre', name: 'By Genre', icon: 'layers' })}>
        <div class="smart-icon" style="background: rgba(16, 185, 129, 0.1); color: #10b981;"><Icon name="layers" size={24} /></div>
        <span>By Genre</span>
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
        >
          <div class="playlist-art">
            <Icon name="music" size={40} />
            <div class="play-overlay">
              <Icon name="play" size={24} color="black" />
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
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    cursor: pointer;
    border: 1px solid transparent;
  }

  .smart-card:hover { transform: translateY(-8px); background: var(--surface-container-high); border-color: var(--primary); }

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
  .create-btn:hover { transform: translateY(-4px); box-shadow: 0 15px 40px var(--primary-30); }

  .playlists-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
  }

  .playlist-card {
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    padding: 1rem;
    background: var(--surface-container);
    border-radius: 32px;
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    border: 1px solid var(--border-color);
    cursor: pointer;
    text-align: left;
  }
  .playlist-card:hover { border-color: var(--primary); transform: translateY(-8px); box-shadow: 0 12px 24px rgba(0,0,0,0.2); }

  .playlist-art {
    aspect-ratio: 1;
    background: var(--surface-container-highest);
    border-radius: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    color: var(--text-muted);
    overflow: hidden;
  }

  .play-overlay {
    position: absolute;
    bottom: 1rem;
    right: 1rem;
    width: 48px;
    height: 48px;
    background: var(--primary);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transform: scale(0.8) translateY(10px);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    box-shadow: 0 8px 20px rgba(0,0,0,0.4);
  }

  .playlist-card:hover .play-overlay { opacity: 1; transform: scale(1) translateY(0); }

  .playlist-info h3 { margin: 0; font-size: 1.1rem; font-weight: 800; color: var(--text-main); }
  .playlist-info p { margin: 0.2rem 0 0; color: var(--text-muted); font-size: 0.85rem; font-weight: 600; }

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
