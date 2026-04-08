<script>
  import Icon from './Icon.svelte';
  import StarRating from './StarRating.svelte';
  import { showToast } from './stores.js';
  import { fade } from 'svelte/transition';

  let { 
    currentPlaying, 
    oncloseFull 
  } = $props();

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
</script>

<div class="anchor-bar" in:fade>
  <div class="left-section">
    {#if currentPlaying}
      <div class="track-mini-info">
        <span class="title">{currentPlaying.title}</span>
        <span class="artist">{currentPlaying.artist}</span>
      </div>
    {/if}
  </div>

  <div class="center-section">
    {#if currentPlaying}
      <StarRating rating={currentPlaying.rating || 0} size={28} onchange={handleRate} />
    {/if}
  </div>

  <div class="right-section">
    <button class="collapse-btn" onclick={oncloseFull} title="Collapse Player">
      <Icon name="chevronDown" size={32} />
    </button>
  </div>
</div>

<style>
  .anchor-bar {
    grid-area: player;
    background: var(--surface-container);
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    padding: 0 2.5rem;
    height: 110px;
    margin: 0;
    border-radius: inherit;
    border: 1px solid var(--border-color);
    border-top: 1px solid rgba(255,255,255,0.05); /* very subtle top separator if needed, or none */
    box-shadow: 0 12px 40px rgba(0,0,0,0.2);
    z-index: 1000;
  }

  .left-section { display: flex; align-items: center; min-width: 0; }
  .track-mini-info { display: flex; flex-direction: column; gap: 2px; overflow: hidden; }
  .title { font-weight: 850; font-size: 1rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .artist { font-size: 0.85rem; opacity: 0.6; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .center-section { display: flex; justify-content: center; }
  .right-section { display: flex; justify-content: flex-end; }

  .collapse-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 64px;
    height: 64px;
    border-radius: 50%;
  }
  .collapse-btn:hover { 
    color: var(--text-main); 
    background: var(--surface-container-high); 
    transform: scale(1.1); 
  }
</style>
