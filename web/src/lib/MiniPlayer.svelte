<script>
  import { fade } from "svelte/transition";
  import Icon from "./Icon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import {
    isPlaying,
    showToast,
    volume,
    isShuffle,
    repeatMode,
    coverRefreshTrigger,
    showContextMenu as triggerContextMenu,
  } from "./stores.js";
  import { copyToClipboard } from "./utils.js";
  import Artwork from "./Artwork.svelte";
  import { getContext } from "svelte";

  let {
    currentPlaying = $bindable(null),
    progress = $bindable(0),
    duration = $bindable(0),
    currentTime = $bindable(0),
    audioRef = $bindable(null),
    onopenFull,
    onedit,
    onhide,
    ondeletePhysical,
    ontoggleQueue,
  } = $props();

  const { handlers } = getContext("app");

  let showVolume = $state(false);

  function showContextMenu(e) {
    if (!currentPlaying) return;
    e.preventDefault();
    triggerContextMenu(e.clientX, e.clientY, [
      {
        label: "Edit Tags",
        icon: "edit",
        action: () => onedit?.(currentPlaying),
      },
      {
        label: "Copy Title",
        icon: "fileText",
        action: () =>
          copyToClipboard(currentPlaying.title).then(
            (ok) => ok && showToast("Title copied", "info"),
          ),
      },
      { divider: true },
      {
        label: "Ocultar de la biblioteca",
        icon: "eyeOff",
        action: () => onhide?.(currentPlaying),
      },
      {
        label: "Borrar físicamente (Disco)",
        icon: "trash",
        color: "danger",
        action: () => ondeletePhysical?.(currentPlaying),
      },
    ]);
  }

  function toggleMute() {
    if ($volume > 0) {
      localStorage.setItem("lastVolume", $volume.toString());
      volume.set(0);
    } else {
      const lastV = parseFloat(localStorage.getItem("lastVolume")) || 0.5;
      volume.set(lastV);
    }
  }

  function handleVolume(e) {
    const v = parseFloat(e.target.value);
    volume.set(v);
    if (audioRef) audioRef.volume = v;
  }

  function toggleRepeat() {
    const modes = ["none", "all", "one"];
    const current = $repeatMode;
    const next = modes[(modes.indexOf(current) + 1) % modes.length];
    repeatMode.set(next);
  }

  function formatTime(secs) {
    if (isNaN(secs)) return "0:00";
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${mins}:${s.toString().padStart(2, "0")}`;
  }
</script>

<footer class="player-wrapper">
  <div
    class="player"
    onclick={onopenFull}
    role="button"
    tabindex="0"
    onkeydown={(e) => e.key === "Enter" && onopenFull?.()}
  >
    <div class="player-bg-clip">
      <div class="progress-bg" style="width: {progress}%"></div>
    </div>

    <div
      class="now-playing"
      oncontextmenu={showContextMenu}
      role="button"
      tabindex="0"
    >
      <div class="cover-container">
        {#if currentPlaying?.id}
          <Artwork
            src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}"
            alt="Cover"
            size={window.innerWidth < 400 ? "44px" : "54px"}
            borderRadius="12px"
          />
        {:else}
          <div class="cover-placeholder"></div>
        {/if}
      </div>
      <div class="info">
        <div class="album-name">{currentPlaying?.album || "Sgound Player"}</div>
        <div class="track-meta">
          <span class="track-title"
            >{currentPlaying?.title || "No track playing"}</span
          >
          {#if currentPlaying?.format}
            <span class="quality-chip"
              >{currentPlaying.format.toUpperCase()}</span
            >
          {/if}
        </div>
      </div>
    </div>

    <div
      class="right-actions"
      onclick={(e) => e.stopPropagation()}
      role="presentation"
    >
      <div class="playback-btns">
        <button
          class="sub-btn"
          class:active={$isShuffle}
          onclick={() => isShuffle.update((v) => !v)}
          title="Aleatorio"
        >
          <Icon name="shuffle" size={18} />
        </button>
        <button
          class="icon-btn"
          onclick={() => handlers.handleSkipBack()}
          title="Previous"
        >
          <Icon name="skipBack" size={22} />
        </button>
        <button
          class="main-play-btn"
          onclick={() =>
            audioRef?.paused
              ? audioRef.play().catch(() => {})
              : audioRef?.pause()}
          title={$isPlaying ? "Pause" : "Play"}
        >
          <Icon name={$isPlaying ? "pause" : "play"} size={24} />
        </button>
        <button
          class="icon-btn"
          onclick={() => handlers.handleSkipNext()}
          title="Next"
        >
          <Icon name="skipForward" size={22} />
        </button>
        <button
          class="sub-btn"
          class:repeat-one={$repeatMode === "one"}
          class:active={$repeatMode !== "none"}
          onclick={toggleRepeat}
          title="Repetir"
        >
          <Icon name="repeat" size={18} />
          {#if $repeatMode === "one"}<span class="repeat-badge">1</span>{/if}
        </button>
      </div>

      <div class="extra-controls">
        <button
          class="icon-btn"
          class:active={false}
          onclick={ontoggleQueue}
          title="Mostrar cola"
        >
          <Icon name="list" size={20} />
        </button>
        <div class="volume-container">
          <button
            class="icon-btn"
            onclick={() => (showVolume = !showVolume)}
            title="Volume"
          >
            <Icon name={$volume === 0 ? "volumeX" : "volume2"} size={20} />
          </button>
          {#if showVolume}
            <div class="volume-popover" transition:fade={{ duration: 150 }}>
              <div class="vol-slider-wrap">
                <input
                  type="range"
                  class="vol-slider-vertical"
                  min="0"
                  max="1"
                  step="0.01"
                  value={$volume}
                  oninput={handleVolume}
                />
                <div class="v-fill" style="height: {$volume * 100}%"></div>
              </div>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>
</footer>

<style>
  .player-wrapper {
    margin: 0;
    padding: 0;
  }
  .player {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 2rem;
    height: 100px;
    margin: 0;
    border-radius: 28px;
    position: relative;
    overflow: visible;
    cursor: pointer;
  }

  .player-bg-clip {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    overflow: hidden;
    z-index: -1;
    pointer-events: none;
  }

  .progress-bg {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    background: rgba(var(--primary-rgb), 0.2);
    transition: width 0.3s linear;
  }

  .now-playing {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex: 1;
    min-width: 0;
    width: 0;
    overflow: hidden;
  }

  .info {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    flex: 1;
    min-width: 0;
    width: 0;
    overflow: hidden;
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
    gap: 0.5rem;
    min-width: 0;
    width: 100%;
    overflow: hidden;
  }

  .track-title {
    font-size: 1.1rem;
    font-weight: 700;
    color: white;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 0 1 auto;
    min-width: 0;
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
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  }

  .right-actions {
    display: flex;
    align-items: center;
    gap: 2rem;
    flex-shrink: 0;
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
    box-shadow: 0 4px 15px rgba(0, 0, 0, 0.4);
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }
  .main-play-btn:hover {
    transform: scale(1.1);
    filter: brightness(1.1);
  }
  .main-play-btn:active {
    transform: scale(0.92);
    filter: brightness(0.9);
    transition: all 0.1s;
  }

  .icon-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 10px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }
  .icon-btn:hover {
    color: white;
    background: rgba(255, 255, 255, 0.08);
    transform: scale(1.1);
  }
  .icon-btn:active {
    transform: scale(0.85);
    transition: all 0.1s;
  }

  .sub-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    opacity: 0.5;
    cursor: pointer;
    display: flex;
    align-items: center;
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    position: relative;
    padding: 8px;
  }
  .sub-btn:hover {
    opacity: 0.8;
    color: var(--text-main);
    transform: scale(1.1);
  }
  .sub-btn.active {
    opacity: 1;
    color: var(--primary);
  }
  .sub-btn:active {
    transform: scale(0.85);
    transition: all 0.1s;
  }

  @media (max-width: 768px) {
    .player {
      margin: 0;
      padding: 0 0.8rem;
      height: 72px;
      border-radius: 20px;
      border-bottom: none;
    }
    .album-name {
      display: none;
    }
    .progress-bg {
      background: rgba(var(--primary-rgb), 0.25);
      top: 0;
      bottom: 0;
      left: 0;
    }
    .right-actions {
      gap: 0.4rem;
    }
    .playback-btns {
      gap: 0.2rem;
    }
    .main-play-btn {
      width: 42px;
      height: 42px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    }
    .track-title {
      font-size: 0.95rem;
    }
    .icon-btn {
      padding: 6px;
    }
    .playback-btns .sub-btn {
      display: none;
    }
    .extra-controls {
      display: none; /* Ocultamos volumen y cola en miniplayer movil para ganar espacio */
    }
  }

  @media (max-width: 400px) {
    .player { padding: 0 0.5rem; }
    .now-playing { gap: 0.8rem; }
    .track-title { font-size: 0.85rem; }
    .main-play-btn { width: 38px; height: 38px; }
  }
</style>
