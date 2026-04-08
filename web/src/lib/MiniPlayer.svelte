<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { queue, currentIndex, audioQuality, isPlaying } from './stores.js';
  import { get } from 'svelte/store';
  
  let { 
    currentPlaying = $bindable(null), 
    progress = $bindable(0), 
    duration = $bindable(0), 
    currentTime = $bindable(0), 
    audioRef = $bindable(null) 
  } = $props();

  const dispatch = createEventDispatcher();
  let volume = $state(1);
  let isMuted = $state(false);

  // Dual audio elements for gapless
  let audioA = $state(), audioB = $state();
  let activeAudio = $state('A');
  
  let nextTrack = $state(null);
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
        { label: 'Editar etiquetas', icon: 'edit', action: () => dispatch('edit', currentPlaying) },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(currentPlaying.title) },
        { divider: true },
        { label: 'Eliminar de biblioteca', icon: 'trash', color: 'danger', action: () => dispatch('delete', currentPlaying) }
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

  // Preload logic: when 90% done, start loading next
  function checkPreload() {
    if (duration > 0 && currentTime / duration > 0.8) {
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
          console.log("Preloading next track:", next.title);
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
    
    // Smooth transition
    prevAudio.pause();
    audioRef.play().catch(e => console.error("Play failed", e));
    
    // Sync currentPlaying with App (via store or prop)
    const q = get(queue);
    const idx = get(currentIndex);
    currentPlaying = q[idx];
    preloadedId = null; // Reset for next cycle
  }

  // Handle manual play from App
  $effect(() => {
    if (currentPlaying && audioRef && audioRef.src.indexOf(currentPlaying.id) === -1 && !preloadedId) {
      const quality = get(audioQuality);
      const url = `/api/stream/${currentPlaying.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.src = url;
      audioRef.play();
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
      // If already preloaded, switch. Else, load fresh
      if (preloadedId === next.id) {
         switchAudio();
      } else {
         const quality = get(audioQuality);
         audioRef.src = `/api/stream/${next.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
         audioRef.play();
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
      audioRef.play();
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

<footer class="player glass">
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="now-playing" onclick={() => dispatch('openFull')} oncontextmenu={showContextMenu}>
    {#if currentPlaying}
      <img class="cover" src={`/api/cover/${currentPlaying.id}`} alt="Artwork" />
      <div class="track-info">
        <span class="title">{currentPlaying.title}</span>
        <span class="artist">{currentPlaying.artist}</span>
      </div>
    {:else}
      <div class="cover-placeholder"></div>
      <div class="track-info">
        <span class="title">No Track</span>
        <span class="artist">Select a song</span>
      </div>
    {/if}
  </div>
  
  <div class="controls-container">
    <div class="main-controls">
      <button class="icon-btn" onclick={skipBack}><Icon name="skipBack" /></button>
      <button class="icon-btn play-btn" onclick={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()}>
        <Icon name={$isPlaying ? 'pause' : 'play'} size={20} color="black" />
      </button>
      <button class="icon-btn" onclick={skipNext}><Icon name="skipForward" /></button>
    </div>
    
    <div class="progress-area">
      <span class="time">{formatTime(currentTime)}</span>
      <input 
        type="range" 
        class="progress-bar" 
        min="0" max="100" 
        style="--progress: {progress}%"
        bind:value={progress} 
        oninput={handleSeek} 
      />
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
    on:close={() => contextMenu.visible = false}
  />

  <!-- Dual hidden audio elements managed by parent or here -->
  <audio bind:this={audioA} style="display:none"></audio>
  <audio bind:this={audioB} style="display:none"></audio>
</footer>

<style>
  .player {
    grid-area: player;
    background: var(--surface-container);
    display: grid;
    grid-template-columns: minmax(180px, 0.8fr) 1.5fr minmax(180px, 0.8fr);
    align-items: center;
    padding: 0 1.5rem;
    margin: 0.5rem 1rem 1rem 1rem;
    border-radius: 28px;
    border: 1px solid var(--border-color);
    box-shadow: 0 8px 32px rgba(0,0,0,0.12);
  }

  .now-playing {
    display: flex;
    align-items: center;
    gap: 1rem;
    cursor: pointer;
  }

  .cover, .cover-placeholder {
    width: 56px;
    height: 56px;
    border-radius: 8px;
    background: #1e293b;
    object-fit: cover;
  }

  .track-info {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .track-info .title { 
    font-weight: 700; 
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 1.05rem;
  }
  .track-info .artist { font-size: 0.85rem; color: var(--text-muted); font-weight: 500; }

  .controls-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.3rem;
  }

  .main-controls {
    display: flex;
    gap: 1.5rem;
    align-items: center;
  }

  .play-btn {
    background: var(--primary) !important;
    color: white !important;
    width: 48px;
    height: 48px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.2rem;
    box-shadow: 0 4px 12px var(--primary-10);
    transition: transform 0.2s;
  }
  .play-btn:hover { transform: scale(1.05); }

  .progress-area {
    width: 100%;
    max-width: 600px;
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .time { font-size: 0.75rem; color: #64748b; min-width: 35px; }

  .progress-bar {
    flex: 1;
    height: 4px;
    appearance: none;
    background: var(--surface-container-highest);
    border-radius: 10px;
    cursor: pointer;
    accent-color: var(--primary);
  }

  .progress-bar::-webkit-slider-runnable-track {
    background: linear-gradient(to right, var(--primary) var(--progress, 0%), var(--surface-container-highest) var(--progress, 0%));
    height: 4px;
    border-radius: 10px;
  }

  .progress-bar::-webkit-slider-thumb {
    appearance: none;
    width: 12px;
    height: 12px;
    background: var(--text-main);
    border-radius: 50%;
    margin-top: -4px;
    box-shadow: 0 0 10px rgba(0,0,0,0.5);
  }

  .volume-container {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 1rem;
  }

  .volume-bar {
    width: 100px;
    height: 4px;
    accent-color: var(--primary);
  }

  .icon-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1.2rem;
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .icon-btn:hover { color: var(--text-main); transform: scale(1.1); }

  .tech-badge {
    background: var(--primary-10);
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.7rem;
    font-weight: 800;
    color: var(--primary);
    border: 1px solid var(--primary-20);
  }
  @media (max-width: 1400px) {
    .player { grid-template-columns: 200px 1fr 200px; padding: 0 1rem; }
    .main-controls { gap: 1rem; }
  }

  @media (max-width: 1100px) {
    .player { 
      grid-template-columns: 180px 1fr auto; 
      height: 80px; 
      padding: 0 1rem;
      margin-left: 0.5rem;
      border-radius: 28px;
    }
    .volume-bar, .tech-badge { display: none; }
    .now-playing { gap: 0.8rem; }
    .cover { width: 44px; height: 44px; }
    .track-info .title { font-size: 0.95rem; }
    .track-info .artist { font-size: 0.75rem; }
  }

  @media (max-width: 768px) {
    .player {
      grid-template-columns: 1fr auto;
      height: 75px;
      margin: 0.5rem 0.5rem 0 0.5rem;
      padding: 0 1rem;
      border-radius: 24px 24px 0 0;
      gap: 1rem;
      border-bottom: 1px solid var(--border-color);
      box-shadow: 0 -10px 30px rgba(0,0,0,0.1);
    }
    .progress-area, .volume-container, .tech-badge { display: none !important; }
    .now-playing { min-width: 0; }
    .cover { width: 44px; height: 44px; border-radius: 12px; }
    .track-info .title { font-size: 0.9rem; }
    .track-info .artist { font-size: 0.75rem; }
    .main-controls { gap: 0.5rem; }
    .play-btn { width: 42px; height: 42px; }
    .icon-btn :global(svg) { width: 20px; height: 20px; }
  }
</style>
