<script>
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { queue, currentIndex, audioQuality, effectiveQuality, isPlaying, showToast } from './stores.js';
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
  let showVolume = $state(false);

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
          const quality = get(effectiveQuality);
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
      const quality = get(effectiveQuality);
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
         const quality = get(effectiveQuality);
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
      const quality = get(effectiveQuality);
      audioRef.src = `/api/stream/${currentPlaying.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.play().catch(() => {});
      preloadedId = null;
    }
  }

  async function handleRate(track, rating) {
    try {
      const res = await fetch(`/api/tracks/${track.id}/rate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rating }),
      });
      if (res.ok) {
        track.rating = rating;
        showToast(`Valorado con ${rating} estrellas`, "success");
      }
    } catch (e) {
      console.error(e);
      showToast("Error al valorar", "error");
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
  <div class="progress-line" style="width: {progress}%"></div>
  <div class="progress-bg" style="width: {progress}%"></div>
  
  <div class="now-playing" onclick={onopenFull} oncontextmenu={showContextMenu} role="button" tabindex="0" onkeydown={(e) => e.key === 'Enter' && onopenFull?.()}>
    <div class="cover-container">
      {#if currentPlaying?.id}
        <img src="/api/cover/{currentPlaying.id}" alt="Cover" class="cover" />
      {:else}
        <div class="cover-placeholder"></div>
      {/if}
    </div>
    <div class="info">
      <div class="album-name">{currentPlaying?.album || 'Sgound Player'}</div>
      <div class="track-meta">
        <span class="track-title">{currentPlaying?.title || 'No track playing'}</span>
        {#if currentPlaying?.format}
          <span class="quality-chip">{currentPlaying.format.toUpperCase()}</span>
        {/if}
      </div>
    </div>
  </div>

  <div class="right-actions">
    <div class="playback-btns">
      <button class="icon-btn" onclick={skipBack} title="Previous">
        <Icon name="skipBack" size={22} />
      </button>
      <button class="main-play-btn" onclick={() => audioRef?.paused ? audioRef.play().catch(() => {}) : audioRef?.pause()} title={$isPlaying ? 'Pause' : 'Play'}>
        <Icon name={$isPlaying ? 'pause' : 'play'} size={24} />
      </button>
      <button class="icon-btn" onclick={skipNext} title="Next">
        <Icon name="skipForward" size={22} />
      </button>
    </div>

    <div class="extra-controls">
      <div class="volume-container">
        <button class="icon-btn" onclick={() => showVolume = !showVolume} title="Volume">
          <Icon name={isMuted ? 'volumeX' : 'volume2'} size={20} />
        </button>
        {#if showVolume}
          <div class="volume-popover" transition:fade={{duration: 150}}>
            <div class="vol-slider-wrap">
              <input 
                type="range" 
                class="vol-slider-vertical" 
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
              <div class="v-fill" style="height: {volume * 100}%"></div>
            </div>
          </div>
        {/if}
      </div>
      <button class="icon-btn expand-btn" onclick={onopenFull} title="Expand">
        <Icon name="maximize" size={20} />
      </button>
    </div>
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
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 2rem;
    height: 100px;
    margin: 0; 
    border-radius: inherit;
    position: relative;
    overflow: hidden;
    z-index: 1000;
  }

  .progress-bg {
    position: absolute;
    left: 0; top: 0; bottom: 0;
    background: linear-gradient(to bottom, rgba(var(--primary-rgb), 0.1), rgba(var(--primary-rgb), 0.25));
    z-index: -1;
    transition: width 0.3s linear;
  }

  .progress-line {
    position: absolute;
    top: 0; left: 0;
    height: 3px;
    background: var(--primary);
    z-index: 10;
    box-shadow: 0 0 10px var(--primary-40);
    transition: width 0.3s linear;
  }

  .now-playing {
    display: flex;
    align-items: center;
    gap: 1.2rem;
    min-width: 0; /* Essential for text trimming */
    flex: 1;
    cursor: pointer;
  }

  .cover {
    width: 60px;
    height: 60px;
    border-radius: 12px;
    object-fit: cover;
    box-shadow: 0 4px 12px rgba(0,0,0,0.2);
    flex-shrink: 0;
  }

  .info {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
  }

  .album-name {
    font-size: 0.72rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    opacity: 0.85;
  }

  .track-meta {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    min-width: 0;
  }

  .track-title {
    font-size: 1.2rem;
    font-weight: 700;
    color: white;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .quality-chip {
    font-size: 0.6rem;
    font-weight: 900;
    padding: 0.1rem 0.45rem;
    background: var(--primary);
    border-radius: 4px;
    color: white;
    letter-spacing: 0.05em;
    flex-shrink: 0;
    box-shadow: 0 2px 4px rgba(0,0,0,0.2);
  }

  .right-actions {
    display: flex;
    align-items: center;
    gap: 2rem;
  }

  .playback-btns {
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }

  .main-play-btn {
    background: white;
    color: black;
    border: none;
    width: 52px;
    height: 52px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 4px 15px rgba(0,0,0,0.4);
  }
  .main-play-btn:hover { transform: scale(1.08); }

  .extra-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .volume-container {
    position: relative;
  }

  .volume-popover {
    position: absolute;
    bottom: calc(100% + 1.8rem);
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-container-highest);
    padding: 1.4rem 0.6rem;
    border-radius: 20px;
    border: 1px solid var(--border-color);
    box-shadow: 0 10px 40px rgba(0,0,0,0.5);
    z-index: 2000;
  }

  .vol-slider-wrap {
    height: 120px;
    width: 6px;
    background: rgba(255,255,255,0.08);
    border-radius: 10px;
    position: relative;
    overflow: hidden;
  }

  .vol-slider-vertical {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    appearance: none;
    background: transparent;
    cursor: pointer;
    z-index: 5;
    writing-mode: vertical-lr;
    direction: rtl;
  }

  .v-fill {
    position: absolute;
    bottom: 0; left: 0; right: 0;
    background: var(--primary);
    border-radius: 10px;
    z-index: 4;
  }

  .icon-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 10px;
    border-radius: 50%;
    transition: all 0.25s ease;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .icon-btn:hover { 
    color: white; 
    background: rgba(255,255,255,0.08);
  }

  @media (max-width: 768px) {
    .player { 
      margin: 0; 
      padding: 0 1rem; 
      height: 72px; 
      border-radius: inherit;
      border-bottom: none;
    }
    .album-name, .progress-bg { display: none; }
    .right-actions { gap: 0.6rem; }
    .playback-btns { gap: 0.4rem; }
    .main-play-btn { width: 38px; height: 38px; box-shadow: 0 2px 8px rgba(0,0,0,0.3); }
    .cover { width: 44px; height: 44px; border-radius: 8px; }
    .track-title { font-size: 0.95rem; }
    .expand-btn { display: none; }
    .icon-btn { padding: 6px; }
  }
</style>
