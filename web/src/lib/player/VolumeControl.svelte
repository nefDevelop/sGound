<script>
  import Icon from '../Icon.svelte';
  import { volume } from '../stores.js';

  let {
    audioRef
  } = $props();

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

  function handleVolume(e) {
    if (!audioRef) return;
    const v = parseFloat(e.target.value);
    volume.set(v);
    audioRef.volume = v;
  }
</script>

<div class="bottom-controls">
  <div class="volume-slider-modern">
      <button class="mute-btn" onclick={toggleMute} title={$volume === 0 ? "Activar sonido" : "Silenciar"}>
        <Icon name={$volume === 0 ? 'volumeX' : ($volume < 0.5 ? 'volume1' : 'volume2')} size={22} />
      </button>
      <div class="modern-slider v-slider-large">
        <input type="range" class="knobless-range" min="0" max="1" step="0.01" value={$volume} oninput={handleVolume} />
        <div class="slider-fill volume-fill" style="width: {$volume * 100}%"></div>
      </div>
  </div>
</div>

<style>
  .bottom-controls {
      width: 100%;
  }

  .volume-slider-modern {
    display: flex;
    align-items: center;
    gap: 1rem;
    width: 100%;
    justify-content: center;
  }

  .mute-btn {
    background: none;
    border: none;
    color: white;
    opacity: 0.4;
    cursor: pointer;
    padding: 0.5rem;
    display: flex;
    align-items: center;
    transition: all 0.2s;
  }
  .mute-btn:hover {
    opacity: 1;
    transform: scale(1.1);
  }

  .modern-slider {
    position: relative;
    width: 100%;
    height: 6px;
    background: rgba(255,255,255,0.1);
    border-radius: 20px;
    cursor: pointer;
    transition: height 0.2s;
  }
  .modern-slider:hover {
    height: 8px;
  }

  .slider-fill {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background: white;
    border-radius: 20px;
    z-index: 1;
    transition: background 0.3s;
  }
  .volume-fill {
    background: var(--primary);
  }

  .knobless-range {
    position: absolute;
    top: -10px;
    bottom: -10px;
    left: 0;
    width: 100%;
    opacity: 0;
    cursor: pointer;
    z-index: 100;
    margin: 0;
  }
</style>