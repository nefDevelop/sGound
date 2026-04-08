<script>
  import { tick } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade, fly } from 'svelte/transition';
  import { audioQuality } from './stores.js';
  import ContextMenu from './ContextMenu.svelte';

  let { 
    currentPlaying = null, 
    audioRef = null, 
    isOpen = $bindable(false), 
    progress = $bindable(0), 
    duration = $bindable(0), 
    currentTime = $bindable(0),
    onedit,
    ondelete
  } = $props();

  let lyrics = $state("");
  let parsedLyrics = $state([]);
  let currentLyricIndex = $state(-1);
  let lyricContainer = $state(null);
  let showLyrics = $state(false);
  let volume = $state(1);

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    items: []
  });

  function showContextMenu(e) {
    if (!currentPlaying) return;
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      items: [
        { label: 'Editar etiquetas', icon: 'edit', action: () => onedit?.(currentPlaying) },
        { divider: true },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(currentPlaying.title) },
        { label: 'Eliminar de biblioteca', icon: 'xCircle', color: 'danger', action: () => ondelete?.(currentPlaying) }
      ]
    };
  }

  // Sincronizar letras
  $effect(() => {
    if (parsedLyrics.length > 0) {
      const index = parsedLyrics.findIndex((l, i) => {
        const next = parsedLyrics[i + 1];
        return currentTime >= l.time && (!next || currentTime < next.time);
      });
      if (index !== currentLyricIndex && index !== -1) {
        currentLyricIndex = index;
        tick().then(scrollToActive);
      }
    }
  });

  // Cargar letras
  $effect(() => {
    if (currentPlaying && isOpen) {
      loadLyrics();
      if (audioRef) volume = audioRef.volume;
    }
  });

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
    if (!audioRef) return;
    const seekTime = (e.target.value / 100) * duration;
    audioRef.currentTime = seekTime;
  }

  function handleVolume(e) {
    if (!audioRef) return;
    volume = e.target.value;
    audioRef.volume = volume;
  }
</script>

{#if isOpen && currentPlaying}
<div class="full-player" in:fly={{ y: '100%', duration: 400 }} out:fly={{ y: '100%', duration: 300 }}>
  <!-- Background blur -->
  <div class="bg-blur" style="background-image: url(/api/cover/{currentPlaying.id})"></div>
  <div class="vignette"></div>
  
  <div class="main-content">
    <header class="player-header">
      <button class="icon-btn" onclick={() => isOpen = false} title="Close">
        <Icon name="chevronDown" size={28} />
      </button>
      <div class="header-info">
        <span class="playing-from">Now Playing</span>
        <span class="album-name">{currentPlaying.album}</span>
      </div>
      <button class="icon-btn mobile-only" onclick={() => showLyrics = !showLyrics}>
        <Icon name={showLyrics ? 'music' : 'list'} size={24} />
      </button>
      <div class="desktop-only" style="width: 28px"></div>
    </header>

    <div class="player-body" class:show-lyrics={showLyrics}>
      <div class="left-side">
        <div class="cover-wrapper">
          <img src="/api/cover/{currentPlaying.id}" alt="Cover" />
        </div>
        
        <div class="track-meta" oncontextmenu={showContextMenu} role="button" tabindex="0">
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
               <input type="range" class="full-progress" min="0" max="100" bind:value={progress} oninput={handleSeek} />
               <div class="progress-fill" style="width: {progress}%"></div>
            </div>
            <span class="time">{formatTime(duration)}</span>
          </div>

          <div class="main-btns">
            <button class="sub-btn" class:active={false}><Icon name="shuffle" size={20} /></button>
            <button class="main-btn" onclick={() => { /* skipBack logic if needed */ }}><Icon name="skipBack" size={32} /></button>
            <button class="play-btn" onclick={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()}>
              <Icon name={audioRef?.paused ? 'play' : 'pause'} size={32} color="black" />
            </button>
            <button class="main-btn" onclick={() => { /* skipNext logic if needed */ }}><Icon name="skipForward" size={32} /></button>
            <button class="sub-btn" class:active={false}><Icon name="repeat" size={20} /></button>
          </div>

          <div class="extra-controls">
            <div class="volume-control">
               <Icon name={volume == 0 ? 'volumeX' : 'volume2'} size={18} />
               <input type="range" min="0" max="1" step="0.01" bind:value={volume} oninput={handleVolume} />
            </div>
            <div class="speed-control">
               <button class="text-btn" onclick={() => audioRef && (audioRef.playbackRate = audioRef.playbackRate >= 2 ? 1 : audioRef.playbackRate + 0.5)}>
                 {audioRef?.playbackRate || 1}x
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
                <button 
                  class="lyric-line" 
                  class:active={i === currentLyricIndex}
                  onclick={() => line.time > 0 && audioRef && (audioRef.currentTime = line.time)}
                >
                  {line.text}
                </button>
              {/each}
            </div>
          {:else}
            <div class="no-lyrics">
                <Icon name="music" size={64} />
                <p>Instrumental o sin letra</p>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    onclose={() => contextMenu.visible = false}
  />
</div>
{/if}

<style>
  .full-player {
    position: fixed;
    top: 0; left: 0; width: 100vw; height: 100vh;
    background: #000;
    z-index: 2000;
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
    padding: 1.5rem 5vw;
    max-width: 1400px;
    margin: 0 auto;
    overflow: hidden;
  }

  .player-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
    flex-shrink: 0;
  }

  .header-info { text-align: center; }
  .playing-from { display: block; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 2px; opacity: 0.5; margin-bottom: 0.2rem; }
  .album-name { font-weight: 800; font-size: 0.9rem; }

  .player-body {
    flex: 1;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4vw;
    align-items: center;
    min-height: 0;
    transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .left-side {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 0;
  }

  .cover-wrapper {
    aspect-ratio: 1;
    width: 100%;
    height: auto;
    max-width: min(85vw, 400px);
    max-height: 45vh;
    border-radius: 28px;
    overflow: hidden;
    box-shadow: 0 40px 80px -15px rgba(0,0,0,0.8);
    flex-shrink: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--surface-container-high);
  }

  .cover-wrapper img {
    width: 100%;
    height: 100%;
    object-fit: cover;
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
    font-size: clamp(1.5rem, 5vh, 3rem); 
    font-weight: 950;
    letter-spacing: -1.5px;
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
    border-radius: 8px;
    font-size: 0.7rem;
    font-weight: 900;
    color: rgba(255,255,255,0.5);
    letter-spacing: 1px;
    border: 1px solid rgba(255,255,255,0.05);
  }

  .quality-badge.transcoded {
    background: var(--primary-10);
    color: var(--primary);
  }

  .artist-name { font-size: 1.2rem; font-weight: 700; opacity: 0.6; margin-top: 0.3rem; }

  .playback-controls {
    width: 100%;
    max-width: 450px;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    flex-shrink: 0;
  }

  .timeline { display: flex; align-items: center; gap: 1rem; }
  .progress-container { flex: 1; position: relative; height: 6px; display: flex; align-items: center; }
  .full-progress { width: 100%; appearance: none; background: rgba(255,255,255,0.1); height: 100%; border-radius: 10px; cursor: pointer; z-index: 2; opacity: 0; }
  .progress-fill { position: absolute; height: 100%; background: white; border-radius: 10px; z-index: 1; pointer-events: none; }
  .progress-container::after { content: ''; position: absolute; width:100%; height:100%; background: rgba(255,255,255,0.1); border-radius: 10px; }

  .main-btns {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 0.5rem 0;
  }

  .play-btn {
    width: clamp(64px, 10vh, 80px);
    height: clamp(64px, 10vh, 80px);
    background: white;
    border: none;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: transform 0.2s;
  }
  .play-btn:hover { transform: scale(1.05); }

  .extra-controls { display: flex; justify-content: space-between; align-items: center; margin-top: 1rem; opacity: 0.6; }
  .volume-control { display: flex; align-items: center; gap: 0.8rem; }
  .volume-control input { width: 80px; accent-color: white; }

  .right-side {
    height: 100%;
    overflow-y: auto;
    scrollbar-width: none;
    mask-image: linear-gradient(to bottom, transparent, black 10%, black 90%, transparent);
    padding: 2rem 1rem;
    display: flex;
    flex-direction: column;
  }

  .lyric-line {
    font-size: clamp(1.5rem, 5vh, 2.8rem);
    font-weight: 800;
    margin: 1.2rem 0;
    color: rgba(255,255,255,0.2);
    transition: all 0.4s ease;
    cursor: pointer;
    line-height: 1.1;
    letter-spacing: -1.5px;
    text-align: left;
    background: none; border: none; padding: 0;
  }

  .lyric-line.active {
    color: white;
    transform: scale(1.05);
    filter: drop-shadow(0 0 20px rgba(255,255,255,0.3));
  }

  .mobile-only { display: none; }
  .icon-btn { background: transparent; border: none; color: white; cursor: pointer; opacity: 0.8; transition: 0.2s; }
  .icon-btn:hover { opacity: 1; transform: scale(1.1); }

  @media (max-width: 1000px) {
    .player-body { display: block; position: relative; }
    .left-side { opacity: 1; transition: opacity 0.4s; height: auto; padding: 2rem 0; }
    .right-side { position: absolute; top: 0; left: 0; width: 100%; height: 100%; opacity: 0; pointer-events: none; transition: opacity 0.4s; }
    .player-body.show-lyrics .left-side { opacity: 0; pointer-events: none; }
    .player-body.show-lyrics .right-side { opacity: 1; pointer-events: auto; }
    .mobile-only { display: block; }
    .desktop-only { display: none; }
    .lyric-line { font-size: 1.8rem; }
    .cover-wrapper { max-width: 320px; }
  }
</style>
