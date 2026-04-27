<script>
  import { queue, currentIndex, isPlaying } from './stores.js';
  import Icon from './Icon.svelte';
  import Artwork from './Artwork.svelte';
  import { fly } from 'svelte/transition';

  let { isOpen = $bindable(false), onclose = undefined } = $props();

  function playAt(index) {
    currentIndex.set(index);
    isPlaying.set(true);
  }

  function clearQueue() {
    queue.set([]);
    currentIndex.set(-1);
    isPlaying.set(false);
  }

  function removeFromQueue(index) {
    const q = $queue;
    q.splice(index, 1);
    queue.set([...q]);
    if ($currentIndex >= index && $currentIndex > 0) {
      currentIndex.set($currentIndex - 1);
    }
  }
</script>

{#if isOpen}
<div 
  class="queue-backdrop" 
  onclick={() => isOpen = false}
  onkeydown={(e) => e.key === 'Escape' && (isOpen = false)}
  role="presentation"
></div>
<aside class="queue-panel" transition:fly={{ x: 400, duration: 300 }}>
  <header class="queue-header">
    <div class="header-main">
      <h2>Cola de reproducción</h2>
      <span class="count">{$queue.length} canciones</span>
    </div>
    <div class="header-actions">
      <button class="clear-btn" onclick={clearQueue} title="Vaciar cola">
        <Icon name="trash" size={18} />
      </button>
      <button class="close-btn" onclick={() => isOpen = false}>
        <Icon name="x" size={20} />
      </button>
    </div>
  </header>

  <div class="queue-list" role="listbox">
    {#each $queue as track, i}
      <div 
        class="queue-item" 
        class:active={$currentIndex === i}
        onclick={() => playAt(i)}
        onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && playAt(i)}
        role="option"
        aria-selected={$currentIndex === i}
        tabindex="0"
      >
        <div class="item-index">{i + 1}</div>
        <Artwork src={`/api/cover/${track.id}`} size="40px" borderRadius="8px" />
        <div class="item-info">
          <span class="title">{track.title}</span>
          <span class="artist">{track.artist}</span>
        </div>
        {#if $currentIndex === i && $isPlaying}
          <div class="playing-indicator">
            <div class="bar"></div>
            <div class="bar"></div>
            <div class="bar"></div>
          </div>
        {/if}
        <button class="remove-btn" onclick={(e) => { e.stopPropagation(); removeFromQueue(i); }}>
          <Icon name="minusCircle" size={16} />
        </button>
      </div>
    {:else}
      <div class="empty-queue">
        <Icon name="music" size={48} />
        <p>Tu cola está vacía</p>
      </div>
    {/each}
  </div>
</aside>
{/if}

<style>
  .queue-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0,0,0,0.4);
    backdrop-filter: blur(4px);
    z-index: 200;
  }

  .queue-panel {
    position: fixed;
    top: 1rem;
    right: 1rem;
    bottom: 1rem;
    width: 380px;
    background: var(--surface-container-high);
    border: 1px solid var(--border-color);
    border-radius: 24px;
    box-shadow: -10px 0 40px rgba(0,0,0,0.4);
    z-index: 201;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .queue-header {
    padding: 1.5rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border-color);
  }

  .header-main h2 { margin: 0; font-size: 1.2rem; font-weight: 800; }
  .header-main .count { font-size: 0.8rem; color: var(--text-muted); font-weight: 600; }

  .header-actions { display: flex; gap: 0.5rem; }
  
  .clear-btn, .close-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    padding: 8px;
    border-radius: 12px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .clear-btn:hover { background: rgba(248, 113, 113, 0.1); color: #f87171; }
  .close-btn:hover { background: var(--surface-container-highest); color: var(--text-main); }

  .queue-list {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .queue-item {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.6rem 0.8rem;
    border-radius: 16px;
    cursor: pointer;
    transition: all 0.2s;
    position: relative;
    group: hover;
  }

  .queue-item:hover { background: var(--surface-container-highest); }
  .queue-item.active { background: var(--primary-10); }
  .queue-item.active .title { color: var(--primary); font-weight: 800; }

  .item-index { width: 20px; font-size: 0.75rem; color: var(--text-muted); font-weight: 700; text-align: right; }
  
  .item-info { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
  .item-info .title { font-size: 0.9rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .item-info .artist { font-size: 0.75rem; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .remove-btn {
    opacity: 0;
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.2s;
  }
  .queue-item:hover .remove-btn { opacity: 1; }
  .remove-btn:hover { color: #f87171; }

  .empty-queue {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    height: 100%;
    color: var(--text-muted);
    opacity: 0.5;
  }

  .playing-indicator {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    height: 12px;
    margin-right: 0.5rem;
  }
  .playing-indicator .bar {
    width: 3px;
    background: var(--primary);
    border-radius: 1px;
    animation: bounce 0.6s ease-in-out infinite alternate;
  }
  .playing-indicator .bar:nth-child(2) { animation-delay: 0.2s; height: 100%; }
  .playing-indicator .bar:nth-child(3) { animation-delay: 0.4s; height: 60%; }

  @keyframes bounce {
    from { height: 20%; }
    to { height: 100%; }
  }

  @media (max-width: 480px) {
    .queue-panel {
      top: 0; right: 0; bottom: 0; left: 0;
      width: 100%;
      border-radius: 0;
    }
  }
</style>
