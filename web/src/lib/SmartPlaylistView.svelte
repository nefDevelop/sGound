<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';
  import TracksView from './TracksView.svelte';

  let { type = "", name = "", icon = "list", currentPlaying = null, onback, onplay, onedit, onselectionChange, onhide, ondeletePhysical, onshuffle } = $props();

  let tracks = $state([]);
  let loading = $state(true);
  let filterValue = $state("");
  let options = $state([]);

  $effect(() => {
    if (type) {
      if (type === 'decade' || type === 'genre') {
          fetchOptions();
      } else {
          fetchTracks();
      }
    }
  });

  async function fetchOptions() {
      try {
          const res = await fetch('/api/library/metadata');
          if (!res.ok) throw new Error("API error");
          const data = await res.json();
          
          if (type === 'decade') {
              options = data.decades || [];
          } else if (type === 'genre') {
              options = data.genres || [];
          }

          if (!filterValue && options.length > 0) {
              filterValue = options[0];
          }
          fetchTracks();
      } catch (e) {
          console.error("Metadata fetch error", e);
      }
  }

  async function fetchTracks() {
    loading = true;
    try {
      let url = `/api/smart-playlists/${type}`;
      if (filterValue) url += `?value=${encodeURIComponent(filterValue)}`;
      
      const res = await fetch(url);
      if (!res.ok) {
          tracks = [];
      } else {
          tracks = await res.json() || [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  function handleFilterChange(e) {
      filterValue = e.target.value;
      fetchTracks();
  }
</script>

<div class="smart-view" in:fade>
  <header class="view-header">
    <div class="top-nav">
      <button class="back-btn" onclick={onback}>
        <Icon name="arrowLeft" size={24} />
      </button>

      {#if type === 'decade' || type === 'genre'}
        <div class="filter-box">
          <span>Filter by {type === 'decade' ? 'Decade' : 'Genre'}</span>
          <select bind:value={filterValue} onchange={handleFilterChange} class="modern-select">
            {#each options as opt}
              <option value={opt}>{type === 'decade' ? opt + 's' : opt}</option>
            {/each}
          </select>
        </div>
      {/if}
    </div>
    
    <div class="header-content">
      <div class="icon-box">
        <Icon name={icon} size={64} />
      </div>
      <div class="texts">
        <span class="type-label">Smart Selection</span>
        <h1>{name}</h1>
        <p class="desc">Automatically generated from your library.</p>
      </div>
    </div>
  </header>

  <div class="content">
    {#if loading}
      <div class="status">Compiling your list...</div>
    {:else}
      <TracksView 
        {tracks} 
        {currentPlaying} 
        onselectionChange={onselectionChange}
        onplay={onplay}
        onedit={onedit}
        onqueue={(track) => {}}
        onhide={onhide}
        ondeletePhysical={ondeletePhysical}
        onshuffle={onshuffle}
      />
    {/if}
  </div>
</div>

<style>
  .smart-view { width: 100%; animation: slideIn 0.3s ease-out; }
  
  .view-header {
    display: flex;
    flex-direction: column;
    gap: 3rem;
    margin-bottom: 4rem;
  }

  .top-nav { display: flex; justify-content: space-between; align-items: center; }

  .back-btn {
    width: 48px; height: 48px; border-radius: 50%;
    background: var(--surface-container-high);
    display: flex; align-items: center; justify-content: center;
    color: var(--text-muted); cursor: pointer; border: 1px solid var(--border-color);
    transition: all 0.2s;
  }
  .back-btn:hover { color: var(--primary); background: var(--surface-container-highest); transform: translateX(-4px); }

  .header-content {
    display: flex;
    align-items: center;
    gap: 3rem;
  }

  .icon-box {
    width: 140px; height: 140px;
    background: var(--surface-container-high);
    border-radius: 32px;
    display: flex; align-items: center; justify-content: center;
    box-shadow: 0 20px 40px rgba(0,0,0,0.3);
    border: 1px solid var(--border-color);
    color: var(--primary);
  }

  .type-label {
    text-transform: uppercase;
    font-size: 0.8rem;
    font-weight: 900;
    letter-spacing: 2px;
    color: var(--primary);
  }

  h1 { font-size: 3.5rem; margin: 0.5rem 0; font-weight: 900; letter-spacing: -2px; color: var(--text-main); }
  .desc { color: var(--text-muted); font-size: 1.1rem; font-weight: 600; }

  .filter-box {
    display: flex; align-items: center; gap: 1rem;
    padding: 0.6rem 1.2rem; background: var(--surface-container);
    border-radius: 100px; border: 1px solid var(--border-color);
  }
  .filter-box span { font-weight: 800; font-size: 0.9rem; color: var(--text-muted); }

  .modern-select {
    background: transparent; border: none; color: var(--primary);
    font-weight: 800; font-size: 1rem; outline: none; cursor: pointer;
  }

  .status { text-align: center; padding: 5rem; color: var(--text-muted); font-weight: 700; }

  @media (max-width: 768px) {
    .header-content { 
      flex-direction: row; 
      text-align: left; 
      gap: 1.5rem; 
      align-items: center; 
    }
    .icon-box { 
      width: 80px; 
      height: 80px; 
      border-radius: 20px; 
      flex-shrink: 0;
    }
    .icon-box :global(svg) { width: 32px; height: 32px; }
    h1 { font-size: 1.8rem; letter-spacing: -1px; margin: 0; }
    .desc { font-size: 0.85rem; }
    .type-label { font-size: 0.65rem; }
    .view-header { gap: 1.5rem; margin-bottom: 2rem; }
    .top-nav { flex-direction: row; justify-content: space-between; align-items: center; }
    .filter-box { padding: 0.4rem 0.8rem; }
    .filter-box span { display: none; }
  }

  @keyframes slideIn {
    from { transform: translateX(20px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
</style>
