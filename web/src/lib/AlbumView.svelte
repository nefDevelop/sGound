<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import Artwork from './Artwork.svelte';
  import TracksView from './TracksView.svelte';
  import { fade } from 'svelte/transition';
  
  let { albumId, onplay, onback, onedit } = $props();
  
  let album = $state(null);
  let tracks = $state([]);
  let loading = $state(true);

  onMount(fetchAlbumDetails);

  async function fetchAlbumDetails() {
    loading = true;
    try {
      // Get album metadata
      const resAlbum = await fetch(`/api/albums/${albumId}`);
      if (resAlbum.ok) album = await resAlbum.json();
      
      // Get tracks
      const resTracks = await fetch(`/api/albums/${albumId}/tracks`);
      if (resTracks.ok) tracks = await resTracks.json();
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  function formatDuration(seconds) {
    const min = Math.floor(seconds / 60);
    const sec = Math.floor(seconds % 60);
    return `${min}:${sec.toString().padStart(2, '0')}`;
  }

  function playAll() {
    if (tracks.length > 0) {
      onplay?.({ track: tracks[0], tracks });
    }
  }
</script>

<div class="album-view" in:fade>
  {#if loading}
    <div class="status">Loading album details...</div>
  {:else if album}
    <header class="album-header">
       <div class="cover-art">
         <Artwork src={`/api/cover/${album.coverId}`} alt={album.title} borderRadius="32px" />
       </div>
       <div class="meta">
         <span class="type-label">Album</span>
         <h1>{album.title}</h1>
         <div class="artist-row">
            <span class="artist-name">{album.artist}</span>
            {#if album.year}
              <span class="dot">•</span>
              <span class="year">{album.year}</span>
            {/if}
            <span class="dot">•</span>
            <span class="count">{tracks.length} tracks</span>
         </div>
         <div class="actions">
            <button class="play-btn" onclick={playAll}>
              <Icon name="play" size={24} />
              Play All
            </button>
            <button class="secondary-btn" onclick={() => onedit?.(album)}>
              <Icon name="edit" size={20} />
            </button>
         </div>
       </div>
    </header>

    <div class="tracks-list">
      <TracksView 
        {tracks} 
        columns={['num', 'title', 'duration']} 
        showHeader={false}
        onplay={(track) => onplay?.({ track, tracks })}
        onedit={onedit}
      />
    </div>
  {/if}
</div>

<style>
  .album-view { width: 100%; max-width: 1200px; margin: 0 auto; padding: 1rem 0 5rem 0; }
  
  .album-header {
    display: flex;
    gap: 3rem;
    align-items: flex-end;
    margin-bottom: 4rem;
    padding: 0 1rem;
  }

  .cover-art {
    width: 280px;
    height: 280px;
    flex-shrink: 0;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
    border-radius: 32px;
  }

  .meta { display: flex; flex-direction: column; gap: 0.5rem; }
  .meta { display: flex; flex-direction: column; gap: 0.2rem; align-self: center; }
  .type-label { text-transform: uppercase; font-size: 0.8rem; font-weight: 950; letter-spacing: 2px; color: var(--primary); opacity: 0.8; }
  
  h1 { font-size: 2.8rem; font-weight: 950; margin: 0; letter-spacing: -1.5px; opacity: 1; line-height: 1.1; color: var(--text-main); }
  
  .artist-row { display: flex; align-items: center; gap: 0.8rem; font-size: 1.1rem; font-weight: 600; color: var(--text-muted); }
  .artist-name { color: var(--text-main); font-weight: 800; cursor: pointer; }
  .artist-name:hover { text-decoration: underline; }
  .dot { opacity: 0.4; }

  .actions { display: flex; gap: 1rem; margin-top: 1.5rem; }
  .play-btn { 
    background: var(--primary); 
    color: white; 
    border: none; 
    padding: 1rem 2.5rem; 
    border-radius: 100px; 
    font-weight: 900; 
    font-size: 1.1rem;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    cursor: pointer;
    transition: all 0.2s;
    box-shadow: 0 8px 25px var(--primary-20);
  }
  .play-btn:hover { transform: scale(1.05); filter: brightness(1.1); }

  .secondary-btn {
    background: var(--surface-container-highest);
    color: var(--text-main);
    border: 1px solid var(--border-color);
    width: 56px; height: 56px;
    border-radius: 50%;
    display: flex; align-items: center; justify-content: center;
    cursor: pointer;
    transition: all 0.2s;
  }
  .secondary-btn:hover { background: var(--surface-container-highest-hover); transform: scale(1.05); }

</style>
