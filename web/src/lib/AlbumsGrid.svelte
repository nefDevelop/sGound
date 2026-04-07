<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  
  const dispatch = createEventDispatcher();
  let albums = [];
  let loading = true;

  let contextMenu = {
    visible: false,
    x: 0,
    y: 0,
    album: null,
    items: []
  };

  function showContextMenu(album, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      album,
      items: [
        { label: 'Reproducir álbum', icon: 'play', action: () => dispatch('playAlbum', album.id) },
        { label: 'Añadir álbum a cola', icon: 'plus', action: () => dispatch('queueAlbum', album.id) },
        { divider: true },
        { label: 'Editar metadatos', icon: 'edit', action: () => dispatch('editAlbum', album) },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(album.title) }
      ]
    };
  }

  onMount(async () => {
    try {
      const res = await fetch('/api/albums');
      albums = await res.json() || [];
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  });
</script>

<div class="albums-container">
  {#if loading}
    <p class="status">Loading albums...</p>
  {:else if albums.length === 0}
    <p class="status">No albums found in your library.</p>
  {:else}
    <div class="albums-grid">
      {#each albums as album}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div 
          class="album-card" 
          on:click={() => dispatch('selectAlbum', album.id)}
          on:contextmenu={(e) => showContextMenu(album, e)}
        >
          <div class="cover-wrapper">
            <img class="cover" src={`/api/cover/${album.coverId}`} alt={album.title} 
                 on:error={(e) => (e.currentTarget.src = 'https://placehold.co/300x300?text=Album')} />
            <div class="play-overlay">
              <div class="play-pill"><Icon name="play" color="white" size={20} /></div>
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
    on:close={() => contextMenu.visible = false}
  />
</div>

<style>
  .albums-container { width: 100%; padding: 1rem 0; }
  
  .status { color: #64748b; text-align: center; margin-top: 4rem; }

  .albums-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
  }

  /* MD3 + GNOME Style Album Card */
  .album-card {
    background: transparent;
    cursor: pointer;
    transition: transform 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 12px;
    border-radius: 32px; /* MD3 Expressive Corners */
  }

  .album-card:hover {
    transform: translateY(-8px);
    background: rgba(255, 255, 255, 0.05); /* GNOME surface hover */
  }

  .cover-wrapper {
    position: relative;
    aspect-ratio: 1;
    width: 100%;
    border-radius: 24px; /* Slightly tighter than the card */
    overflow: hidden;
    box-shadow: 0 8px 30px rgba(0,0,0,0.4); /* MD3 shadow depth */
  }

  .cover {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .play-overlay {
    position: absolute;
    top: 0; left: 0; width: 100%; height: 100%;
    background: rgba(0,0,0,0.3);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.2s;
  }

  .album-card:hover .play-overlay { opacity: 1; }

  .play-pill {
    background: #ff3366; /* Accent Color */
    color: white;
    width: 48px;
    height: 48px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.2rem;
    box-shadow: 0 4px 15px rgba(255, 51, 102, 0.4);
    transform: scale(0.8);
    transition: transform 0.2s;
  }

  .album-card:hover .play-pill { transform: scale(1); }

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
    color: #94a3b8;
    font-size: 0.9rem;
    font-weight: 500;
  }

  .year {
    color: #64748b;
    font-size: 0.8rem;
    margin-top: 2px;
  }
</style>
