<script>
  import { tick } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade, fly } from 'svelte/transition';
  import { audioQuality, queue, currentIndex, volume } from './stores.js';
  import ContextMenu from './ContextMenu.svelte';
  import StarRating from './StarRating.svelte';

  let { 
    currentPlaying = null, 
    audioRef = null, 
    isOpen = $bindable(false), 
    progress = $bindable(0), 
    duration = $bindable(0), 
    currentTime = $bindable(0),
    subView = 'cover',
    onedit,
    onhide,
    ondeletePhysical
  } = $props();

  let lyrics = $state("");
  let parsedLyrics = $state([]);
  let currentLyricIndex = $state(-1);
  let lyricContainer = $state(null);

  function toggleMute() {
    if ($volume > 0) {
      localStorage.setItem('lastVolume', $volume.toString());
      volume.set(0);
    } else {
      const lastV = parseFloat(localStorage.getItem('lastVolume')) || 0.5;
      volume.set(lastV);
    }
    if (audioRef) audioRef.volume = $volume;
  }

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
        { divider: true },
        { label: 'Ocultar de la biblioteca', icon: 'eyeOff', action: () => onhide?.(currentPlaying) },
        { label: 'Borrar físicamente (Disco)', icon: 'trash', color: 'danger', action: () => ondeletePhysical?.(currentPlaying) }
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

  $effect(() => {
    if (currentPlaying && isOpen) {
      loadLyrics();
      if (audioRef) volume.set(audioRef.volume);
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
    if (!audioRef || !duration || isNaN(duration)) return;
    const seekTime = (parseFloat(e.target.value) / 100) * duration;
    audioRef.currentTime = seekTime;
  }

  function handleVolume(e) {
    if (!audioRef) return;
    const v = parseFloat(e.target.value);
    volume.set(v);
    audioRef.volume = v;
  }
</script>

{#if isOpen && currentPlaying}
<div class="full-player-view">
  <div class="view-content">
    <div class="player-body" class:compact-mode={subView !== 'cover'}>
      <div class="main-content-flow">
        <header class="player-header">
          <div class="cover-wrapper-container">
            <div class="cover-wrapper">
               <img src="/api/cover/{currentPlaying.id}" alt="Cover" />
            </div>
          </div>
          
          {#if subView !== 'cover'}
            <div class="compact-meta" in:fade>
              <div class="meta-texts">
                <h2>{currentPlaying.title}</h2>
                <p>{currentPlaying.artist}</p>
              </div>
            </div>
          {/if}
        </header>
        
        <div class="dynamic-area">
          {#if subView === 'cover'}
            <div class="player-island" in:fade>
              <div class="track-meta" oncontextmenu={showContextMenu} role="button" tabindex="0">
                <div class="meta-texts">
                  <div class="title-row">
                    <h1>{currentPlaying.title}</h1>
                  </div>
                  <p class="artist-name">{currentPlaying?.artist || 'Unknown Artist'}</p>
                </div>
              </div>

              <div class="playback-controls">
                <div class="timeline-container">
                  <div class="modern-slider progress-slider">
                     <input type="range" class="knobless-range" min="0" max="100" value={progress} oninput={handleSeek} />
                     <div class="slider-fill" style="width: {progress}%"></div>
                  </div>
                  <div class="time-row">
                    <span class="time">{formatTime(currentTime)}</span>
                    <span class="time">{formatTime(duration)}</span>
                  </div>
                </div>

                <div class="main-btns">
                  <button class="sub-btn"><Icon name="shuffle" size={22} /></button>
                  <button class="main-btn"><Icon name="skipBack" size={38} /></button>
                  <button class="play-btn" onclick={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()}>
                    <Icon name={audioRef?.paused ? 'play' : 'pause'} size={38} color="black" />
                  </button>
                  <button class="main-btn"><Icon name="skipForward" size={38} /></button>
                  <button class="sub-btn"><Icon name="repeat" size={22} /></button>
                </div>

                <div class="bottom-controls">
                  <div class="volume-slider-modern">
                     <button class="mute-btn" onclick={toggleMute} title={$volume === 0 ? "Activar sonido" : "Silenciar"}>
                        <Icon name={$volume === 0 ? 'volumeX' : ($volume < 0.5 ? 'volume1' : 'volume2')} size={20} />
                     </button>
                     <div class="modern-slider v-slider-large">
                       <input type="range" class="knobless-range" min="0" max="1" step="0.01" value={$volume} oninput={handleVolume} />
                       <div class="slider-fill volume-fill" style="width: {$volume * 100}%"></div>
                     </div>
                  </div>
                </div>
              </div>
            </div>
          {:else if subView === 'queue'}
            <div class="view-panel queue-panel" in:fade>
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
                   <p class="empty-queue">Fin de la cola</p>
                {/if}
              </div>
            </div>
          {:else if subView === 'lyrics'}
            <div class="view-panel lyrics-panel" bind:this={lyricContainer} in:fade>
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
                    <p>Instrumental</p>
                </div>
              {/if}
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
    padding: 1rem 0 3rem 0;
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

  .main-content-flow {
    width: 100%;
    max-width: 500px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.5rem;
    transition: all 0.4s ease;
  }

  .player-header {
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }

  .compact-mode .player-header {
    flex-direction: row;
    align-items: center;
    gap: 1.5rem;
    padding: 0 1rem;
  }

  .compact-mode .cover-wrapper-container {
    max-width: 80px;
  }

  .compact-mode .cover-wrapper {
    box-shadow: 0 10px 20px rgba(0,0,0,0.4);
    border-radius: 12px;
  }

  .compact-meta {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: center;
    text-align: left;
    min-width: 0;
  }

  .compact-meta h2 {
    margin: 0;
    font-size: 1.3rem;
    font-weight: 850;
    letter-spacing: -0.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .compact-meta p {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
    opacity: 0.6;
  }

  .dynamic-area {
    width: 100%;
    flex: 1;
  }

  .cover-wrapper-container {
    width: 100%;
    max-width: 300px;
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    align-items: center;
    z-index: 3;
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
  .cover-wrapper img { width: 100%; height: 100%; object-fit: cover; }

  .player-island {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 2.5rem;
  }

  .track-meta { display: flex; justify-content: center; width: 100%; }
  .meta-texts h1 { 
    font-size: 2.2rem; font-weight: 800; letter-spacing: -1px; margin: 0; text-align: center; line-height: 1.1; 
  }
  .artist-name { font-size: 1.1rem; font-weight: 700; opacity: 0.6; text-align: center; width: 100%; margin: 0.2rem 0 0 0; }


  .timeline-container { width: 100%; display: flex; flex-direction: column; gap: 0.8rem; }
  .time-row { display: flex; justify-content: space-between; font-size: 0.8rem; opacity: 0.6; font-variant-numeric: tabular-nums; }

  .playback-controls { width: 100%; display: flex; flex-direction: column; gap: 2rem; }
  .main-btns { display: flex; justify-content: space-between; align-items: center; }
  .play-btn { width: 72px; height: 72px; background: white; border: none; border-radius: 50%; display: flex; align-items: center; justify-content: center; cursor: pointer; }
  .main-btn { color: white; opacity: 0.8; background: none; border: none; cursor: pointer; }
  .sub-btn { color: white; opacity: 0.4; background: none; border: none; cursor: pointer; }

  .volume-slider-modern { display: flex; align-items: center; gap: 1rem; width: 100%; justify-content: center; }
  .mute-btn { background: none; border: none; color: white; opacity: 0.6; cursor: pointer; padding: 0.5rem; display: flex; align-items: center; transition: all 0.2s; }
  .mute-btn:hover { opacity: 1; transform: scale(1.1); }

  .modern-slider { position: relative; width: 100%; height: 6px; background: rgba(255,255,255,0.1); border-radius: 20px; cursor: pointer; }
  .slider-fill { position: absolute; top: 0; left: 0; height: 100%; background: white; border-radius: 20px; z-index: 1; }
  .volume-fill { background: var(--primary); }
  .knobless-range { position: absolute; inset: 0; width: 100%; opacity: 0; cursor: pointer; z-index: 2; }

  /* Panels */
  .view-panel { width: 100%; display: flex; flex-direction: column; gap: 1.5rem; animation: fadeIn 0.3s ease; }

  .queue-list { display: flex; flex-direction: column; gap: 0.6rem; max-height: 400px; overflow-y: auto; padding-right: 0.5rem; }
  .queue-item { display: flex; align-items: center; gap: 1rem; padding: 0.8rem; border-radius: 16px; background: rgba(255,255,255,0.03); }
  .q-cover { width: 44px; height: 44px; border-radius: 8px; }
  .q-info { display: flex; flex-direction: column; min-width: 0; }
  .q-title { font-weight: 700; font-size: 0.9rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .q-artist { font-size: 0.8rem; opacity: 0.5; }

  .lyrics-list { display: flex; flex-direction: column; gap: 1rem; max-height: 500px; overflow-y: auto; scrollbar-width: none; }
  .lyric-line { font-size: 1.5rem; font-weight: 800; color: rgba(255,255,255,0.2); background: none; border: none; cursor: pointer; text-align: center; }
  .lyric-line.active { color: white; transform: scale(1.05); }

  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }
</style>
>
