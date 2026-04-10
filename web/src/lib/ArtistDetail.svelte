<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import AlbumsGrid from './AlbumsGrid.svelte';
  import { fade } from 'svelte/transition';
  
  let { artistId, onselectAlbum, onplayAlbum, onqueueAlbum, oneditAlbum } = $props();
  
  let artist = $state(null);
  let loading = $state(true);

  onMount(async () => {
    loading = true;
    try {
      const res = await fetch(`/api/artists/${artistId}`);
      if (res.ok) artist = await res.json();
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  });

  function getAvatarUrl(id) {
    return `/api/artists/${id}/image`;
  }
</script>

<div class="artist-detail" in:fade>
  {#if loading}
    <div class="status">Loading artist details...</div>
  {:else if artist}
    <header class="artist-header">
       <div class="hero">
         <div class="avatar">
           <img src={getAvatarUrl(artist.id)} alt={artist.name} />
         </div>
         <div class="info">
           <span class="type">Artist</span>
           <h1>{artist.name}</h1>
           <div class="stats">
             <span><b>{artist.albumCount || 0}</b> Albums</span>
             <span class="dot">•</span>
             <span><b>{artist.trackCount || 0}</b> Tracks</span>
           </div>
         </div>
       </div>
    </header>

    <div class="section-content">
       <AlbumsGrid 
         {artistId} 
         onselectAlbum={onselectAlbum}
         onplayAlbum={onplayAlbum}
         onqueueAlbum={onqueueAlbum}
         oneditAlbum={oneditAlbum}
       />
    </div>
  {/if}
</div>

<style>
  .artist-detail { width: 100%; max-width: 1400px; margin: 0 auto; padding-bottom: 5rem; }
  
  .artist-header {
    height: 400px;
    position: relative;
    display: flex;
    align-items: flex-end;
    padding: 3rem;
    margin-bottom: 3rem;
    background: linear-gradient(to top, var(--surface-container), transparent);
    border-radius: 40px;
    overflow: hidden;
  }

  .hero { display: flex; align-items: center; gap: 3rem; z-index: 2; }
  
  .avatar {
    width: 200px;
    height: 200px;
    border-radius: 50%;
    overflow: hidden;
    box-shadow: 0 20px 50px rgba(0,0,0,0.4);
    border: 4px solid var(--surface-container-highest);
  }
  .avatar img { width: 100%; height: 100%; object-fit: cover; }

  .info { display: flex; flex-direction: column; gap: 0.2rem; }
  .type { text-transform: uppercase; font-size: 0.8rem; font-weight: 950; letter-spacing: 2px; color: var(--primary); opacity: 0.8; }
  h1 { font-size: 3.5rem; font-weight: 950; margin: 0; letter-spacing: -2px; line-height: 1.1; color: var(--text-main); }
  
  .stats { display: flex; gap: 1rem; font-size: 1.1rem; color: var(--text-muted); }
  .dot { opacity: 0.4; }

  .status { text-align: center; padding: 5rem; color: var(--text-muted); }

  @media (max-width: 900px) {
    .artist-header { height: auto; padding: 2rem; flex-direction: column; align-items: center; text-align: center; border-radius: 32px; margin-bottom: 2rem; }
    .hero { flex-direction: column; gap: 1.5rem; }
    h1 { font-size: 2.8rem; letter-spacing: -1.5px; }
    .avatar { width: 160px; height: 160px; }
  }

  @media (max-width: 600px) {
    .artist-header { padding: 1.5rem 1rem; }
    h1 { font-size: 2.2rem; }
    .stats { font-size: 0.9rem; justify-content: center; }
    .avatar { width: 120px; height: 120px; border-width: 3px; }
  }
</style>
