<script>
  import Icon from '../Icon.svelte';
  import { isShuffle, repeatMode } from '../stores.js';

  let {
    audioRef,
    isPaused,
    onskipBack,
    onskipNext,
    onPlayToggle
  } = $props();

  function toggleRepeat() {
    const modes = ['none', 'all', 'one'];
    const current = $repeatMode;
    const next = modes[(modes.indexOf(current) + 1) % modes.length];
    repeatMode.set(next);
  }
</script>

<div class="main-btns">
  <button class="sub-btn" class:active={$isShuffle} onclick={() => isShuffle.update(s => !s)} title="Aleatorio">
    <Icon name="shuffle" size={24} />
  </button>
  <button class="main-btn" onclick={onskipBack} title="Anterior">
    <Icon name="skipBack" size={42} />
  </button>
  <button class="play-btn-prominent" onclick={onPlayToggle} title={isPaused ? "Reproducir" : "Pausar"}>
    <Icon name={isPaused ? 'play' : 'pause'} size={42} color="var(--surface)" />
  </button>
  <button class="main-btn" onclick={onskipNext} title="Siguiente">
    <Icon name="skipForward" size={42} />
  </button>
  <button class="sub-btn" class:active={$repeatMode !== 'none'} onclick={toggleRepeat} title="Repetición">
    <Icon name="repeat" size={24} />
    {#if $repeatMode === 'one'}<span class="repeat-one-badge">1</span>{/if}
  </button>
</div>

<style>
  .main-btns {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
  }

  .main-btn {
    color: white;
    opacity: 0.6;
    background: none;
    border: none;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .main-btn:hover {
    opacity: 1;
    transform: scale(1.1);
  }
  .main-btn:active {
    transform: scale(0.9);
  }

  .sub-btn {
    color: white;
    opacity: 0.3;
    background: none;
    border: none;
    cursor: pointer;
    transition: all 0.2s;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .sub-btn:hover {
    opacity: 0.6;
  }
  .sub-btn.active {
    opacity: 1;
    color: var(--primary);
  }

  .play-btn-prominent {
    width: 84px;
    height: 84px;
    background: white;
    border: none;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    box-shadow: 0 10px 30px rgba(255,255,255,0.2);
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }
  .play-btn-prominent:hover {
    box-shadow: 0 15px 40px rgba(255,255,255,0.3);
    transform: scale(1.08);
    filter: brightness(1.1);
  }
  .play-btn-prominent:active {
    transform: scale(0.92);
    filter: brightness(0.9);
    transition: all 0.1s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .repeat-one-badge {
    position: absolute;
    top: -5px;
    right: -8px;
    font-size: 10px;
    font-weight: 900;
    background: var(--primary);
    color: white;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* Responsive sizes managed by parent context or global overrides where needed, 
     but defaults are set here. */
</style>