<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';
  import TracksView from './TracksView.svelte';

  export let playlist = { id: '', name: '' };
  export let currentPlaying = null;
  
  const dispatch = createEventDispatcher();
  let tracks = [];
  let loading = true;
  let error = null;

  $: if (playlist) {
    fetchPlaylistTracks();
  }

  async function fetchPlaylistTracks() {
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
    if (tracks.length > 0) {
       dispatch('play', tracks[0]);
    }
  }
</script>

<div class="playlist-view" in:fade>
  {#if playlist}
    <div class="playlist-header">
      <div class="playlist-cover glass">
          <Icon name="list" size={64} color="rgba(255,255,255,0.2)" />
      </div>
      <div class="playlist-info">
        <span class="type-label">Playlist</span>
        <h1>{playlist.name}</h1>
        <p class="stats">{tracks.length} tracks</p>
        
        <div class="actions">
          <button class="play-btn" on:click={handlePlayAll}>
            <Icon name="play" size={20} color="black" />
            Play
          </button>
        </div>
      </div>
    </div>

    <div class="tracks-list">
      {#if loading}
        <p>Loading tracks...</p>
      {:else if error}
        <p class="error">{error}</p>
      {:else if tracks.length === 0}
        <div class="empty">
            <p>No tracks in this playlist.</p>
        </div>
      {:else}
        <TracksView 
          {tracks} 
          {currentPlaying} 
          on:play 
          on:edit 
          on:selectionChange 
          on:queue
          on:delete
        />
      {/if}
    </div>
  {/if}
</div>

<style>
  .playlist-view {
    padding: 0;
  }

  .playlist-header {
    display: flex;
    gap: 2rem;
    align-items: flex-end;
    margin-bottom: 3rem;
  }

  .playlist-cover {
    width: 230px;
    height: 230px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #333 0%, #111 100%);
    border-radius: 24px;
    box-shadow: 0 20px 50px rgba(0,0,0,0.4);
  }

  .playlist-info h1 {
    font-size: 4rem;
    margin: 0.5rem 0;
    font-weight: 900;
    font-family: var(--font-headline);
    letter-spacing: -2px;
  }

  .type-label {
    text-transform: uppercase;
    font-size: 0.8rem;
    font-weight: 700;
    letter-spacing: 1px;
    color: var(--primary);
  }

  .stats {
    color: var(--text-muted);
    margin-bottom: 1.5rem;
    font-weight: 600;
  }

  .play-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 0.8rem 2.5rem;
    border-radius: 100px;
    font-weight: 800;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
    transition: transform 0.2s, background 0.2s;
    box-shadow: 0 4px 15px var(--primary-10);
  }
  .play-btn:hover { transform: scale(1.05); background: var(--primary-80); }

  .tracks-list {
    margin-top: 1rem;
  }

  .empty {
    text-align: center;
    padding: 4rem;
    color: var(--text-muted);
    background: rgba(255,255,255,0.02);
    border-radius: var(--radius);
    border: 1px dashed var(--border-color);
  }
</style>

