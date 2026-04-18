<script>
  import { inboxCount } from './stores.js';
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';

  let { onselectArtist, onnavinbox } = $props();

  let artists = $state([]);
  let searchTerm = $state('');
  let loading = $state(true);

  let filteredArtists = $derived(
    artists.filter(a => a.name.toLowerCase().includes(searchTerm.toLowerCase()))
  );

  onMount(async () => {
    loading = true;
    try {
      const res = await fetch('/api/artists');
      if (res.ok) {
        artists = (await res.json()) || [];
      }
    } catch (e) {
      console.error("Failed to load artists", e);
    } finally {
      loading = false;
    }
  });

  function getAvatarUrl(artist) {
    return `/api/artists/${artist.id}/image`; 
  }

  function handleImageError(e, artist) {
    // Si la imagen local falla y tenemos una URL externa, la probamos
    if (artist.imageUrl && e.target.src !== artist.imageUrl) {
      e.target.src = artist.imageUrl;
    } else {
      // Si todo falla, mostrar el fallback
      e.target.style.display = 'none';
      const fallback = e.target.nextElementSibling;
      if (fallback) fallback.style.display = 'flex';
    }
  }

  function playRandomArtist() {
    if (filteredArtists.length > 0 && onselectArtist) {
      const randomIndex = Math.floor(Math.random() * filteredArtists.length);
      onselectArtist(filteredArtists[randomIndex]);
    }
  }
</script>

<div class="artists-view" in:fade>
  <header class="section-header">
    <div class="title-row">
      <h1>Artistas</h1>
      {#if filteredArtists.length > 0}
        <button class="subtle-shuffle-btn" onclick={playRandomArtist} title="Ir a un artista aleatorio">
          <Icon name="shuffle" size={24} />
        </button>
      {/if}
    </div>
    {#if artists.length > 0}
      <div class="chip secondary" title="Total de artistas">
        <Icon name="users" size={14} />
        {artists.length}
      </div>
    {/if}
  </header>

  {#if loading}
    <div class="status-box">Sourcing artist profiles...</div>
  {:else if filteredArtists.length === 0}
    <div class="empty-state">
      {#if $inboxCount > 0 && !searchTerm}
        <div class="inbox-alert">
          <Icon name="inbox" size={48} />
          <p>Aún no tienes artistas en tu biblioteca, pero tienes <b>{$inboxCount}</b> canciones en el Inbox esperando.</p>
          <button class="primary-btn" onclick={onnavinbox}>Ir al Inbox</button>
        </div>
      {:else}
        <div class="empty-icon">
          <Icon name="users" size={64} />
        </div>
        <p>No se han encontrado artistas. Haz clic en "Escanear Biblioteca" en Ajustes para comenzar.</p>
      {/if}
    </div>
  {:else}
    <div class="artist-grid">
      {#each filteredArtists as artist}
        <button class="artist-card" onclick={() => onselectArtist?.(artist)}>
          <div class="avatar-container">
            <img 
              class="smooth-image"
              src={getAvatarUrl(artist)} 
              alt={artist.name} 
              onload={(e) => { if (e.target instanceof HTMLImageElement) e.target.classList.add('loaded'); }}
              onerror={(e) => handleImageError(e, artist)} 
            />
            <div class="avatar-fallback" style="display: none;">
              <Icon name="users" size={40} />
            </div>
          </div>
          <span class="artist-name">{artist.name}</span>
          <span class="album-count">{artist.albumCount || 0} álbumes</span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .artists-view { width: 100%; max-width: 1400px; margin: 0 auto; }

  .artist-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 2rem;
  }

  .artist-card {
    background: var(--surface-container);
    padding: 1.8rem;
    border-radius: 32px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    cursor: pointer;
    transition: all 0.4s cubic-bezier(0.2, 0, 0, 1);
    border: none;
    appearance: none;
    color: inherit;
  }
  .artist-card:hover { transform: translateY(-8px); background: var(--surface-container-high); box-shadow: 0 20px 40px rgba(0,0,0,0.2); }

  .avatar-container {
    width: 140px;
    height: 140px;
    border-radius: 50%;
    margin-bottom: 1.5rem;
    overflow: hidden;
    background: var(--surface-container-highest);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 10px 25px rgba(0,0,0,0.3);
    border: 3px solid transparent;
    transition: all 0.4s;
  }
  .artist-card:hover .avatar-container { border-color: var(--primary); transform: scale(1.05); }

  .avatar-container img { width: 100%; height: 100%; object-fit: cover; }
  .avatar-fallback { color: var(--text-muted); opacity: 0.5; width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; }

  .artist-name { 
    font-weight: 900; 
    font-size: 1.1rem; 
    color: var(--text-main); 
    margin-bottom: 0.2rem; 
    display: block; 
    width: 100%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .album-count { font-size: 0.75rem; font-weight: 700; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }

  .status-box { padding: 4rem; text-align: center; color: var(--text-muted); font-weight: 700; }

  @media (max-width: 900px) {
    .artist-grid { grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 1rem; }
    .artist-card { padding: 1.2rem; border-radius: 24px; }
    .avatar-container { width: 100px; height: 100px; }
  }

  .faded-icon { opacity: 0.2; }
</style>
