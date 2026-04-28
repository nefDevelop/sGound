<script>
  import { onMount } from "svelte";
  import Icon from "./Icon.svelte";
  import { showToast, coverRefreshTrigger } from "./stores.js";
  import { fade, scale } from "svelte/transition";

  let {
    isOpen = $bindable(false),
    albumId = "",
    initialArtist = "",
    initialAlbum = "",
    onapply = null,
    onclose = null,
  } = $props();

  let artist = $state("");
  let album = $state("");
  let loading = $state(false);
  let results = $state([]);
  let searchQuery = $state("");
  let lastOpenState = $state(false);

  // Initialize only when opening to allow manual editing
  $effect(() => {
    if (isOpen && !lastOpenState) {
      artist = initialArtist;
      album = initialAlbum;
      search();
    }
    lastOpenState = isOpen;
  });

  async function search() {
    if (!artist || !album) return;
    loading = true;
    results = [];
    try {
      const res = await fetch(
        `/api/metadata/search-album?artist=${encodeURIComponent(artist)}&album=${encodeURIComponent(album)}`,
      );
      if (res.ok) {
        results = await res.json();
      }
    } catch (e) {
      showToast("Search failed", "error");
    } finally {
      loading = false;
    }
  }

  async function applyCover(releaseId) {
    loading = true;
    const coverUrl = `https://coverartarchive.org/release/${releaseId}/front-500`;

    try {
      // 1. Get all tracks for this album to apply the cover to all of them
      const tracksRes = await fetch(`/api/albums/${albumId}/tracks`);
      if (!tracksRes.ok) throw new Error("Failed to get album tracks");
      const tracks = await tracksRes.json();

      showToast(`Applying cover to ${tracks.length} tracks...`, "info");

      const promises = tracks.map((t) =>
        fetch(`/api/tracks/${t.id}/cover-url`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ url: coverUrl }),
        }),
      );

      await Promise.all(promises);

      // 2. Clear album metadata in DB to force rescan of dimensions
      await fetch(`/api/library/rescan-cover?id=${albumId}`, {
        method: "POST",
      });

      showToast("HD Cover applied successfully", "success");
      coverRefreshTrigger.set(Date.now());
      onapply?.();
      isOpen = false;
    } catch (e) {
      showToast("Failed to apply cover", "error");
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e) {
    if (e.key === "Escape") isOpen = false;
    if (e.key === "Enter") search();
  }
</script>

{#if isOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="modal-overlay"
    transition:fade
    onclick={() => (isOpen = false)}
    role="presentation"
  >
    <div
      class="cover-searcher"
      transition:scale={{ start: 0.95 }}
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <header class="search-header">
        <div class="title-row">
          <div class="icon-circle">
            <Icon name="image" size={24} color="var(--primary)" />
          </div>
          <div class="texts">
            <h2>Find Album Artwork</h2>
            <p>{initialArtist} — {initialAlbum}</p>
          </div>
          <button class="close-btn" onclick={() => (isOpen = false)}>
            <Icon name="x" size={20} />
          </button>
        </div>

        <div class="search-controls">
          <div class="input-group">
            <Icon name="user" size={16} />
            <input
              type="text"
              bind:value={artist}
              placeholder="Artist..."
              onkeydown={handleKeydown}
            />
          </div>
          <div class="input-group">
            <Icon name="disc" size={16} />
            <input
              type="text"
              bind:value={album}
              placeholder="Album..."
              onkeydown={handleKeydown}
            />
          </div>
          <button class="search-btn" onclick={search} disabled={loading}>
            {#if loading}
              <div class="spinner-mini"></div>
            {:else}
              <Icon name="search" size={20} />
              <span>Search</span>
            {/if}
          </button>
        </div>
      </header>

      <div class="results-container">
        {#if loading && results.length === 0}
          <div class="status-box">
            <div class="spinner"></div>
            <p>Consulting Global Databases...</p>
          </div>
        {:else if !loading && results.length === 0}
          <div class="status-box empty">
            <Icon name="search" size={48} />
            <p>No high-quality covers found for this query.</p>
            <span>Try adjusting the artist or album name above.</span>
          </div>
        {:else}
          <div class="cover-grid">
            {#each results as release}
              <button
                class="cover-card"
                onclick={() => applyCover(release.id)}
                disabled={loading}
              >
                <div class="img-wrapper">
                  <img
                    src="https://coverartarchive.org/release/{release.id}/front-250"
                    alt={release.title}
                    loading="lazy"
                  />
                  <div class="overlay">
                    <Icon name="check" size={32} color="white" />
                    <span>Apply This</span>
                  </div>
                </div>
                <div class="info">
                  <span class="rel-title" title={release.title}
                    >{release.title}</span
                  >
                  <span class="rel-meta"
                    >{release.date?.split("-")[0] || "N/A"} • {release[
                      "track-count"
                    ]} tracks</span
                  >
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.85);
    backdrop-filter: blur(12px);
    z-index: 9999;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2rem;
  }

  .cover-searcher {
    background: var(--surface-container);
    width: 100%;
    max-width: 900px;
    max-height: 85vh;
    border-radius: 40px;
    border: 1px solid var(--border-color);
    box-shadow: 0 30px 60px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .search-header {
    padding: 2.5rem;
    background: var(--surface-container-high);
    border-bottom: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }
  .icon-circle {
    width: 54px;
    height: 54px;
    background: var(--primary-10);
    border-radius: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .texts {
    flex: 1;
  }
  .texts h2 {
    margin: 0;
    font-size: 1.6rem;
    font-weight: 900;
    letter-spacing: -0.02em;
  }
  .texts p {
    margin: 0;
    color: var(--text-muted);
    font-weight: 600;
    opacity: 0.8;
  }

  .close-btn {
    border: none;
    background: var(--surface-container-highest);
    color: var(--text-muted);
    width: 44px;
    height: 44px;
    border-radius: 14px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .close-btn:hover {
    background: #ff444420;
    color: #ff4444;
  }

  .search-controls {
    display: flex;
    gap: 1rem;
    background: var(--surface-container-highest);
    padding: 0.6rem;
    border-radius: 24px;
    border: 1px solid rgba(255, 255, 255, 0.05);
  }

  .input-group {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    padding: 0 1.2rem;
    background: rgba(0, 0, 0, 0.2);
    border-radius: 16px;
    border: 1px solid transparent;
    transition: all 0.2s;
  }
  .input-group:focus-within {
    border-color: var(--primary-40);
    background: rgba(0, 0, 0, 0.3);
  }

  .input-group input {
    background: none;
    border: none;
    color: var(--text-main);
    padding: 0.8rem 0;
    width: 100%;
    font-weight: 600;
    outline: none;
  }

  .search-btn {
    background: var(--primary);
    color: white;
    border: none;
    padding: 0 2rem;
    border-radius: 16px;
    font-weight: 800;
    display: flex;
    align-items: center;
    gap: 0.8rem;
    cursor: pointer;
    transition: all 0.2s;
  }
  .search-btn:hover {
    filter: brightness(1.1);
    transform: scale(1.02);
  }
  .search-btn:disabled {
    opacity: 0.5;
    cursor: wait;
  }

  .results-container {
    flex: 1;
    overflow-y: auto;
    padding: 2.5rem;
    min-height: 400px;
  }

  .status-box {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    color: var(--text-muted);
    font-weight: 600;
  }
  .status-box.empty {
    opacity: 0.5;
  }

  .cover-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 2rem;
  }

  .cover-card {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    text-align: left;
    transition: transform 0.3s cubic-bezier(0.2, 0, 0, 1);
  }
  .cover-card:hover {
    transform: translateY(-8px);
  }

  .img-wrapper {
    position: relative;
    aspect-ratio: 1;
    border-radius: 20px;
    overflow: hidden;
    background: var(--surface-container-highest);
    box-shadow: 0 10px 20px rgba(0, 0, 0, 0.2);
  }

  .img-wrapper img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    transition: transform 0.6s;
  }
  .cover-card:hover img {
    transform: scale(1.1);
  }

  .overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(var(--primary-rgb), 0.8);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.8rem;
    opacity: 0;
    transition: all 0.3s;
    backdrop-filter: blur(4px);
  }
  .cover-card:hover .overlay {
    opacity: 1;
  }
  .overlay span {
    color: white;
    font-weight: 900;
    text-transform: uppercase;
    font-size: 0.8rem;
    letter-spacing: 1px;
  }

  .info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .rel-title {
    color: var(--text-main);
    font-weight: 800;
    font-size: 0.95rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .rel-meta {
    color: var(--text-muted);
    font-size: 0.8rem;
    font-weight: 600;
    opacity: 0.7;
  }

  .spinner {
    width: 40px;
    height: 40px;
    border: 4px solid var(--primary-20);
    border-top-color: var(--primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  .spinner-mini {
    width: 20px;
    height: 20px;
    border: 3px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 600px) {
    .search-controls {
      flex-direction: column;
    }
    .cover-grid {
      grid-template-columns: repeat(2, 1fr);
      gap: 1rem;
    }
  }
</style>
