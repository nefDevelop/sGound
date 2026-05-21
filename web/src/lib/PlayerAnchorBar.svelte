<script>
  import Icon from './Icon.svelte';
  import StarRating from './StarRating.svelte';
  import { showToast, audioQuality } from './stores.js';
  import { fade } from 'svelte/transition';

  let { 
    currentPlaying, 
    view = $bindable('cover'),
    oncloseFull 
  } = $props();

  let showMenu = $state(false);

  async function handleRate(rating) {
    if (!currentPlaying) return;
    try {
      const res = await fetch(`/api/tracks/${currentPlaying.id}/rate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rating }),
      });
      if (res.ok) {
        currentPlaying.rating = rating;
        showToast(`Valorado con ${rating} estrellas`, "success");
      }
    } catch (e) {
      console.error(e);
      showToast("Error al valorar", "error");
    }
  }

  function handleOverlayClick(e) {
    if (e.target.closest('button') || e.target.closest('.star-rating') || e.target.closest('.view-menu')) return;
    oncloseFull?.();
  }

  function selectView(v) {
    view = v;
    showMenu = false;
  }
</script>

<div class="anchor-bar" in:fade onclick={handleOverlayClick} role="button" tabindex="0" onkeydown={(e) => e.key === 'Escape' && oncloseFull?.()}>
  <div class="left-section">
    <div class="view-selector-wrap">
      {#if showMenu}
        <div class="view-menu" in:fade={{duration: 150}}>
          <button class="menu-item" class:active={view === 'cover'} onclick={() => selectView('cover')}>
            <Icon name="image" size={18} /> Portada
          </button>
          <button class="menu-item" class:active={view === 'lyrics'} onclick={() => selectView('lyrics')}>
            <Icon name="music" size={18} /> Letras
          </button>
          <button class="menu-item" class:active={view === 'queue'} onclick={() => selectView('queue')}>
            <Icon name="list" size={18} /> Cola
          </button>
          <button class="menu-item" class:active={view === 'cinema'} onclick={() => selectView('cinema')}>
            <Icon name="maximize" size={18} /> Cine
          </button>
        </div>
      {/if}
      <button class="view-toggle-btn" onclick={() => showMenu = !showMenu} title="Cambiar vista">
        <Icon name="layers" size={24} />
      </button>
    </div>
  </div>

  <div class="center-section">
    {#if currentPlaying}
      <div class="stars-wrapper">
        <StarRating 
          rating={currentPlaying.rating || 0} 
          size={window.innerWidth < 400 ? 22 : 28} 
          onchange={handleRate} 
        />
      </div>
    {/if}
  </div>

  <div class="right-section">
    {#if currentPlaying}
      <div class="quality-badge" class:transcoded={$audioQuality !== 'original'}>
        {#if $audioQuality === 'original'}
          {currentPlaying.format?.toUpperCase()} {currentPlaying.bit_rate ? Math.round(currentPlaying.bit_rate / 1000) : '--'}k
        {:else}
          TRANS {$audioQuality.toUpperCase()}
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .anchor-bar {
    background: transparent;
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    padding: 0 1rem;
    height: 100px;
    cursor: pointer;
    z-index: 1000;
  }

  .left-section { display: flex; align-items: center; min-width: 80px; }
  .center-section { display: flex; justify-content: center; align-items: center; flex: 1; }
  .right-section { display: flex; justify-content: flex-end; align-items: center; min-width: 80px; }

  .stars-wrapper {
    display: flex;
    justify-content: center;
    width: 100%;
  }

  .quality-badge {
    background: rgba(255,255,255,0.15);
    padding: 0.3rem 0.6rem;
    border-radius: 8px;
    font-size: 0.68rem;
    font-weight: 950;
    color: white;
    letter-spacing: 0.5px;
    border: 1px solid rgba(255,255,255,0.1);
    white-space: nowrap;
  }

  .quality-badge.transcoded {
    background: var(--primary-10);
    color: var(--primary);
    border-color: var(--primary-20);
  }
  .right-section { display: flex; justify-content: flex-end; }

  .view-selector-wrap { position: relative; }
  
  .view-toggle-btn {
    width: 54px; height: 54px;
    border-radius: 50%;
    background: rgba(255,255,255,0.06);
    color: var(--text-muted);
    border: 1px solid rgba(255,255,255,0.04);
  }
  .view-toggle-btn:hover { background: rgba(255,255,255,0.12); color: white; }

  .view-menu {
    position: absolute;
    bottom: calc(100% + 1rem);
    left: 0;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    border-radius: 24px;
    padding: 0.6rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    min-width: 160px;
    box-shadow: 0 20px 50px rgba(0,0,0,0.5);
    z-index: 2000;
    pointer-events: auto !important;
  }

  .menu-item {
    justify-content: flex-start;
    gap: 0.8rem;
    padding: 1rem 1.2rem;
    border-radius: 16px;
    font-weight: 700;
    font-size: 0.95rem;
    color: var(--text-muted);
    width: 100%;
    pointer-events: auto !important;
    user-select: none;
  }
  .menu-item:hover { background: rgba(255,255,255,0.08); color: white; }
  .menu-item:active { background: rgba(255,255,255,0.15); }
  .menu-item.active { background: var(--primary); color: white; }

  @media (max-width: 770px) {
    .anchor-bar { 
      padding: 0 1rem; 
      height: 80px; 
      padding-bottom: 8px; 
    }
    .view-toggle-btn { width: 48px; height: 48px; }
  }

  @media (max-width: 400px) {
    .anchor-bar { padding: 0 0.5rem; height: 72px; padding-bottom: 4px; }
    .view-toggle-btn { width: 42px; height: 42px; }
    .view-toggle-btn :global(svg) { width: 20px; height: 20px; }
    .quality-badge { font-size: 0.6rem; padding: 0.2rem 0.4rem; }
  }
</style>
