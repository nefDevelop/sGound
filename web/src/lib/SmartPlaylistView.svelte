<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade } from 'svelte/transition';
  import TracksView from './TracksView.svelte';

  export let type = "";
  export let name = "";
  export let icon = "list";
  export let currentPlaying = null;

  const dispatch = createEventDispatcher();
  let tracks = [];
  let loading = true;
  let filterValue = "";
  
  let options = []; // Para décadas o géneros

  $: if (type) {
    if (type === 'decade' || type === 'genre') {
        fetchOptions();
    } else {
        fetchTracks();
    }
  }

  async function fetchOptions() {
      try {
          const res = await fetch('/api/library/metadata');
          if (!res.ok) throw new Error("API responded with " + res.status);
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
          console.error("Smart playlist failed", res.status);
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
    <button class="back-btn" on:click={() => dispatch('back')}>
      <Icon name="arrowLeft" size={24} />
    </button>
    
    <div class="header-content">
      <div class="icon-box" class:quality={type === 'high-quality'} class:decades={type === 'decade'}>
        <Icon name={icon} size={48} />
      </div>
      <div class="texts">
        <h1>{name}</h1>
        <p class="desc">Lista generada automáticamente basada en tu biblioteca.</p>
      </div>
    </div>

    {#if type === 'decade' || type === 'genre'}
      <div class="filter-box">
        <span>Filtrar por {type === 'decade' ? 'Década' : 'Género'}:</span>
        <select bind:value={filterValue} on:change={handleFilterChange} class="glass-select">
          {#each options as opt}
            <option value={opt}>{type === 'decade' ? opt + 's' : opt}</option>
          {/each}
        </select>
      </div>
    {/if}
  </header>

  <div class="content">
    {#if loading}
      <div class="loading-state">Calculando lista...</div>
    {:else}
      <TracksView 
        {tracks} 
        {currentPlaying} 
        on:play 
        on:edit 
        on:selectionChange 
      />
    {/if}
  </div>
</div>

<style>
  .smart-view { animation: slideIn 0.3s ease; }
  
  .view-header {
    display: flex;
    flex-direction: column;
    gap: 2rem;
    margin-bottom: 3rem;
  }

  .back-btn {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    background: rgba(255,255,255,0.05);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
  }
  .back-btn:hover { color: white; background: rgba(255,255,255,0.1); }

  .header-content {
    display: flex;
    align-items: center;
    gap: 2rem;
  }

  .icon-box {
    width: 120px;
    height: 120px;
    background: var(--primary-10);
    color: var(--primary);
    border-radius: var(--radius);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 20px 40px rgba(0,0,0,0.2);
  }

  .icon-box.quality { background: rgba(214, 184, 92, 0.1); color: var(--tertiary); }
  .icon-box.decades { background: rgba(194, 112, 133, 0.1); color: var(--secondary); }

  h1 { 
    font-family: var(--font-headline);
    font-size: 3.5rem;
    margin: 0;
    letter-spacing: -2px;
  }

  .desc { color: var(--text-muted); font-size: 1.1rem; margin-top: 0.5rem; }

  .filter-box {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 1rem 1.5rem;
    background: rgba(255,255,255,0.03);
    border-radius: 100px;
    align-self: flex-start;
  }

  .glass-select {
    background: #1e293b;
    border: 1px solid rgba(255,255,255,0.1);
    color: white;
    padding: 0.4rem 1rem;
    border-radius: 8px;
    font-weight: 700;
    outline: none;
  }

  .loading-state { text-align: center; padding: 5rem; color: var(--text-muted); font-style: italic; }

  @keyframes slideIn {
    from { transform: translateX(20px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
</style>
