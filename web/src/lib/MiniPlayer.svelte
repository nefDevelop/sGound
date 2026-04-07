<script>
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  
  export let currentPlaying = null;
  export let audioRef = null;
  export let progress = 0;
  export let duration = 0;
  export let currentTime = 0;

  const dispatch = createEventDispatcher();
  let volume = 1;
  let isMuted = false;

  $: if (audioRef) {
    audioRef.ontimeupdate = () => {
      currentTime = audioRef.currentTime;
      duration = audioRef.duration;
      progress = (currentTime / duration) * 100;
    };
  }

  function handleSeek(e) {
    const seekTime = (e.target.value / 100) * duration;
    audioRef.currentTime = seekTime;
  }

  function toggleMute() {
    isMuted = !isMuted;
    audioRef.muted = isMuted;
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
  <div class="now-playing" on:click={() => dispatch('openFull')}>
    {#if currentPlaying}
      <img class="cover" src={`/api/cover?id=${encodeURIComponent(currentPlaying.id)}`} alt="Artwork" />
    {:else}
      <div class="cover-placeholder"></div>
    {/if}
    <div class="track-info">
      <span class="title">{currentPlaying ? currentPlaying.title : 'No Track'}</span>
      <span class="artist">{currentPlaying ? currentPlaying.artist : 'Select a song'}</span>
    </div>
  </div>
  
  <div class="controls-container">
    <div class="main-controls">
      <button class="icon-btn"><Icon name="skipBack" /></button>
      <button class="icon-btn play-btn" on:click={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()}>
        <Icon name={audioRef?.paused ? 'play' : 'pause'} size={20} color="black" />
      </button>
      <button class="icon-btn"><Icon name="skipForward" /></button>
    </div>
    
    <div class="progress-area">
      <span class="time">{formatTime(currentTime)}</span>
      <input 
        type="range" 
        class="progress-bar" 
        min="0" max="100" 
        bind:value={progress} 
        on:input={handleSeek} 
      />
      <span class="time">{formatTime(duration)}</span>
    </div>
  </div>
  
  <div class="volume-container">
    <button class="icon-btn" on:click={toggleMute}>
      <Icon name={isMuted ? 'volumeX' : 'volume2'} size={20} />
    </button>
    <input 
      type="range" 
      class="volume-bar" 
      min="0" max="1" step="0.1" 
      bind:value={volume} 
      on:input={() => audioRef.volume = volume}
    />
    {#if currentPlaying}
      <div class="tech-badge">{currentPlaying.format?.toUpperCase()}</div>
    {/if}
  </div>

  <!-- Hidden audio element managed by parent or here -->
  <audio bind:this={audioRef} style="display:none"></audio>
</footer>

<style>
  .player {
    grid-area: player;
    background: #0f111a;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
    display: grid;
    grid-template-columns: 300px 1fr 300px;
    align-items: center;
    padding: 0 2rem;
    height: 90px;
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
    font-weight: 600; 
    color: white;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .track-info .artist { font-size: 0.85rem; color: #94a3b8; }

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
    background: white !important;
    color: black !important;
    width: 36px;
    height: 36px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.2rem;
  }

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
    background: rgba(255,255,255,0.1);
    border-radius: 2px;
    cursor: pointer;
  }

  .progress-bar::-webkit-slider-thumb {
    appearance: none;
    width: 12px;
    height: 12px;
    background: white;
    border-radius: 50%;
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
  }

  .icon-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    font-size: 1.2rem;
    cursor: pointer;
    transition: color 0.2s;
  }

  .icon-btn:hover { color: white; }

  .tech-badge {
    background: rgba(255,255,255,0.05);
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.7rem;
    font-weight: 700;
    color: #64748b;
  }
</style>
