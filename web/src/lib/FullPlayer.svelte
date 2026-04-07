<script>
  import { onMount, tick } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade, fly } from 'svelte/transition';
  import { audioQuality } from './stores.js';

  export let currentPlaying = null;
  export let audioRef = null;
  export let isOpen = false;
  export let progress = 0;
  export let duration = 0;
  export let currentTime = 0;

  let lyrics = "";
  let parsedLyrics = [];
  let currentLyricIndex = -1;
  let lyricContainer;
  let showLyrics = false; // Toggle for mobile
  let volume = 1;

  // Sincronizar letras con el tiempo actual
  $: if (parsedLyrics.length > 0) {
    const index = parsedLyrics.findIndex((l, i) => {
      const next = parsedLyrics[i + 1];
      return currentTime >= l.time && (!next || currentTime < next.time);
    });
    if (index !== currentLyricIndex && index !== -1) {
      currentLyricIndex = index;
      tick().then(scrollToActive);
    }
  }

  // Cargar letras cuando cambia la canción
  $: if (currentPlaying && isOpen) {
    loadLyrics();
    if (audioRef) volume = audioRef.volume;
  }

  async function loadLyrics() {
    try {
      const res = await fetch(`/api/lyrics/${currentPlaying.id}`);
      const data = await res.json();
      lyrics = data.lyrics || "No lyrics available.";
      parseLRC(lyrics);
    } catch (e) {
      lyrics = "Failed to load lyrics.";
      parsedLyrics = [];
    }
  }

  function parseLRC(lrc) {
    const lines = lrc.split('\n');
    const result = [];
    const timeReg = /\[(\d+):(\d+\.?\d*)\]/;

    lines.forEach(line => {
      const match = timeReg.exec(line);
      if (match) {
        const minutes = parseInt(match[1]);
        const seconds = parseFloat(match[2]);
        const time = minutes * 60 + seconds;
        const text = line.replace(timeReg, '').trim();
        if (text) result.push({ time, text });
      }
    });

    if (result.length === 0 && lrc.trim()) {
        parsedLyrics = lrc.split('\n').filter(l => l.trim()).map(text => ({ time: 0, text }));
    } else {
        parsedLyrics = result.sort((a, b) => a.time - b.time);
    }
  }

  function scrollToActive() {
    if (!lyricContainer) return;
    const activeLine = lyricContainer.querySelector('.active');
    if (activeLine) {
      const top = activeLine.offsetTop - lyricContainer.clientHeight / 2 + activeLine.clientHeight / 2;
      lyricContainer.scrollTo({ top, behavior: 'smooth' });
    }
  }

  function formatTime(secs) {
    if (isNaN(secs)) return "0:00";
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${mins}:${s.toString().padStart(2, '0')}`;
  }

  function handleSeek(e) {
    const seekTime = (e.target.value / 100) * duration;
    audioRef.currentTime = seekTime;
  }

  function handleVolume(e) {
    volume = e.target.value;
    audioRef.volume = volume;
  }
</script>

{#if isOpen && currentPlaying}
<div class="full-player" in:fly={{ y: '100%', duration: 400 }} out:fly={{ y: '100%', duration: 300 }}>
  <!-- Background blur -->
  <div class="bg-blur" style="background-image: url(/api/cover/${currentPlaying.id})"></div>
  <div class="vignette"></div>
  
  <div class="main-content">
    <header class="player-header">
      <button class="icon-btn" on:click={() => isOpen = false} title="Close">
        <Icon name="chevronDown" size={28} />
      </button>
      <div class="header-info">
        <span class="playing-from">En reproduccion</span>
        <span class="album-name">{currentPlaying.album}</span>
      </div>
      <button class="icon-btn mobile-only" on:click={() => showLyrics = !showLyrics}>
        <Icon name={showLyrics ? 'music' : 'list'} size={24} />
      </button>
      <div class="desktop-only" style="width: 28px"></div>
    </header>

    <div class="player-body" class:show-lyrics={showLyrics}>
      <div class="left-side">
        <div class="cover-wrapper">
          <img src="/api/cover/{currentPlaying.id}" alt="Cover" />
        </div>
        
        <div class="track-meta">
          <div class="meta-texts">
            <div class="title-row">
              <h1>{currentPlaying.title}</h1>
              <div class="quality-badge" class:transcoded={$audioQuality !== 'original'}>
                {#if $audioQuality === 'original'}
                  {currentPlaying.format?.toUpperCase()} {Math.round(currentPlaying.bit_rate / 1000)}k
                {:else}
                  TRANSCODED {$audioQuality.toUpperCase()}
                {/if}
              </div>
            </div>
            <p class="artist-name">{currentPlaying?.artist || 'Unknown Artist'}</p>
          </div>
          <button class="icon-btn favorite-btn" class:active={false}>
            <Icon name="heart" size={28} />
          </button>
        </div>

        <div class="playback-controls">
          <div class="timeline">
            <span class="time">{formatTime(currentTime)}</span>
            <div class="progress-container">
               <input type="range" class="full-progress" min="0" max="100" bind:value={progress} on:input={handleSeek} />
               <div class="progress-fill" style="width: {progress}%"></div>
            </div>
            <span class="time">{formatTime(duration)}</span>
          </div>

          <div class="main-btns">
            <button class="sub-btn" class:active={false}><Icon name="shuffle" size={20} /></button>
            <button class="main-btn"><Icon name="skipBack" size={32} /></button>
            <button class="play-btn" on:click={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()}>
              <Icon name={audioRef?.paused ? 'play' : 'pause'} size={32} color="black" />
            </button>
            <button class="main-btn"><Icon name="skipForward" size={32} /></button>
            <button class="sub-btn" class:active={false}><Icon name="repeat" size={20} /></button>
          </div>

          <div class="extra-controls">
            <div class="volume-control">
               <Icon name={volume == 0 ? 'volumeX' : 'volume2'} size={18} />
               <input type="range" min="0" max="1" step="0.01" value={volume} on:input={handleVolume} />
            </div>
            <div class="speed-control">
               <button class="text-btn" on:click={() => audioRef.playbackRate = audioRef.playbackRate >= 2 ? 1 : audioRef.playbackRate + 0.5}>
                 {audioRef?.playbackRate}x
               </button>
            </div>
          </div>
        </div>
      </div>

      <div class="right-side" bind:this={lyricContainer}>
        <div class="lyrics-container">
          {#if parsedLyrics.length > 0}
            <div class="lyrics-list">
              {#each parsedLyrics as line, i}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <p 
                  class="lyric-line" 
                  class:active={i === currentLyricIndex}
                  on:click={() => line.time > 0 && (audioRef.currentTime = line.time)}
                >
                  {line.text}
                </p>
              {/each}
            </div>
          {:else}
            <div class="no-lyrics">
                <Icon name="music" size={64} color="rgba(255,255,255,0.05)" />
                <p>Instrumental o sin letra</p>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>
</div>
{/if}

<style>
  .full-player {
    position: fixed;
    top: 0; left: 0; width: 100vw; height: 100vh;
    background: #000;
    z-index: 200;
    color: white;
    overflow: hidden;
  }

  .bg-blur {
    position: absolute;
    top: -10%; left: -10%; width: 120%; height: 120%;
    background-size: cover;
    background-position: center;
    filter: blur(60px) brightness(0.3);
    z-index: 1;
    transition: background-image 0.8s ease;
  }

  .vignette {
    position: absolute;
    top: 0; left: 0; width: 100%; height: 100%;
    background: radial-gradient(circle at center, transparent 20%, rgba(0,0,0,0.4) 100%);
    z-index: 2;
  }

  .main-content {
    position: relative;
    z-index: 3;
    height: 100%;
    display: flex;
    flex-direction: column;
    padding: 1.5rem 5vw; /* Reducido padding superior */
    max-width: 1400px;
    margin: 0 auto;
    overflow: hidden; /* Evitar scroll en el contenedor principal */
  }

  .player-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem; /* Reducido de 2rem */
    flex-shrink: 0; /* No se encoge */
  }

  .player-body {
    flex: 1;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4vw;
    align-items: center;
    min-height: 0; /* Crítico para que los hijos respeten el flex-grow */
    transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .left-side {
    display: flex;
    flex-direction: column;
    gap: 1.5rem; /* Reducido de 3rem */
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 0;
  }

  .cover-wrapper {
    aspect-ratio: 1;
    width: 100%;
    max-width: 400px; /* Reducido un poco */
    max-height: 45vh; /* Se adapta a la altura de la pantalla */
    border-radius: 24px;
    overflow: hidden;
    box-shadow: 0 30px 60px -12px rgba(0,0,0,0.7);
    flex-shrink: 1; /* Permite que la portada se haga más pequeña si no hay sitio */
  }
  
  .track-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    max-width: 450px;
    flex-shrink: 0;
  }

  .meta-texts h1 { 
    font-family: var(--font-headline);
    font-size: clamp(1.5rem, 5vh, var(--font-size-h1)); 
    font-weight: var(--font-weight-h1);
    letter-spacing: var(--letter-spacing-h1);
    margin: 0; 
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: 1.2rem;
    flex-wrap: wrap;
  }

  .quality-badge {
    background: rgba(255,255,255,0.08);
    padding: 0.25rem 0.7rem;
    border-radius: calc(var(--radius) / 4);
    font-size: 0.7rem;
    font-weight: 900;
    color: rgba(255,255,255,0.5);
    letter-spacing: 1px;
    border: 1px solid rgba(255,255,255,0.05);
  }

  .quality-badge.transcoded {
    background: var(--primary-10);
    color: var(--primary);
    border-color: rgba(255, 51, 102, 0.3);
  }

  .playback-controls {
    width: 100%;
    max-width: 450px;
    display: flex;
    flex-direction: column;
    gap: 1rem; /* Reducido */
    flex-shrink: 0;
  }

  .favorite-btn.active { color: var(--secondary); filter: drop-shadow(0 0 10px rgba(194, 112, 133, 0.3)); }
  .favorite-btn.active :global(svg) { fill: var(--secondary); }

  .sub-btn.active { color: var(--primary); opacity: 1; }

  .main-btns {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 0.5rem 0;
  }

  .play-btn {
    width: clamp(56px, 8vh, 72px);
    height: clamp(56px, 8vh, 72px);
    background: white;
    border: none;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
  }

  /* Ajuste de letras para que ocupen el máximo alto sin romper el layout */
  .right-side {
    height: 100%; /* Usa el alto del flex parent */
    overflow-y: auto;
    scrollbar-width: none;
    mask-image: linear-gradient(to bottom, transparent, black 10%, black 90%, transparent);
    padding: 0 1rem;
  }

  .lyric-line {
    font-size: clamp(1.5rem, 5vh, 2.8rem); /* Fuente elástica según altura */
    font-weight: 800;
    margin: 1.5rem 0;
    color: rgba(255,255,255,0.15);
    transition: all 0.5s ease;
    cursor: pointer;
    line-height: 1.1;
    letter-spacing: -1.5px;
    transform-origin: left center;
  }

  .lyric-line.active {
    color: white;
    transform: scale(1.08);
    opacity: 1;
    filter: drop-shadow(0 0 20px rgba(255,255,255,0.2));
  }

  .no-lyrics {
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 1.5rem;
      opacity: 0.4;
  }

  .mobile-only { display: none; }

  @media (max-width: 1000px) {
    .main-content { padding: 2rem 1.5rem; }
    .player-body { 
        display: block; 
        position: relative;
    }
    
    .left-side {
        display: flex;
        opacity: 1;
        transition: opacity 0.4s;
    }

    .right-side {
        position: absolute;
        top: 0; left: 0; width: 100%; height: 100%;
        opacity: 0;
        pointer-events: none;
        transition: opacity 0.4s;
    }

    .player-body.show-lyrics .left-side { opacity: 0; pointer-events: none; }
    .player-body.show-lyrics .right-side { opacity: 1; pointer-events: auto; }

    .mobile-only { display: block; }
    .desktop-only { display: none; }

    .meta-texts h1 { font-size: 2rem; }
    .lyric-line { font-size: 1.8rem; margin: 1.5rem 0; }
    .cover-wrapper { max-width: 320px; }
  }
</style>
