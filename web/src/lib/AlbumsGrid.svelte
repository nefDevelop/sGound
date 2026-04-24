<script>
  import { inboxCount, rawCount } from './stores.js';
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import Artwork from './Artwork.svelte';
  import Skeleton from './Skeleton.svelte';
  
  let { artistId = null, onselectAlbum, onplayAlbum, onqueueAlbum, oneditAlbum, onnavinbox = null } = $props();
  
  let albums = $state([]);
  let loading = $state(true);

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    album: null,
    items: []
  });

  function showContextMenu(album, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      album,
      items: [
        { label: 'Play Album', icon: 'play', action: () => onplayAlbum?.(album.id) },
        { label: 'Queue Album', icon: 'plus', action: () => onqueueAlbum?.(album.id) },
        { divider: true },
        { label: 'Edit Tags', icon: 'edit', action: () => oneditAlbum?.(album) },
        { label: 'Copy Title', icon: 'fileText', action: () => navigator.clipboard.writeText(album.title) }
      ]
    };
  }

  $effect(() => {
    fetchAlbums(artistId);
  });

  function playRandomAlbum() {
    if (albums.length > 0 && onplayAlbum) {
      const randomIndex = Math.floor(Math.random() * albums.length);
      onplayAlbum(albums[randomIndex].id);
    }
  }

  async function fetchAlbums(id) {
    loading = true;
    try {
      const url = id ? `/api/artists/${id}/albums` : '/api/albums';
      const res = await fetch(url);
      albums = (await res.json()) || [];
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="albums-container">
  <header class="section-header">
    <div class="title-row">
      <h1>Albums</h1>
      {#if albums.length > 0 && onplayAlbum}
        <button class="subtle-shuffle-btn" onclick={playRandomAlbum} title="Reproducir álbum aleatorio">
          <Icon name="shuffle" size={24} />
        </button>
      {/if}
    </div>
    {#if albums.length > 0}
      <div class="chip tertiary" title="Total de álbumes">
        <Icon name="disc" size={14} />
        {albums.length}
      </div>
    {/if}
  </header>

  {#if loading}
    <div class="albums-grid">
      {#each Array(8) as _}
        <div class="album-card skeleton-card">
          <Skeleton width="100%" height="auto" borderRadius="24px" style="aspect-ratio: 1" />
          <div class="album-info">
            <Skeleton width="80%" height="1.1rem" borderRadius="4px" margin="0.5rem 0 0.3rem" />
            <Skeleton width="50%" height="0.9rem" borderRadius="4px" />
          </div>
        </div>
      {/each}
    </div>
  {:else if albums.length === 0}
    <div class="empty-state">
      {#if ($inboxCount > 0 || $rawCount > 0) && !artistId}
        <div class="inbox-alert">
          <Icon name="inbox" size={48} />
          <p>Tu biblioteca de álbumes está vacía, pero tienes <b>{$inboxCount + $rawCount}</b> canciones en el Inbox pendientes de aprobar.</p>
          <button class="primary-btn" onclick={onnavinbox}>Ir al Inbox</button>
        </div>
      {:else}
        <div class="empty-icon"><Icon name="disc" size={64} /></div>
        <p>No se han encontrado álbumes. Haz clic en "Escanear Biblioteca" en Ajustes para comenzar.</p>
      {/if}
    </div>
  {:else}
    <div class="albums-grid">
      {#each albums as album}
        <div 
          class="album-card" 
          onclick={() => onselectAlbum?.(album.id)}
          oncontextmenu={(e) => showContextMenu(album, e)}
          role="button"
          tabindex="0"
          onkeydown={(e) => e.key === 'Enter' && onselectAlbum?.(album.id)}
        >
          <div class="cover-wrapper">
            <Artwork 
              src={`/api/cover/${album.coverId}`} 
              alt={album.title} 
            />
            <div class="play-overlay">
              <button 
                class="play-pill" 
                onclick={(e) => {
                  e.stopPropagation();
                  onplayAlbum?.(album.id);
                }}
                aria-label="Play album"
              >
                <Icon name="play" size={20} />
              </button>
            </div>
          </div>
          <div class="album-info">
            <span class="title">{album.title}</span>
            <span class="artist">{album.artist}</span>
            {#if album.year}
              <span class="year">{album.year}</span>
            {/if}
          </div>
        </div>
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
  .albums-container { width: 100%; padding: 0 0 3rem 0; animation: fadeIn 0.4s ease-out; }

  .albums-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 1.5rem;
  }

  .album-card {
    background: transparent;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
    padding: 10px;
    border-radius: 24px;
    border: 1px solid transparent;
    outline: none;
  }

  .album-card:hover {
    transform: translateY(-6px);
    background: var(--surface-container-high);
    border-color: var(--border-color);
  }

  .cover-wrapper {
    position: relative;
    aspect-ratio: 1;
    width: 100%;
    border-radius: 24px;
    overflow: hidden;
    box-shadow: 0 8px 30px rgba(0,0,0,0.4);
  }

  .play-overlay {
    position: absolute;
    top: 0; left: 0; width: 100%; height: 100%;
    background: linear-gradient(to top, rgba(0,0,0,0.4), transparent 40%);
    display: flex;
    align-items: flex-end;
    justify-content: flex-end;
    padding: 1rem;
    opacity: 0;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .album-card:hover .play-overlay { opacity: 1; }

  .play-pill {
    background: var(--primary);
    color: white;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    cursor: pointer;
    box-shadow: 0 4px 15px rgba(0,0,0,0.4);
    transform: scale(0.8);
    transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .album-card:hover .play-pill { transform: scale(1); }
  .play-pill:hover { transform: scale(1.1) !important; filter: brightness(1.2); }

  .album-info {
    display: flex;
    flex-direction: column;
    padding: 0 8px;
  }

  .title {
    color: white;
    font-weight: 700;
    font-size: 1.1rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .artist {
    color: var(--text-muted);
    font-size: 0.9rem;
    font-weight: 500;
  }

  .year {
    color: var(--text-muted);
    font-size: 0.8rem;
    margin-top: 2px;
    opacity: 0.7;
  }

  @media (max-width: 768px) {
    .albums-grid {
      grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
      gap: 1rem;
    }
    .album-card { border-radius: 20px; padding: 6px; }
    .cover-wrapper { border-radius: 16px; }
    .title { font-size: 1rem; }
  }
</style>
