<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';
  import TracksView from './TracksView.svelte';

  let { playlist = { id: '', name: '' }, currentPlaying = null, onplay, onedit, onselectionChange, onqueue, onhide, ondeletePhysical, onshuffle } = $props();
  
  let tracks = $state([]);
  let loading = $state(true);
  let error = $state(null);

  $effect(() => {
    if (playlist?.id) fetchPlaylistTracks();
  });

  export async function fetchPlaylistTracks() {
    loading = true;
    try {
      const res = await fetch(`/api/playlists/${playlist.id}/tracks`);
      if (!res.ok) throw new Error("Could not load playlist tracks");
      tracks = await res.json();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function handlePlayAll() {
    if (tracks.length > 0) onplay?.(tracks[0]);
  }
</script>

<div class="playlist-view" in:fade>
  {#if playlist}
    <div class="playlist-header">
      <div class="playlist-cover" class:is-smart={playlist.isSmart}>
          <Icon name={playlist.isSmart ? "zap" : "list"} size={80} />
      </div>
      <div class="playlist-info">
        <span class="type-label">{playlist.isSmart ? 'Intelligent Playlist' : 'Playlist'}</span>
        <div class="title-with-actions">
          <h1>{playlist.name}</h1>
          {#if tracks.length > 0 && onshuffle}
            <button class="header-shuffle-btn" onclick={() => onshuffle(tracks)} title="Shuffle Play">
              <Icon name="shuffle" size={32} />
            </button>
          {/if}
        </div>
      </div>
    </div>

    <div class="tracks-list">
      {#if loading}
        <div class="status">Sourcing tracks...</div>
      {:else if error}
        <p class="error">{error}</p>
      {:else if tracks.length === 0}
        <div class="empty-state">
            <div class="empty-icon"><Icon name="music" size={64} /></div>
            <p>No se han encontrado canciones en esta lista. Haz clic en "Escanear Biblioteca" en Ajustes para comenzar.</p>
        </div>
      {:else}
        <TracksView 
          {tracks} 
          {currentPlaying} 
          onplay={onplay}
          onedit={onedit}
          onselectionChange={onselectionChange}
          onqueue={onqueue}
          onhide={onhide}
          ondeletePhysical={ondeletePhysical}
          showHeader={false}
        />
      {/if}
    </div>
  {/if}
</div>

<style>
  .playlist-view { width: 100%; padding: 0 0 2rem 0; }
  
  .playlist-header {
    display: flex;
    gap: 3rem;
    align-items: center;
    margin-bottom: 4rem;
  }

  .playlist-cover {
    width: 240px; height: 240px;
    display: flex; align-items: center; justify-content: center;
    background: var(--surface-container-high);
    border-radius: 32px;
    box-shadow: 0 30px 60px rgba(0,0,0,0.4);
    color: var(--primary);
    border: 1px solid var(--border-color);
  }
  .playlist-cover.is-smart {
    background: linear-gradient(135deg, var(--primary), var(--secondary));
    color: white;
    border: none;
    box-shadow: 0 30px 60px var(--primary-20);
  }

  h1 { 
    font-family: var(--font-headline); 
    font-size: clamp(2.5rem, 6vw, 4rem); 
    margin: 0; 
    letter-spacing: -0.04em; 
    font-weight: 950;
    line-height: 1;
  }
  .type-label {
    text-transform: uppercase;
    font-size: 0.8rem;
    font-weight: 900;
    letter-spacing: 2px;
    color: var(--primary);
  }

  .title-with-actions { display: flex; align-items: center; gap: 1.5rem; }
  
  .header-shuffle-btn {
    background: transparent;
    border: none;
    color: var(--primary);
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.6;
    padding: 10px;
    border-radius: 50%;
  }
  .header-shuffle-btn:hover {
    transform: scale(1.1) rotate(15deg);
    opacity: 1;
    background: var(--primary-10);
  }

  .tracks-list { margin-top: 1rem; }

  .status { padding: 4rem; text-align: center; color: var(--text-muted); font-weight: 700; }

  .empty {
    text-align: center;
    padding: 4rem;
    color: var(--text-muted);
    background: var(--surface-container-low);
    border-radius: 32px;
    border: 1px dashed var(--border-color);
    display: flex; flex-direction: column; align-items: center; gap: 1rem;
  }

  @media (max-width: 900px) {
    .playlist-header { 
      flex-direction: row; 
      gap: 1.5rem; 
      align-items: center; 
      text-align: left; 
      margin-bottom: 2rem;
    }
    .playlist-cover { 
      width: 100px; 
      height: 100px; 
      border-radius: 20px; 
      flex-shrink: 0;
    }
    .playlist-cover :global(svg) { width: 40px; height: 40px; }
    .title-with-actions { gap: 1rem; }
    h1 { font-size: 1.8rem; }
    .type-label { font-size: 0.65rem; }
  }
</style>
