<script>
  import { tick } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade, fly } from 'svelte/transition';
  import { audioQuality, queue, currentIndex } from './stores.js';
  import ContextMenu from './ContextMenu.svelte';
  import StarRating from './StarRating.svelte';

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
  let showQueue = $state(false);
  let volume = $state(1);

  async function handleRate(track, rating) {
    try {
      const res = await fetch('/api/tracks/rate', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: track.id, rating })
      });
      if (res.ok) track.rating = rating;
    } catch (e) { console.error(e); }
  }

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
<div class="full-player-view">
  <div class="view-content">
    <div class="player-body" class:with-side={showLyrics || showQueue}>
      <div class="left-side">
        <div class="cover-wrapper-container">
          <div class="cover-wrapper">
             <img src="/api/cover/{currentPlaying.id}" alt="Cover" />
          </div>
        </div>
        
        <div class="player-island">
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
          </div>

          <div class="playback-controls">
            <div class="timeline-container">
              <div class="modern-slider progress-slider">
                 <input type="range" class="knobless-range" min="0" max="100" bind:value={progress} oninput={handleSeek} />
                 <div class="slider-fill" style="width: {progress}%"></div>
              </div>
              <div class="time-row">
                <span class="time">{formatTime(currentTime)}</span>
                <span class="time">{formatTime(duration)}</span>
              </div>
            </div>

            <div class="main-btns">
              <button class="sub-btn" class:active={false}><Icon name="shuffle" size={22} /></button>
              <button class="main-btn" onclick={() => { /* skipBack logic if needed */ }}><Icon name="skipBack" size={38} /></button>
              <button class="play-btn" onclick={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()}>
                <Icon name={audioRef?.paused ? 'play' : 'pause'} size={38} color="black" />
              </button>
              <button class="main-btn" onclick={() => { /* skipNext logic if needed */ }}><Icon name="skipForward" size={38} /></button>
              <button class="sub-btn" class:active={false}><Icon name="repeat" size={22} /></button>
            </div>

            <div class="bottom-controls">
              <div class="volume-slider-modern">
                 <Icon name={volume == 0 ? 'volumeX' : 'volume2'} size={20} />
                 <div class="modern-slider v-slider-large">
                   <input type="range" class="knobless-range" min="0" max="1" step="0.01" bind:value={volume} oninput={handleVolume} />
                   <div class="slider-fill" style="width: {volume * 100}%"></div>
                 </div>
              </div>
              
              <div class="player-nav-tabs">
                <button class="nav-tab" class:active={!showLyrics && !showQueue} onclick={() => { showLyrics = false; showQueue = false; }}>
                  <Icon name="image" size={20} />
                  <span>Portada</span>
                </button>
                <button class="nav-tab" class:active={showLyrics} onclick={() => { showLyrics = true; showQueue = false; }}>
                  <Icon name="music" size={20} />
                  <span>Letras</span>
                </button>
                <button class="nav-tab" class:active={showQueue} onclick={() => { showLyrics = false; showQueue = true; }}>
                  <Icon name="list" size={20} />
                  <span>Cola</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="right-side" class:visible={showLyrics || showQueue}>
        {#if showQueue}
          <div class="queue-container" in:fade>
            <h3>A continuación</h3>
            <div class="queue-list">
              {#each $queue.slice($currentIndex + 1) as next, i}
                <div class="queue-item">
                  <span class="q-index">{i + 1}</span>
                  <img src="/api/cover/{next.id}" alt="" class="q-cover" />
                  <div class="q-info">
                    <span class="q-title">{next.title}</span>
                    <span class="q-artist">{next.artist}</span>
                  </div>
                </div>
              {/each}
              {#if $queue.length <= $currentIndex + 1}
                 <p class="empty-queue">No hay más canciones en la cola</p>
              {/if}
            </div>
          </div>
        {:else}
          <div class="lyrics-container" bind:this={lyricContainer} in:fade>
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
        {/if}
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
  .full-player-view {
    position: relative;
    width: 100%;
    min-height: 100%;
    background: transparent;
    color: white;
    overflow: visible;
    display: flex;
    flex-direction: column;
    padding-bottom: 2rem;
  }





  .view-content {
    position: relative;
    z-index: 3;
    flex: 1;
    display: flex;
    flex-direction: column;
    width: 100%;
    max-width: 500px;
    margin: 0 auto;
  }



  .player-body {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    align-items: center;
    justify-content: flex-start;
    min-height: min-content;
    width: 100%;
  }

  .player-island {
    width: 100%;
    background: var(--surface-container);
    border-radius: 34px;
    border: 1px solid var(--border-color);
    box-shadow: 0 12px 40px rgba(0,0,0,0.25);
    padding: 2rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
    margin-top: -1rem; /* Subtle overlap with cover area */
    z-index: 2;
  }

  .left-side {
    display: flex;
    flex-direction: column;
    gap: 0;
    align-items: center;
    justify-content: flex-start;
    width: 100%;
  }

  .cover-wrapper-container {
    width: 100%;
    max-width: 300px;
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    align-items: center;
    z-index: 3; /* Above island */
  }

  .cover-wrapper {
    aspect-ratio: 1;
    width: 100%;
    height: auto;
    border-radius: 24px;
    overflow: hidden;
    box-shadow: 0 40px 80px -10px rgba(0,0,0,0.9);
    background: var(--surface-container-high);
    transition: transform 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    border: 1px solid rgba(255,255,255,0.1);
  }
  .cover-wrapper:hover { transform: scale(1.02); }



  .cover-wrapper img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  
  .track-meta {
    display: flex;
    justify-content: center;
    align-items: center;
    width: 100%;
    flex-shrink: 0;
    margin-bottom: -0.5rem;
  }

  .meta-texts h1 { 
    font-family: var(--font-headline);
    font-size: 2.2rem; 
    font-weight: 800;
    letter-spacing: -1px;
    margin: 0; 
    text-align: center;
    line-height: 1.1;
    color: white;
  }

  .title-row {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
  }

  .quality-badge {
    background: rgba(255,255,255,0.08);
    padding: 0.2rem 0.6rem;
    border-radius: 6px;
    font-size: 0.65rem;
    font-weight: 900;
    color: rgba(255,255,255,0.5);
    letter-spacing: 1px;
    border: 1px solid rgba(255,255,255,0.05);
  }

  .quality-badge.transcoded {
    background: var(--primary-10);
    color: var(--primary);
  }

  .artist-name { font-size: 1.1rem; font-weight: 700; opacity: 0.6; text-align: center; width: 100%; margin: 0.2rem 0 0 0; }

  /* Modern Knobless Sliders */
  .modern-slider { position: relative; width: 100%; height: 8px; background: rgba(255,255,255,0.1); border-radius: 20px; cursor: pointer; overflow: hidden; }
  .knobless-range { position: absolute; inset: 0; width: 100%; height: 100%; appearance: none; background: transparent; z-index: 5; cursor: pointer; margin: 0; }
  .slider-fill { position: absolute; top: 0; left: 0; height: 100%; background: white; border-radius: 20px; z-index: 4; transition: background 0.2s; }

  .timeline-container { width: 100%; display: flex; flex-direction: column; gap: 0.8rem; }
  .time-row { display: flex; justify-content: space-between; width: 100%; }
  .time { font-size: 0.8rem; font-weight: 700; opacity: 0.6; font-variant-numeric: tabular-nums; }

  .bottom-controls { 
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    padding-bottom: 2rem;
    width: 100%;
  }

  .volume-slider-modern { display: flex; align-items: center; gap: 1rem; width: 100%; justify-content: center; transform: scale(0.9); }
  .v-slider-large { height: 6px; flex: 1; }

  .player-nav-tabs { 
    display: flex; 
    background: rgba(255,255,255,0.08); 
    padding: 0.3rem; 
    border-radius: 100px; 
    gap: 0.4rem; 
    justify-content: center;
    width: 100%;
  }
  .nav-tab {
    background: transparent;
    border: none;
    color: rgba(255,255,255,0.4);
    padding: 0.8rem;
    border-radius: 100px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    cursor: pointer;
    transition: all 0.2s;
  }
  .nav-tab span { display: none; }
  .nav-tab.active { background: rgba(255,255,255,0.95); color: black; }
  .nav-tab:hover:not(.active) { color: white; background: rgba(255,255,255,0.1); }
  


  .playback-controls {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    flex-shrink: 0;
  }

  .main-btns {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 0;
    transform: scale(0.85);
  }

  .play-btn {
    width: 72px;
    height: 72px;
    background: white;
    border: none;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    box-shadow: 0 10px 30px rgba(255,255,255,0.2);
  }
  .play-btn:hover { transform: scale(1.08); background: #eee; }
  .play-btn:active { transform: scale(0.95); }

  .main-btn { color: white; opacity: 0.8; }
  .main-btn:hover { opacity: 1; transform: scale(1.1); }
  .sub-btn { color: white; opacity: 0.4; }
  .sub-btn.active { color: var(--primary); opacity: 1; }

  /* Queue and Lyrics share the same style foundation */
  .right-side {
    display: none;
    width: 100%;
    flex-direction: column;
    gap: 1.5rem;
    padding: 0 0 5rem 0;
    min-height: 400px;
  }
  .right-side.visible { display: flex; }

  .queue-container { display: flex; flex-direction: column; gap: 1.5rem; }
  .queue-container h3 { margin: 0; font-size: 1.3rem; font-weight: 900; letter-spacing: -0.5px; text-align: center; }
  .queue-list { display: flex; flex-direction: column; gap: 0.5rem; }
  
  .queue-item { 
    display: flex; 
    align-items: center; 
    gap: 1rem; 
    padding: 0.6rem; 
    border-radius: 12px; 
    background: rgba(255,255,255,0.03); 
    border: 1px solid rgba(255,255,255,0.05);
  }
  .q-index { font-size: 0.7rem; font-weight: 900; opacity: 0.3; width: 14px; }
  .q-cover { width: 40px; height: 40px; border-radius: 6px; object-fit: cover; }
  .q-info { display: flex; flex-direction: column; gap: 1px; overflow: hidden; }
  .q-title { font-weight: 700; font-size: 0.85rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .q-artist { font-size: 0.75rem; opacity: 0.5; font-weight: 600; }

  .lyrics-container { display: flex; flex-direction: column; gap: 1rem; }
  .lyrics-list { display: flex; flex-direction: column; align-items: center; }
  .lyric-line {
    font-size: 1.5rem;
    font-weight: 800;
    margin: 0.8rem 0;
    color: rgba(255,255,255,0.2);
    transition: all 0.4s ease;
    cursor: pointer;
    line-height: 1.1;
    letter-spacing: -0.8px;
    text-align: center;
    background: none; border: none; padding: 0;
  }
  .lyric-line.active { color: white; transform: scale(1.05); filter: drop-shadow(0 0 15px rgba(255,255,255,0.2)); }
  .no-lyrics { text-align: center; padding: 3rem; opacity: 0.4; display: flex; flex-direction: column; align-items: center; gap: 1rem; }


</style>
