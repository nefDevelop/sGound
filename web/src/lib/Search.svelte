<script>
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import { showToast } from "./stores.js";

  let { onselect, onplay, onsearch } = $props();

  let query = $state("");
  let results = $state({ tracks: [], artists: [], albums: [], genres: [], playlists: [] });
  let isOpen = $state(false);
  let isSearching = $state(false);
  let debounceTimer;

  async function performSearch() {
    if (!query || query.length < 2) {
      results = { tracks: [], artists: [], albums: [], genres: [], playlists: [] };
      isSearching = false;
      return;
    }

    isSearching = true;
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
      if (res.ok) {
        const data = await res.json();
        results = {
          tracks: data.tracks || [],
          artists: data.artists || [],
          albums: data.albums || [],
          genres: data.genres || [],
          playlists: data.playlists || []
        };
      }
    } catch (e) {
      console.error("Search error", e);
    } finally {
      isSearching = false;
    }
  }

  function handleInput() {
    isOpen = true;
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(performSearch, 300);
  }

  function handleSelect(type, item) {
    isOpen = false;
    query = "";
    onselect(type, item);
  }

  function handlePlay(item) {
    isOpen = false;
    query = "";
    onplay(item);
  }

  function handleKeyDown(e) {
    if (e.key === "Enter") {
      isOpen = false;
      onsearch(query);
    }
  }

  // Close when clicking outside
  let container;
  function handleClickOutside(e) {
    if (container && !container.contains(e.target)) {
      isOpen = false;
    }
  }

  onMount(() => {
    window.addEventListener("click", handleClickOutside);
    return () => window.removeEventListener("click", handleClickOutside);
  });

  let hasResults = $derived(
    (results?.tracks?.length || 0) > 0 || 
    (results?.artists?.length || 0) > 0 || 
    (results?.albums?.length || 0) > 0 ||
    (results?.genres?.length || 0) > 0 ||
    (results?.playlists?.length || 0) > 0
  );

</script>

<div class="search-container" bind:this={container}>
  <div class="input-wrapper">
    <div class="search-icon">
      <Icon name="search" size={18} />
    </div>
    <input
      type="text"
      placeholder="Buscar canciones, lanzamientos, géneros..."
      bind:value={query}
      oninput={handleInput}
      onkeydown={handleKeyDown}
      onfocus={() => { if(query) isOpen = true; }}
    />
    {#if query}
      <button class="clear-btn" onclick={() => { query = ""; results = {tracks:[], artists:[], albums:[], genres:[], playlists:[]}; isOpen = false; }}>
        <Icon name="x" size={16} />
      </button>
    {/if}
    {#if isSearching}
       <div class="loader"></div>
    {/if}
  </div>

  {#if isOpen && (hasResults || isSearching)}
    <div class="dropdown">
      {#if isSearching && !hasResults}
        <div class="status-msg">Buscando...</div>
      {:else if !hasResults}
        <div class="status-msg">No se encontraron resultados</div>
      {:else}
        <div class="results-scroll">
          {#if results.tracks.length > 0}
            <div class="section">
              <div class="section-header">Canciones</div>
              {#each results.tracks as track}
                <div class="result-item" onclick={() => handleSelect('track', track)}>
                  <div class="icon-box">
                    <Icon name="music" size={16} />
                  </div>
                  <div class="info">
                    <div class="title">{track.title}</div>
                    <div class="subtitle">{track.artist}</div>
                  </div>
                  <button class="play-action" onclick={(e) => { e.stopPropagation(); handlePlay(track); }}>
                    <Icon name="play" size={14} />
                  </button>
                </div>
              {/each}
            </div>
          {/if}

          {#if results.albums.length > 0}
            <div class="section">
              <div class="section-header">Álbumes</div>
              {#each results.albums as album}
                <div class="result-item" onclick={() => handleSelect('album', album)}>
                  <div class="icon-box disc">
                    <Icon name="disc" size={16} />
                  </div>
                  <div class="info">
                    <div class="title">{album.title}</div>
                    <div class="subtitle">{album.artist}</div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}

          {#if (results?.artists?.length || 0) > 0}
            <div class="section">
              <div class="section-header">Artistas</div>
              {#each results.artists as artist}
                <div class="result-item" onclick={() => handleSelect('artist', artist)}>
                  <div class="icon-box artist">
                    <Icon name="user" size={16} />
                  </div>
                  <div class="info">
                    <div class="title">{artist.name}</div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}

          {#if (results?.genres?.length || 0) > 0}
            <div class="section">
              <div class="section-header">Géneros</div>
              {#each results.genres as genre}
                <div class="result-item" onclick={() => handleSelect('genre', genre)}>
                  <div class="icon-box genre">
                    <Icon name="layers" size={16} />
                  </div>
                  <div class="info">
                    <div class="title">{genre}</div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}

          {#if (results?.playlists?.length || 0) > 0}
            <div class="section">
              <div class="section-header">Playlists</div>
              {#each results.playlists as pl}
                <div class="result-item" onclick={() => handleSelect('playlist', pl)}>
                  <div class="icon-box playlist">
                    <Icon name="list" size={16} />
                  </div>
                  <div class="info">
                    <div class="title">{pl.name}</div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .search-container {
    position: relative;
    width: 320px;
    z-index: 500;
  }

  .input-wrapper {
    display: flex;
    align-items: center;
    background: var(--surface-container-highest, #f0f0f0);
    padding: 0 1.5rem;
    height: 48px;
    border-radius: 100px;
    border: 1px solid var(--border-color, transparent);
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 2px 8px rgba(0,0,0,0.05);
  }

  .input-wrapper:focus-within {
    background: var(--surface-container-high, #fff);
    border-color: var(--primary);
    box-shadow: 0 4px 12px var(--primary-20);
    width: 400px;
  }

  input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    padding: 0 0.8rem;
    font-size: 0.9rem;
    color: var(--text-main);
    font-weight: 500;
  }

  input::placeholder {
    color: var(--text-muted);
    opacity: 0.45;
  }

  .search-icon {
    color: var(--text-muted);
    opacity: 0.4;
    display: flex;
    align-items: center;
    transition: opacity 0.2s;
  }

  .input-wrapper:focus-within .search-icon {
    opacity: 0.8;
    color: var(--primary);
  }

  .clear-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    border-radius: 50%;
  }

  .clear-btn:hover {
    background: var(--surface-container-highest);
    color: var(--text-main);
  }

  .loader {
    width: 16px;
    height: 16px;
    border: 2px solid var(--text-muted);
    border-top-color: var(--primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .dropdown {
    position: absolute;
    top: calc(100% + 8px);
    left: 0;
    width: 100%;
    min-width: 400px;
    max-height: 450px;
    background: var(--surface);
    border-radius: 16px;
    border: 1px solid var(--border-color);
    box-shadow: 0 12px 32px rgba(0,0,0,0.15);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    animation: dropIn 0.2s ease-out;
  }

  @keyframes dropIn {
    from { opacity: 0; transform: translateY(-10px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .results-scroll {
    overflow-y: auto;
    padding: 0.5rem;
  }

  .section {
    margin-bottom: 0.5rem;
  }

  .section-header {
    padding: 0.5rem 0.8rem;
    font-size: 0.7rem;
    font-weight: 800;
    text-transform: uppercase;
    color: var(--text-muted);
    letter-spacing: 0.05em;
  }

  .result-item {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    padding: 0.6rem 0.8rem;
    border-radius: 10px;
    cursor: pointer;
    transition: background 0.2s;
  }

  .result-item:hover {
    background: var(--surface-container-highest);
  }

  .icon-box {
    width: 32px;
    height: 32px;
    background: var(--primary-10);
    color: var(--primary);
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .icon-box.disc { color: #8b5cf6; background: rgba(139, 92, 246, 0.1); }
  .icon-box.artist { color: #f59e0b; background: rgba(245, 158, 11, 0.1); }
  .icon-box.genre { color: #10b981; background: rgba(16, 185, 129, 0.1); }
  .icon-box.playlist { color: #3b82f6; background: rgba(59, 130, 246, 0.1); }

  .info {
    flex: 1;
    min-width: 0;
  }

  .title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .subtitle {
    font-size: 0.75rem;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .play-action {
    opacity: 0;
    background: var(--primary);
    color: white;
    border: none;
    border-radius: 50%;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s;
    flex-shrink: 0;
  }

  .result-item:hover .play-action {
    opacity: 1;
  }

  .play-action:hover {
    transform: scale(1.1);
    box-shadow: 0 4px 12px var(--primary-40);
  }

  .status-msg {
    padding: 2rem;
    text-align: center;
    color: var(--text-muted);
    font-size: 0.9rem;
  }
</style>
