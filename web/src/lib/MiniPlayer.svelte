<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { queue, currentIndex, audioQuality, isPlaying } from './stores.js';
  import { get } from 'svelte/store';
  
  let { 
    currentPlaying = $bindable(null), 
    progress = $bindable(0), 
    duration = $bindable(0), 
    currentTime = $bindable(0), 
    audioRef = $bindable(null),
    onopenFull,
    onedit,
    ondelete
  } = $props();

  let volume = $state(1);
  let isMuted = $state(false);

  // Dual audio elements for gapless
  let audioA = $state(), audioB = $state();
  let activeAudio = $state('A');
  
  let preloadedId = $state(null);

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
        { label: 'Edit Tags', icon: 'edit', action: () => onedit?.(currentPlaying) },
        { label: 'Copy Title', icon: 'fileText', action: () => navigator.clipboard.writeText(currentPlaying.title) },
        { divider: true },
        { label: 'Remove from Library', icon: 'trash', color: 'danger', action: () => ondelete?.(currentPlaying) }
      ]
    };
  }

  onMount(() => {
    audioRef = audioA;
    setupAudio(audioA);
    setupAudio(audioB);
  });

  function setupAudio(el) {
    el.onended = handleTrackEnded;
    el.ontimeupdate = () => {
      if (el === audioRef) {
        currentTime = el.currentTime;
        duration = el.duration;
        progress = (currentTime / duration) * 100;
        checkPreload();
      }
    };
    el.onplay = () => isPlaying.set(true);
    el.onpause = () => isPlaying.set(false);
  }

  function checkPreload() {
    if (duration > 0 && currentTime / duration > 0.85) {
      const q = get(queue);
      const idx = get(currentIndex);
      if (idx < q.length - 1) {
        const next = q[idx + 1];
        if (next && next.id !== preloadedId) {
          const nextEl = activeAudio === 'A' ? audioB : audioA;
          const quality = get(audioQuality);
          const url = `/api/stream/${next.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
          nextEl.src = url;
          nextEl.load();
          preloadedId = next.id;
        }
      }
    }
  }

  function handleTrackEnded() {
     const q = get(queue);
     const idx = get(currentIndex);
     if (idx < q.length - 1) {
        currentIndex.set(idx + 1);
        switchAudio();
     } else {
        isPlaying.set(false);
     }
  }

  function switchAudio() {
    const prevAudio = activeAudio === 'A' ? audioA : audioB;
    activeAudio = activeAudio === 'A' ? 'B' : 'A';
    audioRef = activeAudio === 'A' ? audioA : audioB;
    
    prevAudio.pause();
    audioRef.play().catch(e => console.error("Play failed", e));
    
    const q = get(queue);
    const idx = get(currentIndex);
    currentPlaying = q[idx];
    preloadedId = null;
  }

  $effect(() => {
    if (currentPlaying && audioRef && audioRef.src.indexOf(currentPlaying.id) === -1 && !preloadedId) {
      const quality = get(audioQuality);
      const url = `/api/stream/${currentPlaying.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.src = url;
      audioRef.play().catch(() => {});
    }
  });

  function handleSeek(e) {
    const seekTime = (e.target.value / 100) * duration;
    audioRef.currentTime = seekTime;
  }

  function toggleMute() {
    isMuted = !isMuted;
    audioA.muted = isMuted;
    audioB.muted = isMuted;
  }

  function skipNext() {
    const q = get(queue);
    const idx = get(currentIndex);
    if (idx < q.length - 1) {
      currentIndex.set(idx + 1);
      const next = q[idx + 1];
      currentPlaying = next;
      if (preloadedId === next.id) {
         switchAudio();
      } else {
         const quality = get(audioQuality);
         audioRef.src = `/api/stream/${next.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
         audioRef.play().catch(() => {});
         preloadedId = null;
      }
    }
  }

  function skipBack() {
    const idx = get(currentIndex);
    if (idx > 0) {
      currentIndex.set(idx - 1);
      const q = get(queue);
      currentPlaying = q[idx-1];
      const quality = get(audioQuality);
      audioRef.src = `/api/stream/${currentPlaying.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.play().catch(() => {});
      preloadedId = null;
    }
  }

  function formatTime(secs) {
    if (isNaN(secs)) return "0:00";
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${mins}:${s.toString().padStart(2, '0')}`;
  }
</script>

<footer class="player">
  <div class="now-playing" onclick={onopenFull} oncontextmenu={showContextMenu} role="button" tabindex="0" onkeydown={(e) => e.key === 'Enter' && onopenFull?.()}>
    {#if currentPlaying}
      <img class="cover" src={`/api/cover/${currentPlaying.id}`} alt="Artwork" />
      <div class="track-info">
        <span class="title">{currentPlaying.title}</span>
        <span class="artist">{currentPlaying.artist}</span>
      </div>
    {:else}
      <div class="cover-placeholder"></div>
      <div class="track-info">
        <span class="title">No Active Track</span>
        <span class="artist">Select something to play</span>
      </div>
    {/if}
  </div>
  
  <div class="controls-container">
    <div class="main-controls">
      <button class="icon-btn" onclick={skipBack} title="Previous"><Icon name="skipBack" size={28} /></button>
      <button class="play-btn" onclick={() => audioRef?.paused ? audioRef.play().catch(() => {}) : audioRef?.pause()} title="Play/Pause">
        <Icon name={$isPlaying ? 'pause' : 'play'} size={28} />
      </button>
      <button class="icon-btn" onclick={skipNext} title="Next"><Icon name="skipForward" size={28} /></button>
    </div>
    
    <div class="progress-area">
      <span class="time">{formatTime(currentTime)}</span>
      <div class="modern-slider-container">
        <input 
          type="range" 
          class="progress-bar" 
          min="0" max="100" 
          style="--progress: {progress}%"
          bind:value={progress} 
          oninput={handleSeek} 
        />
        <div class="progress-fill" style="width: {progress}%"></div>
      </div>
      <span class="time">{formatTime(duration)}</span>
    </div>
  </div>
  
  <div class="volume-container">
    <button class="icon-btn" onclick={toggleMute}>
      <Icon name={isMuted ? 'volumeX' : 'volume2'} size={20} />
    </button>
    <input 
      type="range" 
      class="volume-bar" 
      min="0" max="1" step="0.01" 
      bind:value={volume} 
      oninput={() => {
        if (!audioRef) return;
        const gain = currentPlaying?.replay_gain || 0;
        const linearGain = Math.pow(10, gain / 20);
        audioA.volume = Math.min(1.0, volume * linearGain);
        audioB.volume = Math.min(1.0, volume * linearGain);
      }}
    />
    {#if currentPlaying && currentPlaying.format}
      <div class="tech-badge">{currentPlaying.format.toUpperCase()}</div>
    {/if}
  </div>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    onclose={() => contextMenu.visible = false}
  />

  <audio bind:this={audioA} style="display:none"></audio>
  <audio bind:this={audioB} style="display:none"></audio>
</footer>

<style>
  .player {
    grid-area: player;
    background: var(--surface-container);
    display: grid;
    grid-template-columns: minmax(220px, 1fr) 2fr minmax(220px, 1fr);
    align-items: center;
    padding: 0 2.5rem;
    height: 110px;
    margin: 0.5rem 1.5rem 1.5rem 1.5rem;
    border-radius: 40px;
    border: 1px solid var(--border-color);
    box-shadow: 0 12px 40px rgba(0,0,0,0.2);
    backdrop-filter: blur(20px);
    z-index: 1000;
  }

  .now-playing {
    display: flex;
    align-items: center;
    gap: 1.2rem;
    cursor: pointer;
    min-width: 0;
    transition: transform 0.2s;
  }
  .now-playing:hover { transform: translateX(4px); }

  .cover, .cover-placeholder {
    width: 68px;
    height: 68px;
    border-radius: 16px;
    background: var(--surface-container-high);
    object-fit: cover;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  }

  .track-info {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    gap: 2px;
  }

  .track-info .title { 
    font-weight: 850; 
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 1.1rem;
    letter-spacing: -0.3px;
  }
  .track-info .artist { font-size: 0.9rem; color: var(--text-muted); font-weight: 600; opacity: 0.8; }

  .controls-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.8rem;
    width: 100%;
  }

  .main-controls {
    display: flex;
    gap: 2rem;
    align-items: center;
  }

  .play-btn {
    background: var(--primary) !important;
    color: white !important;
    width: 64px;
    height: 64px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 8px 24px var(--primary-30);
    transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
    border: none;
    cursor: pointer;
  }
  .play-btn:hover { transform: scale(1.1); box-shadow: 0 12px 32px var(--primary-40); }
  .play-btn:active { transform: scale(0.95); }

  .progress-area {
    width: 100%;
    max-width: 700px;
    display: flex;
    align-items: center;
    gap: 1.2rem;
  }

  .modern-slider-container {
    position: relative;
    flex: 1;
    height: 6px;
    display: flex;
    align-items: center;
  }

  .progress-bar {
    position: absolute;
    width: 100%;
    height: 100%;
    appearance: none;
    background: var(--surface-container-highest);
    border-radius: 100px;
    cursor: pointer;
    z-index: 2;
    opacity: 0;
  }

  .progress-fill {
    position: absolute;
    height: 100%;
    background: var(--primary);
    border-radius: 100px;
    z-index: 1;
    pointer-events: none;
  }

  .modern-slider-container::after {
    content: '';
    position: absolute;
    width: 100%;
    height: 100%;
    background: var(--surface-container-highest);
    border-radius: 100px;
    z-index: 0;
  }

  .progress-bar::-webkit-slider-thumb {
    appearance: none;
    width: 16px;
    height: 16px;
    background: white;
    border-radius: 50%;
    box-shadow: 0 0 10px rgba(0,0,0,0.5);
  }

  .time { font-size: 0.8rem; color: var(--text-muted); font-weight: 700; font-variant-numeric: tabular-nums; width: 45px; }

  .volume-container {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 1.2rem;
  }

  .volume-bar {
    width: 100px;
    height: 5px;
    appearance: none;
    background: var(--surface-container-highest);
    border-radius: 10px;
    cursor: pointer;
    accent-color: var(--primary);
  }

  .icon-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 8px;
    border-radius: 12px;
  }
  .icon-btn:hover { color: var(--text-main); background: var(--surface-container-high); transform: scale(1.1); }

  .tech-badge {
    background: var(--primary-10);
    padding: 0.3rem 0.6rem;
    border-radius: 8px;
    font-size: 0.7rem;
    font-weight: 800;
    color: var(--primary);
    border: 1px solid var(--primary-20);
    letter-spacing: 0.5px;
  }

  @media (max-width: 1400px) {
    .player { grid-template-columns: 240px 1fr 240px; padding: 0 1.5rem; }
  }

  @media (max-width: 1100px) {
    .player { 
      grid-template-columns: 1fr 1.5fr auto; 
      height: 115px; 
      padding: 0 1.5rem; 
      gap: 1rem;
    }
    .volume-bar, .tech-badge { display: none; }
    .play-btn { width: 56px; height: 56px; }
    .controls-container { gap: 0.4rem; }
    .cover { width: 60px; height: 60px; }
  }

  @media (max-width: 768px) {
    .player {
      grid-template-columns: 1fr auto;
      height: 80px;
      margin: 0.5rem 1rem 0 1rem;
      border-radius: 32px 32px 0 0;
      border-bottom: none;
      padding: 0 1.5rem;
      gap: 1.5rem;
    }
    .progress-area, .volume-container, .tech-badge { display: none !important; }
    .play-btn { width: 52px; height: 52px; }
    .cover { width: 48px; height: 48px; }
    .track-info .title { font-size: 1rem; }
    .main-controls { gap: 1rem; }
  }
</style>
