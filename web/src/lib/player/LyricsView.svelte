<script>
  import { tick } from 'svelte';
  import { fade } from 'svelte/transition';
  import Icon from '../Icon.svelte';

  let {
    audioRef,
    currentTime,
    parsedLyrics = [],
    lyrics = "Loading...",
    currentLyricIndex = -1,
    onUserInteraction
  } = $props();

  let lyricContainer = $state(null);
  let userScrolling = $state(false);
  let scrollTimeout;

  function handleScroll(e) {
      if (e.currentTarget) {
          const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
          // optional logging
      }
      userScrolling = true;
      clearTimeout(scrollTimeout);
      scrollTimeout = setTimeout(() => {
          userScrolling = false;
      }, 3000);
      
      if(onUserInteraction) onUserInteraction(e);
  }

  // Reactively scroll to active lyric
  $effect(() => {
      if (currentLyricIndex !== -1 && lyricContainer && !userScrolling) {
          tick().then(() => {
              const activeLine = lyricContainer.querySelector('.active');
              if (activeLine) activeLine.scrollIntoView({ behavior: 'smooth', block: 'center' });
          });
      }
  });
</script>

<div class="view-panel lyrics-panel" in:fade>
  {#if parsedLyrics.length > 0}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div 
      class="lyrics-list" 
      bind:this={lyricContainer}
      onwheel={handleScroll}
      onmousedown={handleScroll}
      ontouchstart={handleScroll}
      role="region"
      aria-label="Letras de la canción"
    >
      {#each parsedLyrics as line, i}
        <button 
          class="lyric-line" 
          class:active={i === currentLyricIndex}
          onclick={(e) => { e.stopPropagation(); if(audioRef) audioRef.currentTime = line.time; }}
          in:fade={{ delay: Math.min(i * 20, 800), duration: 500 }}
        >
          {line.text}
        </button>
      {/each}
    </div>
  {:else if lyrics === "Loading..."}
      <div class="no-lyrics loading-state">
        <div class="loading-spinner"></div>
        <p>Buscando letra...</p>
      </div>
  {:else}
    <div class="no-lyrics">
        <Icon name="music" size={64} />
        <p>Instrumental</p>
    </div>
  {/if}
</div>

<style>
  .view-panel { 
    width: 100%; 
    display: flex; 
    flex-direction: column; 
    gap: 1.5rem; 
    animation: fadeInQuick 0.2s cubic-bezier(0,0,0.2,1); 
    flex: 1;
    min-height: 0;
  }

  .lyrics-panel {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .lyrics-list {
    display: block;
    flex: 1;
    width: 100%;
    overflow-y: auto;
    scrollbar-width: none;
    position: relative;
    padding: 50vh 2rem;
    padding-top: 100px;
    pointer-events: auto !important;
    user-select: none;
    touch-action: pan-y;
    -webkit-overflow-scrolling: touch;
    z-index: 10;
    min-height: 0;
  }
  .lyrics-list::-webkit-scrollbar { display: none; }

  .lyric-line {
    font-size: 2.2rem;
    font-weight: 900;
    color: rgba(255,255,255,0.15);
    background: none;
    border: none;
    cursor: pointer;
    text-align: center;
    transition: all 0.5s cubic-bezier(0.16, 1, 0.3, 1);
    line-height: 1.4;
    letter-spacing: -1px;
    padding: 1rem 0;
    width: 100%;
    margin-bottom: 1.5rem;
    display: block;
  }
  .lyric-line.active {
    color: white;
    transform: scale(1.05);
    filter: drop-shadow(0 0 20px rgba(255,255,255,0.3));
  }
  .lyric-line:hover {
    color: rgba(255,255,255,0.6);
  }

  .no-lyrics {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    gap: 1.5rem;
    opacity: 0.15;
    min-height: 50vh;
  }
  .no-lyrics p { font-size: 1.2rem; font-weight: 800; letter-spacing: 1px; text-transform: uppercase; }

  .loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255,255,255,0.1);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
  
  @keyframes fadeInQuick { 
    from { opacity: 0; transform: translateY(4px); } 
    to { opacity: 1; transform: translateY(0); } 
  }
</style>