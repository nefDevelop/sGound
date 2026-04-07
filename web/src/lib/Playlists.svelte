<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  
  const dispatch = createEventDispatcher();
  let playlists = [];
  let loading = true;

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

  function createPlaylist() {
    const name = prompt("Nombre de la nueva playlist:");
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
    <button class="create-btn" on:click={createPlaylist}>
      <Icon name="plus" size={18} />
      Nueva Playlist
    </button>
  </header>

  {#if loading}
    <div class="loading">Cargando playlists...</div>
  {:else if playlists.length === 0}
    <div class="empty-state">
      <div class="empty-icon"><Icon name="list" size={64} /></div>
      <p>No tienes listas de reproducción aún.</p>
      <button class="secondary-btn" on:click={createPlaylist}>Crear la primera</button>
    </div>
  {:else}
    <div class="playlists-grid">
      {#each playlists as playlist}
        <button class="playlist-card glass" on:click={() => dispatch('selectPlaylist', playlist)}>
          <div class="playlist-art">
            <Icon name="music" size={40} />
          </div>
          <div class="playlist-info">
            <h3>{playlist.name}</h3>
            <p>{playlist.trackCount || 0} canciones</p>
          </div>
          <div class="play-overlay">
            <Icon name="play" size={24} color="black" />
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .playlists-view { animation: fadeIn 0.4s ease; }
  
  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2.5rem;
  }

  h1 { 
    font-family: var(--font-headline);
    font-size: var(--font-size-h1);
    font-weight: var(--font-weight-h1);
    letter-spacing: var(--letter-spacing-h1);
    margin: 0;
  }

  .create-btn {
    background: var(--primary);
    color: white;
    padding: 0.8rem 1.5rem;
    border-radius: 100px;
    font-weight: 700;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    box-shadow: 0 10px 20px var(--primary-10);
  }

  .playlists-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 2rem;
  }

  .playlist-card {
    position: relative;
    padding: 1.5rem;
    border-radius: var(--radius);
    text-align: left;
    transition: all 0.3s ease;
    cursor: pointer;
    border: 1px solid rgba(255,255,255,0.05);
  }

  .playlist-card:hover {
    transform: translateY(-8px);
    background: rgba(255,255,255,0.05);
    border-color: var(--primary);
  }

  .playlist-art {
    aspect-ratio: 1;
    background: linear-gradient(135deg, #1e293b, #0f172a);
    border-radius: calc(var(--radius) / 2);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 1.2rem;
    color: var(--primary);
    box-shadow: 0 20px 40px rgba(0,0,0,0.3);
  }

  .playlist-info h3 {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 700;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .playlist-info p {
    margin: 0.4rem 0 0;
    color: var(--text-muted);
    font-size: 0.9rem;
  }

  .play-overlay {
    position: absolute;
    top: 1.5rem;
    right: 1.5rem;
    width: 48px;
    height: 48px;
    background: white;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transform: translateY(10px);
    transition: all 0.3s ease;
    box-shadow: 0 10px 20px rgba(0,0,0,0.4);
  }

  .playlist-card:hover .play-overlay {
    opacity: 1;
    transform: translateY(0);
  }

  .empty-state {
    text-align: center;
    padding: 5rem 0;
    color: var(--text-muted);
  }

  .empty-icon { margin-bottom: 1.5rem; opacity: 0.2; }

  .secondary-btn {
    margin-top: 1.5rem;
    padding: 0.8rem 2rem;
    border: 1px solid var(--primary);
    border-radius: 100px;
    color: var(--primary);
    font-weight: 700;
  }
</style>
