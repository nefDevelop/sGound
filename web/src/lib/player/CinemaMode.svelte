<script>
  import { tick } from 'svelte';
  import { fade } from 'svelte/transition';
  import Icon from '../Icon.svelte';

  let {
    currentPlaying,
    audioRef,
    progress,
    duration,
    currentTime,
    parsedLyrics = [],
    lyrics = "",
    currentLyricIndex = -1,
    coverRefreshTrigger,
    showControls = $bindable(true),
    onClose,
    onPlayToggle,
    onSkipBack,
    onSkipNext,
    onSeek,
    onUserInteraction
  } = $props();

  let cinemaContainer = $state();
  let lyricContainerCinema = $state();
  let isFullscreen = $state(false);
  let tiltX = $state(0);
  let tiltY = $state(0);
  let controlsTimeout;

  $effect(() => {
    const handler = () => {
      isFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', handler);
    return () => document.removeEventListener('fullscreenchange', handler);
  });

  function toggleFullscreen(e) {
    if (e) e.stopPropagation();
    if (!document.fullscreenElement) {
      if (cinemaContainer) {
        cinemaContainer.requestFullscreen().catch(err => console.error(err));
      }
    } else {
      if (document.exitFullscreen) document.exitFullscreen();
    }
  }

  function handleMouseMove(e) {
    const { clientX, clientY } = e;
    const { innerWidth, innerHeight } = window;
    tiltX = (clientX / innerWidth - 0.5) * 15;
    tiltY = (clientY / innerHeight - 0.5) * -15;
    
    showControls = true;
    clearTimeout(controlsTimeout);
    controlsTimeout = setTimeout(() => {
      showControls = false;
    }, 3000);
  }

  function formatTime(secs) {
    if (isNaN(secs)) return "0:00";
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${mins}:${s.toString().padStart(2, '0')}`;
  }
  
  // Reactively scroll to active lyric
  $effect(() => {
      if (currentLyricIndex !== -1 && lyricContainerCinema && !userScrolling) {
          tick().then(() => {
              const activeLine = lyricContainerCinema.querySelector('.active');
              if (activeLine) activeLine.scrollIntoView({ behavior: 'smooth', block: 'center' });
          });
      }
  });

  let userScrolling = $state(false);
  let scrollTimeout;
  function handleScroll(e) {
      userScrolling = true;
      clearTimeout(scrollTimeout);
      scrollTimeout = setTimeout(() => { userScrolling = false; }, 3000);
      if (onUserInteraction) onUserInteraction(e);
  }
</script>

<div 
  role="presentation"
  class="cinema-view-container" 
  class:hide-ui={!showControls}
  bind:this={cinemaContainer}
  in:fade={{ duration: 800 }}
  onmousemove={handleMouseMove}
  onclick={onPlayToggle}
>
  <!-- Liquid Ambient Background -->
  <div class="cinema-ambient-scene">
    <div class="ambient-blob blob-1" style="background-image: url('/api/cover/{currentPlaying?.id}?v={$coverRefreshTrigger}')"></div>
    <div class="ambient-blob blob-2" style="background-image: url('/api/cover/{currentPlaying?.id}?v={$coverRefreshTrigger}')"></div>
    <div class="ambient-blob blob-3" style="background-image: url('/api/cover/{currentPlaying?.id}?v={$coverRefreshTrigger}')"></div>
    <div class="ambient-noise"></div>
    <div class="ambient-vignette"></div>
  </div>

  <!-- Top UI: Actions -->
  <div class="cinema-top-ui" class:visible={showControls}>
    <div class="cinema-top-btns">
      <button class="cinema-action-btn" title="Pantalla Completa" onclick={toggleFullscreen}>
        <Icon name={isFullscreen ? "minimize" : "maximize"} size={20} />
      </button>
      <button class="cinema-exit-btn" onclick={(e) => { e.stopPropagation(); onClose(); }}>
        <Icon name="x" size={20} />
        <span>Salir</span>
      </button>
    </div>
  </div>

  <div class="cinema-main-layout">
    <!-- Left: 3D Artwork & Dynamic Meta -->
    <div class="cinema-artwork-pane">
      <div class="artwork-3d-container">
        <div class="artwork-card-wrapper" style="transform: rotateX({tiltY}deg) rotateY({tiltX}deg)">
            <img src="/api/cover/{currentPlaying?.id}?v={$coverRefreshTrigger}" alt="" class="artwork-main-img" />
            <div class="artwork-reflection"></div>
        </div>
        <div class="artwork-shadow-glow" style="background-image: url('/api/cover/{currentPlaying?.id}?v={$coverRefreshTrigger}')"></div>
      </div>
      
      <div class="cinema-track-info" class:visible={showControls}>
        <h1 class="cinema-title-text">{currentPlaying?.title || 'Unknown Title'}</h1>
        <div class="cinema-artist-row">
            <span class="cinema-artist-text">{currentPlaying?.artist || 'Unknown Artist'}</span>
            {#if currentPlaying?.album}
              <span class="cinema-separator">•</span>
              <span class="cinema-album-text">{currentPlaying.album}</span>
            {/if}
        </div>
      </div>
    </div>

    <!-- Right: Immersive Lyrics -->
    <div class="cinema-lyrics-pane">
      {#if parsedLyrics.length > 0}
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <div class="cinema-lyrics-scroller" 
              bind:this={lyricContainerCinema}
              onwheel={handleScroll}
              onmousedown={handleScroll}
              ontouchstart={handleScroll}
              role="region"
              aria-label="Letras inmersivas"
        >
          {#each parsedLyrics as line, i}
            <button class="cinema-lyric-item" 
                  class:active={i === currentLyricIndex}
                  class:nearby={Math.abs(i - currentLyricIndex) <= 2 && i !== currentLyricIndex}
                  class:distant={Math.abs(i - currentLyricIndex) > 2}
                  onclick={(e) => { e.stopPropagation(); if(audioRef) audioRef.currentTime = line.time; }}>
              {line.text}
            </button>
          {/each}
        </div>
      {:else if lyrics === "Loading..."}
          <div class="cinema-empty-state">
            <div class="loading-pulse"></div>
            <p>Sincronizando letras...</p>
          </div>
      {:else}
        <div class="cinema-empty-state">
          <div style="opacity: 0.2">
            <Icon name="music" size={64} />
          </div>
          <p>Solo música, sin palabras</p>
        </div>
      {/if}
    </div>
  </div>

  <!-- Bottom UI: Glass Capsule Controls -->
  <div class="cinema-bottom-ui" class:visible={showControls} onclick={(e) => e.stopPropagation()} role="presentation">
    <div class="glass-capsule">
      <div class="capsule-progress-bar">
        <input type="range" class="capsule-range" min="0" max="100" value={progress} oninput={onSeek} />
        <div class="capsule-fill" style="width: {progress}%"></div>
      </div>
      
      <div class="capsule-main-row">
        <div class="capsule-time">{formatTime(currentTime)}</div>
        
        <div class="capsule-btns">
          <button class="capsule-btn" onclick={(e) => { e.stopPropagation(); onSkipBack(); }}><Icon name="skipBack" size={24} /></button>
          <button class="capsule-btn-play" onclick={(e) => { e.stopPropagation(); onPlayToggle(); }}>
            <Icon name={audioRef?.paused ? 'play' : 'pause'} size={32} />
          </button>
          <button class="capsule-btn" onclick={(e) => { e.stopPropagation(); onSkipNext(); }}><Icon name="skipForward" size={24} /></button>
        </div>

        <div class="capsule-time">{formatTime(duration)}</div>
      </div>
    </div>
  </div>
</div>

<style>
  .cinema-view-container {
    position: absolute;
    inset: 0;
    z-index: 9999;
    background: #000;
    color: #fff;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    cursor: default;
    min-height: 100%;
  }

  .cinema-view-container:fullscreen {
    width: 100vw;
    height: 100vh;
    position: fixed;
  }

  .cinema-view-container:-webkit-full-screen {
    width: 100vw;
    height: 100vh;
  }

  .cinema-view-container.hide-ui {
    cursor: none;
  }

  /* Liquid Ambient Background */
  .cinema-ambient-scene {
    position: absolute;
    inset: -10%;
    z-index: 1;
    pointer-events: none;
    overflow: hidden;
    filter: blur(120px) saturate(1.8) brightness(0.4);
  }

  .ambient-blob {
    position: absolute;
    width: 100%;
    height: 100%;
    background-size: cover;
    background-position: center;
    border-radius: 50%;
    opacity: 0.6;
  }

  .blob-1 {
    width: 120%;
    height: 120%;
    top: -10%;
    left: -10%;
    animation: liquid-move-1 40s infinite alternate ease-in-out;
  }
  .blob-2 {
    width: 100%;
    height: 100%;
    bottom: -10%;
    right: -10%;
    animation: liquid-move-2 30s infinite alternate-reverse ease-in-out;
    opacity: 0.4;
  }
  .blob-3 {
    width: 80%;
    height: 80%;
    top: 20%;
    left: 20%;
    animation: liquid-move-3 25s infinite alternate ease-in-out;
    opacity: 0.3;
  }

  @keyframes liquid-move-1 {
    0% { transform: translate(0, 0) scale(1) rotate(0deg); }
    100% { transform: translate(10%, 5%) scale(1.1) rotate(10deg); }
  }
  @keyframes liquid-move-2 {
    0% { transform: translate(0, 0) scale(1.1) rotate(0deg); }
    100% { transform: translate(-8%, -10%) scale(1) rotate(-8deg); }
  }
  @keyframes liquid-move-3 {
    0% { transform: translate(0, 0) scale(0.9); }
    100% { transform: translate(15%, -15%) scale(1.2); }
  }

  .ambient-noise {
    position: absolute;
    inset: 0;
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.65' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)'/%3E%3C/svg%3E");
    opacity: 0.05;
    mix-blend-mode: overlay;
    z-index: 4;
  }

  .ambient-vignette {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at center, transparent 0%, rgba(0,0,0,0.4) 60%, rgba(0,0,0,0.8) 100%);
    z-index: 5;
  }

  /* Top UI */
  .cinema-top-ui {
    position: absolute;
    top: 3rem;
    left: 4rem;
    right: 4rem;
    display: flex;
    justify-content: flex-end;
    z-index: 100;
    opacity: 0;
    transform: translateY(-20px);
    transition: all 0.6s cubic-bezier(0.16, 1, 0.3, 1);
    pointer-events: none;
  }

  .cinema-top-ui.visible {
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }

  .cinema-top-btns {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: rgba(255, 255, 255, 0.08);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    padding: 0.5rem;
    border-radius: 100px;
  }

  .cinema-action-btn, .cinema-exit-btn {
    background: transparent;
    border: none;
    color: #fff;
    height: 42px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .cinema-action-btn {
    width: 42px;
    border-radius: 50%;
  }

  .cinema-exit-btn {
    padding: 0 1.2rem;
    border-radius: 100px;
    gap: 0.5rem;
    font-weight: 600;
    font-size: 0.9rem;
    background: rgba(255, 255, 255, 0.1);
  }

  .cinema-action-btn:hover, .cinema-exit-btn:hover {
    background: rgba(255, 255, 255, 0.2);
    transform: scale(1.05);
  }

  .cinema-exit-btn:hover {
    background: rgba(255, 255, 255, 0.15);
    transform: scale(1.05);
  }

  /* Main Layout */
  .cinema-main-layout {
    flex: 1;
    display: flex;
    width: 100%;
    max-width: 1600px;
    margin: 0 auto;
    padding: clamp(2rem, 5vh, 6rem) 4rem;
    align-items: center;
    justify-content: center;
    gap: clamp(2rem, 5vw, 6rem);
    z-index: 10;
    position: relative;
    box-sizing: border-box;
  }

  /* Left Pane: Artwork */
  .cinema-artwork-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    max-width: 600px;
  }

  .artwork-3d-container {
    position: relative;
    width: 100%;
    max-width: clamp(180px, 35vh, 450px);
    aspect-ratio: 1;
    perspective: 1200px;
    margin-bottom: 1.5rem;
  }

  .artwork-card-wrapper {
    width: 100%;
    height: 100%;
    position: relative;
    z-index: 2;
    border-radius: 24px;
    overflow: hidden;
    box-shadow: 0 30px 60px rgba(0,0,0,0.8);
    transform: rotateX(2deg) rotateY(-5deg);
    transition: transform 0.8s cubic-bezier(0.16, 1, 0.3, 1);
    animation: artwork-float 12s infinite alternate ease-in-out;
  }

  @keyframes artwork-float {
    0% { transform: rotateX(2deg) rotateY(-5deg) translateY(0); }
    100% { transform: rotateX(4deg) rotateY(3deg) translateY(-20px); }
  }

  .artwork-main-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .artwork-reflection {
    position: absolute;
    inset: 0;
    background: linear-gradient(135deg, rgba(255,255,255,0.15) 0%, transparent 40%, rgba(255,255,255,0.05) 100%);
    z-index: 3;
    pointer-events: none;
  }

  .artwork-shadow-glow {
    position: absolute;
    inset: -5%;
    background-size: cover;
    background-position: center;
    filter: blur(50px) saturate(2);
    opacity: 0.5;
    z-index: 1;
    border-radius: 40px;
    transform: translateY(20px) scale(0.95);
  }

  .cinema-track-info {
    text-align: center;
    width: 100%;
    opacity: 0;
    transform: translateY(20px);
    transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.2s;
  }

  .cinema-track-info.visible {
    opacity: 1;
    transform: translateY(0);
  }

  .cinema-title-text {
    font-size: clamp(2rem, 5vh, 3.5rem);
    font-weight: 900;
    letter-spacing: -2px;
    margin: 0 0 0.5rem 0;
    line-height: 1.1;
    text-shadow: 0 10px 30px rgba(0,0,0,0.6);
  }

  .cinema-artist-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.8rem;
    font-size: clamp(1rem, 2.5vh, 1.6rem);
    font-weight: 600;
    opacity: 0.7;
  }

  .cinema-separator {
    opacity: 0.3;
  }

  /* Right Pane: Lyrics */
  .cinema-lyrics-pane {
    flex: 1.2;
    height: 100%;
    max-height: 60vh;
    position: relative;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .cinema-lyrics-scroller {
    height: 100%;
    overflow-y: auto;
    scrollbar-width: none;
    padding: 30vh 100px 30vh 40px; /* Increased horizontal padding for glow */
    scroll-behavior: smooth;
    mask-image: linear-gradient(to bottom, transparent, black 25%, black 75%, transparent);
    -webkit-mask-image: linear-gradient(to bottom, transparent, black 25%, black 75%, transparent);
  }

  .cinema-lyrics-scroller::-webkit-scrollbar { display: none; }

  .cinema-lyric-item {
    font-size: 3.2rem;
    font-weight: 800;
    line-height: 1.25;
    padding: 1.2rem 0;
    margin: 0;
    color: rgba(255,255,255,0.25);
    text-align: left;
    background: none;
    border: none;
    cursor: pointer;
    width: 100%;
    transition: all 0.7s cubic-bezier(0.16, 1, 0.3, 1);
    transform-origin: left center;
    letter-spacing: -1.5px;
    display: block;
    filter: blur(2px);
    transform: scale(0.95);
    overflow-wrap: break-word; /* Ensure long words wrap */
    word-break: break-word;
    hyphens: auto;
  }

  .cinema-lyric-item:hover {
    color: rgba(255,255,255,0.6);
    filter: blur(0);
  }

  .cinema-lyric-item.active {
    color: #fff;
    transform: scale(1.05) translateX(20px);
    filter: blur(0);
    text-shadow: 0 0 40px rgba(255,255,255,0.4), 0 10px 20px rgba(0,0,0,0.5);
    padding: 2.5rem 0;
  }

  .cinema-lyric-item.nearby {
    color: rgba(255,255,255,0.5);
    filter: blur(0.5px);
    transform: scale(0.98);
  }

  .cinema-empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 2rem;
    color: rgba(255,255,255,0.4);
  }

  .cinema-empty-state p {
    font-size: 1.8rem;
    font-weight: 700;
    letter-spacing: 1px;
    text-transform: uppercase;
  }

  .loading-pulse {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    background: #fff;
    opacity: 0.1;
    animation: pulse 2s infinite ease-in-out;
  }

  @keyframes pulse {
    0% { transform: scale(0.8); opacity: 0.1; }
    50% { transform: scale(1.2); opacity: 0.3; }
    100% { transform: scale(0.8); opacity: 0.1; }
  }

  /* Bottom UI: Capsule */
  .cinema-bottom-ui {
    position: absolute;
    bottom: 3rem;
    left: 0;
    width: 100%;
    display: flex;
    justify-content: center;
    z-index: 200;
    opacity: 0;
    transform: translateY(30px);
    transition: all 0.6s cubic-bezier(0.16, 1, 0.3, 1);
    pointer-events: none;
  }

  .cinema-bottom-ui.visible {
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }

  .glass-capsule {
    background: rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(30px) saturate(2);
    -webkit-backdrop-filter: blur(30px) saturate(2);
    border: 1px solid rgba(255, 255, 255, 0.15);
    padding: 1.2rem 2rem;
    border-radius: 40px;
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    width: auto;
    min-width: 500px;
    box-shadow: 0 25px 60px rgba(0,0,0,0.6);
  }

  .capsule-progress-bar {
    position: relative;
    width: calc(100% - 1rem);
    margin: 0 auto;
    height: 6px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 10px;
    overflow: hidden;
  }

  .capsule-fill {
    position: absolute;
    height: 100%;
    background: #fff;
    box-shadow: 0 0 15px rgba(255,255,255,0.8);
    transition: width 0.1s linear;
  }

  .capsule-range {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
    z-index: 10;
  }

  .capsule-main-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 2rem;
  }

  .capsule-time {
    font-size: 0.85rem;
    font-weight: 700;
    opacity: 0.5;
    font-variant-numeric: tabular-nums;
    min-width: 60px;
    text-align: center;
  }

  .capsule-btns {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  .capsule-btn {
    background: none;
    border: none;
    color: #fff;
    opacity: 0.6;
    cursor: pointer;
    transition: all 0.3s;
    padding: 0.5rem;
    display: flex;
    align-items: center;
  }

  .capsule-btn:hover {
    opacity: 1;
    transform: scale(1.15);
  }

  .capsule-btn-play {
    background: #fff;
    color: #000;
    border: none;
    width: 56px;
    height: 56px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    box-shadow: 0 8px 20px rgba(0,0,0,0.3);
  }

  .capsule-btn-play:hover {
    transform: scale(1.1);
    box-shadow: 0 15px 30px rgba(255,255,255,0.2);
  }

  /* Responsive Adjustments */
  @media (max-width: 1400px) {
    .cinema-title-text { font-size: 2.8rem; }
    .cinema-lyric-item { font-size: 2.6rem; }
    .cinema-main-layout { gap: 4rem; }
  }

  @media (max-width: 1024px) {
    .cinema-main-layout {
      flex-direction: column;
      padding: 5rem 2rem 10rem 2rem;
      gap: 2rem;
      justify-content: flex-start;
      overflow-y: auto;
    }

    .cinema-artwork-pane {
      max-width: none;
      flex: none;
    }

    .artwork-3d-container {
      max-width: 300px;
      margin-bottom: 2rem;
    }

    .cinema-title-text { font-size: 2.2rem; }
    .cinema-artist-row { font-size: 1.2rem; }

    .cinema-lyrics-pane {
      width: 100%;
      height: auto;
      flex: none;
    }

    .cinema-lyrics-scroller {
      padding: 0;
      mask-image: none;
      -webkit-mask-image: none;
      height: auto;
    }

    .cinema-lyric-item {
      font-size: 1.8rem;
      text-align: center;
      padding: 1rem 0;
    }
    
    .cinema-lyric-item.active {
      transform: scale(1.05) translateX(0);
      padding: 1.5rem 0;
    }

    .glass-capsule {
      min-width: 0;
      width: 92vw;
      padding: 0.8rem 1.2rem;
    }
    .capsule-main-row {
      gap: 0.5rem;
    }

    .cinema-top-ui {
      top: 2rem;
      left: 1rem;
      right: 1rem;
    }
  }

  @media (max-width: 480px) {
    .cinema-title-text { font-size: 1.8rem; }
    .cinema-lyric-item { font-size: 1.5rem; }
    .capsule-time { display: none; }
    .glass-capsule { padding: 1rem; }
    .cinema-bottom-ui { bottom: 1.5rem; }
  }
</style>