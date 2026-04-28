<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import AlbumsGrid from './AlbumsGrid.svelte';
  import { fade, slide } from 'svelte/transition';
  
  let { artistId, onselectAlbum, onplayAlbum, onqueueAlbum, oneditAlbum, onshuffle } = $props();
  
  let artist = $state(null);
  let loading = $state(true);
  let showFullBio = $state(false);

  async function handleShuffle() {
    try {
      const res = await fetch(`/api/tracks?artistId=${artistId}`);
      if (res.ok) {
        const tracks = await res.json();
        onshuffle?.(tracks);
      }
    } catch (e) {
      console.error(e);
    }
  }

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
    return `/api/artists/${encodeURIComponent(id)}/image`;
  }
</script>

<div class="artist-detail" in:fade>
  {#if loading}
    <div class="status">
      <div class="spinner-large"></div>
      <p>Cargando detalles del artista...</p>
    </div>
  {:else if artist}
    <header class="artist-header">
       <!-- Dynamic Background Banner -->
        {#if artist.imageUrl}
          <div class="header-bg" style="background-image: url({getAvatarUrl(artist.id)})"></div>
        {/if}
        <div class="header-overlay"></div>
 
        <div class="hero">
          <div class="avatar">
            <img 
              class="smooth-image"
              src={getAvatarUrl(artist.id)} 
              alt={artist.name} 
              onload={(e) => { if (e.target instanceof HTMLImageElement) e.target.classList.add('loaded'); }}
              onerror={(e) => {
                if (artist.imageUrl && e.target instanceof HTMLImageElement) e.target.src = artist.imageUrl;
              }}
            />
          </div>
         <div class="info">
           <span class="type">Artista</span>
            <div class="title-row">
              <h1>{artist.name}</h1>
              <button class="subtle-shuffle-btn" onclick={handleShuffle} title="Reproducción Aleatoria">
                <Icon name="shuffle" size={32} />
              </button>
            </div>
           <div class="stats">
             <span><b>{artist.albumCount || 0}</b> Álbumes</span>
             <span class="dot">•</span>
             <span><b>{artist.trackCount || 0}</b> Canciones</span>
           </div>
           
           {#if artist.tags && artist.tags.length > 0}
             <div class="artist-tags">
               {#each artist.tags.slice(0, 5) as tag}
                 <span class="tag-pill">{tag}</span>
               {/each}
             </div>
           {/if}
         </div>
        </div>
        {#if artist.imageSource}
          <div class="attribution">
            <Icon name="camera" size={12} />
            <span>Imagen de {artist.imageSource}</span>
          </div>
        {/if}
     </header>

    <div class="detail-grid">
      <div class="main-content">
        <div class="section-title">
          <h2>Discografía</h2>
        </div>
        <AlbumsGrid 
          {artistId} 
          onselectAlbum={onselectAlbum}
          onplayAlbum={onplayAlbum}
          onqueueAlbum={onqueueAlbum}
          oneditAlbum={oneditAlbum}
          onnavinbox={() => {}}
          showHeader={false}
        />
      </div>

      <aside class="sidebar-info">
        {#if artist.biography}
          <div class="bio-card">
            <h3>Sobre el artista</h3>
            <div class="bio-text" class:expanded={showFullBio}>
              {artist.biography}
            </div>
            {#if artist.biography.length > 300}
              <button class="bio-toggle" onclick={() => showFullBio = !showFullBio}>
                {showFullBio ? 'Leer menos' : 'Leer biografía completa'}
              </button>
            {/if}
          </div>
        {/if}

        {#if artist.similar && artist.similar.length > 0}
          <div class="similar-card">
            <h3>Artistas Similares</h3>
            <div class="similar-list">
              {#each artist.similar.slice(0, 6) as sim}
                <span class="similar-item">{sim}</span>
              {/each}
            </div>
          </div>
        {/if}

        <div class="info-card">
          <h3>Detalles de la biblioteca</h3>
          <div class="info-item">
            <span class="label">ID de Artista</span>
            <span class="value">{artist.id}</span>
          </div>
          <div class="info-item">
            <span class="label">Tracks totales</span>
            <span class="value">{artist.trackCount}</span>
          </div>
        </div>
      </aside>
    </div>
  {/if}
</div>

<style>
  .artist-detail { width: 100%; max-width: 1400px; margin: 0 auto; padding-bottom: 5rem; }
  
  .artist-header {
    height: 450px;
    position: relative;
    display: flex;
    align-items: flex-end;
    padding: 3.5rem;
    margin-bottom: 3rem;
    border-radius: 48px;
    overflow: hidden;
    background: var(--surface-container);
  }

  .header-bg {
    position: absolute;
    inset: 0;
    background-size: cover;
    background-position: center 20%;
    filter: blur(20px) brightness(0.4) saturate(1.2);
    transform: scale(1.1);
    z-index: 0;
  }

  .header-overlay {
    position: absolute;
    inset: 0;
    background: linear-gradient(to top, var(--surface-00) 0%, transparent 100%);
    z-index: 1;
  }

  .hero { display: flex; align-items: center; gap: 3.5rem; z-index: 2; width: 100%; }
  
  .avatar {
    width: 220px;
    height: 220px;
    border-radius: 50%;
    overflow: hidden;
    box-shadow: 0 30px 60px rgba(0,0,0,0.6);
    border: 6px solid rgba(255,255,255,0.1);
    flex-shrink: 0;
  }
  .avatar img { width: 100%; height: 100%; object-fit: cover; }

  .info { display: flex; flex-direction: column; gap: 0.5rem; }
  .type { text-transform: uppercase; font-size: 0.85rem; font-weight: 950; letter-spacing: 3px; color: var(--primary); }
  .title-row { display: flex; align-items: center; gap: 2rem; }
  h1 { font-size: 4.5rem; font-weight: 1000; margin: 0; letter-spacing: -3px; line-height: 1; color: white; text-shadow: 0 10px 30px rgba(0,0,0,0.5); }
  
  .subtle-shuffle-btn {
    background: rgba(255,255,255,0.1);
    color: white;
    padding: 12px;
    border-radius: 50%;
    transition: all 0.3s;
    cursor: pointer;
    border: none;
    backdrop-filter: blur(10px);
  }
  .subtle-shuffle-btn:hover {
    background: var(--primary);
    transform: scale(1.1) rotate(15deg);
  }
  
  .stats { display: flex; gap: 1.2rem; font-size: 1.2rem; color: rgba(255,255,255,0.7); font-weight: 600; }
  .dot { opacity: 0.5; }

  .artist-tags { display: flex; gap: 0.8rem; margin-top: 1rem; }
  .tag-pill {
    background: rgba(255,255,255,0.1);
    color: white;
    padding: 0.4rem 1rem;
    border-radius: 100px;
    font-size: 0.8rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255,255,255,0.1);
  }

  .attribution {
    position: absolute;
    bottom: 2rem;
    right: 2.5rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 1.2rem;
    background: var(--surface-container-highest);
    backdrop-filter: blur(10px);
    border-radius: 100px;
    font-size: 0.7rem;
    font-weight: 800;
    color: var(--text-main);
    z-index: 10;
    text-transform: uppercase;
    letter-spacing: 1px;
    border: 1px solid var(--white-05);
    box-shadow: 0 4px 15px rgba(0,0,0,0.3);
  }

  .detail-grid {
    display: grid;
    grid-template-columns: 1fr 350px;
    gap: 3rem;
    padding: 0 1rem;
  }

  .section-title h2 { font-size: 2rem; font-weight: 900; letter-spacing: -1px; margin-bottom: 2rem; }

  .sidebar-info { display: flex; flex-direction: column; gap: 2rem; }
  
  .bio-card, .info-card, .similar-card {
    background: var(--surface-container-low);
    padding: 2rem;
    border-radius: 32px;
    border: 1px solid var(--white-05);
  }

  .bio-card h3, .info-card h3, .similar-card h3 { 
    margin: 0 0 1.2rem 0; 
    font-size: 1.2rem; 
    font-weight: 800; 
    color: var(--primary);
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  .similar-list { display: flex; flex-wrap: wrap; gap: 0.8rem; }
  .similar-item {
    background: var(--surface-container-high);
    padding: 0.6rem 1.2rem;
    border-radius: 16px;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text-main);
    border: 1px solid var(--white-05);
  }

  .bio-text {
    font-size: 1rem;
    line-height: 1.6;
    color: var(--text-muted);
    display: -webkit-box;
    -webkit-line-clamp: 6;
    line-clamp: 6;
    -webkit-box-orient: vertical;
    overflow: hidden;
    transition: all 0.5s ease;
  }
  .bio-text.expanded { -webkit-line-clamp: unset; line-clamp: unset; }

  .bio-toggle {
    background: none;
    border: none;
    color: var(--primary);
    font-weight: 700;
    cursor: pointer;
    margin-top: 1rem;
    padding: 0;
    font-size: 0.9rem;
  }
  .bio-toggle:hover { text-decoration: underline; }

  .info-item {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    margin-bottom: 1.2rem;
  }
  .info-item:last-child { margin-bottom: 0; }
  .label { font-size: 0.8rem; font-weight: 700; opacity: 0.5; text-transform: uppercase; }
  .value { font-family: var(--font-mono); font-size: 0.9rem; color: var(--text-main); }

  .status { text-align: center; padding: 10rem 0; color: var(--text-muted); }
  .spinner-large { 
    width: 50px; height: 50px; border: 4px solid var(--white-10); border-top-color: var(--primary); 
    border-radius: 50%; animation: spin 1s linear infinite; margin: 0 auto 1.5rem;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  @media (max-width: 1100px) {
    .detail-grid { grid-template-columns: 1fr; }
    .sidebar-info { order: -1; }
    .artist-header { height: auto; padding: 3rem 2rem; flex-direction: column; text-align: center; }
    .hero { flex-direction: column; gap: 2rem; }
    h1 { font-size: 3.5rem; }
    .avatar { width: 180px; height: 180px; }
  }
</style>
