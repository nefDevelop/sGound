<script>
  import { tick } from 'svelte';
  import Icon from './Icon.svelte';
  import { fade, fly } from 'svelte/transition';
  import { audioQuality, queue, currentIndex, volume, isShuffle, repeatMode, coverRefreshTrigger, showToast } from './stores.js';
  import { copyToClipboard } from './utils.js';
  import ContextMenu from './ContextMenu.svelte';
  import StarRating from './StarRating.svelte';
  import Artwork from './Artwork.svelte';

  let { 
    currentPlaying = null, 
    audioRef = null, 
    isOpen = $bindable(false), 
    progress = $bindable(0), 
    duration = $bindable(0), 
    currentTime = $bindable(0),
    subView = $bindable('cover'),
    onedit,
    onhide,
    ondeletePhysical,
    onskipNext,
    onskipBack
  } = $props();

  let lyrics = $state("");
  let parsedLyrics = $state([]);
  let currentLyricIndex = $state(-1);
  let lyricContainerDesktop = $state(null);
  let lyricContainerMobile = $state(null);
  let lyricContainerCinema = $state(null);
  let userScrolling = $state(false);
  let scrollTimeout;
  let isPaused = $state(true);
  let isDesktop = $state(true);

  $effect(() => {
    const check = () => isDesktop = window.innerWidth > 1024;
    check();
    window.addEventListener('resize', check);
    return () => window.removeEventListener('resize', check);
  });

  $effect(() => {
    if (!audioRef) return;
    const updateStatus = () => isPaused = audioRef.paused;
    audioRef.addEventListener('play', updateStatus);
    audioRef.addEventListener('pause', updateStatus);
    // Initial check
    isPaused = audioRef.paused;
    return () => {
      audioRef.removeEventListener('play', updateStatus);
      audioRef.removeEventListener('pause', updateStatus);
    };
  });

  function handleUserInteraction(e) {
    if (e.currentTarget) {
      const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
      console.log(`Scroll Event: ${e.type} | Top: ${scrollTop} | Height: ${scrollHeight} | Client: ${clientHeight}`);
    }
    userScrolling = true;
    clearTimeout(scrollTimeout);
    scrollTimeout = setTimeout(() => {
      userScrolling = false;
    }, 3000);
  }

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

  function toggleRepeat() {
    const modes = ['none', 'all', 'one'];
    const current = $repeatMode;
    const next = modes[(modes.indexOf(current) + 1) % modes.length];
    repeatMode.set(next);
  }

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    items: []
  });

  let showControls = $state(true);
  let controlsTimeout;
  let tiltX = $state(0);
  let tiltY = $state(0);

  function resetControlsTimeout() {
    if (subView !== 'cinema') {
        showControls = true;
        return;
    }
    showControls = true;
    clearTimeout(controlsTimeout);
    controlsTimeout = setTimeout(() => {
        if (subView === 'cinema') showControls = false;
    }, 3000);
  }

  function handleCinemaMouseMove(e) {
    if (subView === 'cinema') {
      const { clientX, clientY } = e;
      const { innerWidth, innerHeight } = window;
      tiltX = (clientX / innerWidth - 0.5) * 15;
      tiltY = (clientY / innerHeight - 0.5) * -15;
    }
    resetControlsTimeout();
  }

  let isFullscreen = $state(false);
  let cinemaContainer = $state();

  $effect(() => {
    const handler = () => {
      isFullscreen = !!document.fullscreenElement;
    };
    document.addEventListener('fullscreenchange', handler);
    return () => document.removeEventListener('fullscreenchange', handler);
  });

  function toggleFullscreen(e) {
    if (e) e.stopPropagation();
    if (!document.fullscreenElement) {
      if (cinemaContainer) {
        cinemaContainer.requestFullscreen().catch(err => {
          console.error(`Error: ${err.message}`);
        });
      }
    } else {
      if (document.exitFullscreen) document.exitFullscreen();
    }
  }

  function togglePlay() {
    if (!audioRef) return;
    if (audioRef.paused) audioRef.play();
    else audioRef.pause();
  }

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
        { label: 'Copiar título', icon: 'fileText', action: () => copyToClipboard(currentPlaying.title).then(ok => ok && showToast("Título copiado", "info")) },
        { divider: true },
        { label: 'Ocultar de la biblioteca', icon: 'eyeOff', action: () => onhide?.(currentPlaying) },
        { label: 'Borrar físicamente (Disco)', icon: 'trash', color: 'danger', action: () => ondeletePhysical?.(currentPlaying) }
      ]
    };
  }

  // Sincronizar letras usando currentTime reactivo
  $effect(() => {
    if (isOpen && parsedLyrics.length > 0) {
      const time = currentTime; // Track dependency
      const index = parsedLyrics.findIndex((l, i) => {
        const next = parsedLyrics[i + 1];
        return time >= l.time && (!next || time < next.time);
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
    lyrics = "Loading...";
    parsedLyrics = [];
    try {
      const res = await fetch(`/api/lyrics/${currentPlaying.id}`);
      const data = await res.json();
      lyrics = data.lyrics || "";
      if (lyrics) {
        parseLRC(lyrics);
      } else {
        lyrics = "No lyrics available.";
      }
    } catch (e) {
      lyrics = "Failed to load lyrics.";
      parsedLyrics = [];
    }
  }

  function parseLRC(lrc) {
    if (!lrc) {
      parsedLyrics = [];
      return;
    }
    
    console.log("📝 Parsing LRC content...");
    const lines = lrc.split('\n');
    const result = [];
    // Regex mejorada para detectar [mm:ss.xx], [mm:ss:xx] o [mm:ss]
    const timeReg = /\[(\d+):(\d+(?:[.:]\d+)?)\]/g;

    lines.forEach(line => {
      let match;
      const timestamps = [];
      let text = line;
      
      // Reiniciar lastIndex para el regex global
      timeReg.lastIndex = 0;
      
      while ((match = timeReg.exec(line)) !== null) {
        const mins = parseInt(match[1]);
        const secsStr = match[2].replace(':', '.'); 
        const secs = parseFloat(secsStr);
        timestamps.push(mins * 60 + secs);
        text = text.replace(match[0], '');
      }
      
      text = text.trim();
      if (text) {
        if (timestamps.length > 0) {
          timestamps.forEach(time => {
            result.push({ time, text });
          });
        } else if (result.length === 0) {
          // Si no hay timestamps pero es texto útil, lo guardamos con tiempo 0 (fallback)
          // Pero solo si no hemos encontrado NINGÚN timestamp en el archivo aún
        }
      }
    });

    if (result.length === 0 && lrc.trim()) {
      console.log("⚠️ No markers found, treating as plain text");
      parsedLyrics = lrc.split('\n')
        .map(l => l.trim())
        .filter(l => l && !l.startsWith('['))
        .map(text => ({ time: 0, text }));
    } else {
      parsedLyrics = result.sort((a, b) => a.time - b.time);
      console.log(`✅ Parsed ${parsedLyrics.length} synced lines`);
    }
    
    currentLyricIndex = -1; // Reset index when new lyrics load
  }

  function scrollToActive() {
    if (!isOpen || userScrolling) return;
    [lyricContainerDesktop, lyricContainerMobile, lyricContainerCinema].forEach(container => {
      if (!container) return;
      const activeLine = container.querySelector('.active');
      if (activeLine) {
        activeLine.scrollIntoView({
          behavior: 'smooth',
          block: 'center'
        });
      }
    });
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
  <!-- Immersive Background -->
  <div class="immersive-bg" aria-hidden="true">
    <div class="bg-layer layer-1" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
    <div class="bg-layer layer-2" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
    <div class="bg-overlay"></div>
  </div>

  <div class="view-content">
    <div class="player-body" class:compact-mode={subView !== 'cover'}>
      <div class="main-content-flow">
        {#if (subView !== 'cover' && subView !== 'queue') || !isDesktop}
          <header class="player-header {(subView !== 'cover') ? 'compact-mode' : ''}">
            <div class="header-inner">
              <div class="cover-wrapper" style="border-radius: {subView === 'cover' ? '24px' : '12px'}">
                 <Artwork 
                   src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" 
                   alt="Cover" 
                   borderRadius={subView === 'cover' ? '22px' : '10px'}
                   onclick={() => { if(subView !== 'cover') subView = 'cover'; }}
                 />
              </div>
            
              {#if subView !== 'cover'}
                <div class="compact-meta" in:fade>
                  <div class="meta-texts">
                    <h2>{currentPlaying.title}</h2>
                    <p>{currentPlaying.artist}</p>
                  </div>
                  <div class="compact-controls">
                    <div class="mini-timeline">
                      <span class="time-mini">{formatTime(currentTime)}</span>
                      <div class="modern-slider progress-slider-mini">
                        <input type="range" class="knobless-range" min="0" max="100" value={progress} oninput={handleSeek} />
                        <div class="slider-fill" style="width: {progress}%"></div>
                      </div>
                      <span class="time-mini">{formatTime(duration)}</span>
                    </div>

                    <div class="mini-btns">
                      <button class="mini-btn" onclick={(e) => { e.stopPropagation(); onskipBack(); }}><Icon name="skipBack" size={20} /></button>
                      <button class="mini-btn play" onclick={(e) => { e.stopPropagation(); isPaused ? audioRef?.play() : audioRef?.pause(); }}>
                        <Icon name={isPaused ? 'play' : 'pause'} size={24} />
                      </button>
                      <button class="mini-btn" onclick={(e) => { e.stopPropagation(); onskipNext(); }}><Icon name="skipForward" size={20} /></button>
                    </div>
                  </div>
                </div>
              {/if}
            </div>
          </header>
        {/if}
        
        <div class="dynamic-area">
          {#if subView !== 'cinema'}
          <!-- Desktop Side-by-Side View (Logic handled via CSS) -->
          <div class="desktop-split-layout" class:is-centered={subView === 'cover'} class:lyrics-mode={subView === 'lyrics'}>
            <div class="artwork-column">
              <div class="cover-wrapper-container">
                <div class="cover-wrapper main-artwork" style="border-radius: {subView === 'cover' ? '24px' : '12px'}">
                   <Artwork 
                     src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" 
                     alt="Cover" 
                     borderRadius={subView === 'cover' ? '22px' : '10px'}
                     onclick={() => { if(subView !== 'cover') subView = 'cover'; }}
                   />
                </div>
              </div>
              
              <div class="desktop-meta">
                <div class="meta-texts" oncontextmenu={showContextMenu} role="button" tabindex="0">
                  <h1>{currentPlaying.title}</h1>
                  <p class="artist-name">{currentPlaying?.artist || 'Unknown Artist'}</p>
                </div>
              </div>

              <div class="desktop-controls">
                <div class="playback-controls">
                  <div class="timeline-container">
                    <div class="modern-slider progress-slider">
                       <input type="range" class="knobless-range" min="0" max="100" value={progress} oninput={handleSeek} />
                       <div class="slider-fill" style="width: {progress}%"></div>
                       <div class="slider-glow" style="width: {progress}%"></div>
                    </div>
                    <div class="time-row">
                      <span class="time">{formatTime(currentTime)}</span>
                      <span class="time">{formatTime(duration)}</span>
                    </div>
                  </div>

                  <div class="main-btns">
                    <button class="sub-btn" class:active={$isShuffle} onclick={() => isShuffle.update(s => !s)} title="Aleatorio">
                      <Icon name="shuffle" size={24} />
                    </button>
                    <button class="main-btn" onclick={onskipBack} title="Anterior">
                      <Icon name="skipBack" size={42} />
                    </button>
                    <button class="play-btn-prominent" onclick={() => audioRef?.paused ? audioRef.play() : audioRef?.pause()} title={audioRef?.paused ? "Reproducir" : "Pausar"}>
                      <Icon name={audioRef?.paused ? 'play' : 'pause'} size={42} color="var(--surface)" />
                    </button>
                    <button class="main-btn" onclick={onskipNext} title="Siguiente">
                      <Icon name="skipForward" size={42} />
                    </button>
                    <button class="sub-btn" class:active={$repeatMode !== 'none'} onclick={toggleRepeat} title="Repetición">
                      <Icon name="repeat" size={24} />
                      {#if $repeatMode === 'one'}<span class="repeat-one-badge">1</span>{/if}
                    </button>
                  </div>

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
                </div>
              </div>
            </div>

            <div class="content-column">
              <div class="panel-content">
                {#if subView === 'lyrics'}
                  <div class="view-panel lyrics-panel" in:fade>
                    {#if parsedLyrics.length > 0}
                      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                      <div 
                        class="lyrics-list" 
                        bind:this={lyricContainerDesktop}
                        onwheel={handleUserInteraction}
                        onmousedown={handleUserInteraction}
                        ontouchstart={handleUserInteraction}
                        role="region"
                        aria-label="Letras de la canción"
                      >
                        {#each parsedLyrics as line, i}
                          <button 
                            class="lyric-line" 
                            class:active={i === currentLyricIndex}
                            onclick={(e) => { e.stopPropagation(); if(audioRef) audioRef.currentTime = line.time; }}
                            in:fade={{ delay: Math.min(i * 30, 1000), duration: 600 }}
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
                {:else if subView === 'queue'}
                  <div class="view-panel queue-panel" in:fade>
                    <div class="queue-list">
                      {#each $queue.slice($currentIndex + 1) as next, i}
                        <div 
                          class="queue-item" 
                          in:fly={{ y: 20, delay: i * 50, duration: 400 }}
                          onclick={() => currentIndex.set($currentIndex + 1 + i)}
                          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); currentIndex.set($currentIndex + 1 + i); }}}
                          role="button"
                          tabindex="0"
                        >
                          <span class="q-index">{i + 1}</span>
                          <Artwork 
                            src="/api/cover/{next.id}?v={$coverRefreshTrigger}" 
                            alt="" 
                            size="48px" 
                            borderRadius="12px" 
                          />
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
                {/if}
              </div>
            </div>
          </div>

          <!-- Mobile/Compact View -->
          <div class="mobile-stack-layout">
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
                       <div class="slider-glow" style="width: {progress}%"></div>
                    </div>
                    <div class="time-row">
                      <span class="time">{formatTime(currentTime)}</span>
                      <span class="time">{formatTime(duration)}</span>
                    </div>
                  </div>

                  <div class="main-btns">
                    <button class="sub-btn" class:active={$isShuffle} onclick={() => isShuffle.update(s => !s)} title="Aleatorio">
                      <Icon name="shuffle" size={22} />
                    </button>
                    <button class="main-btn" onclick={onskipBack} title="Anterior">
                      <Icon name="skipBack" size={38} />
                    </button>
                    <button class="play-btn-prominent" onclick={() => isPaused ? audioRef.play() : audioRef?.pause()} title={isPaused ? "Reproducir" : "Pausar"}>
                      <Icon name={isPaused ? 'play' : 'pause'} size={38} color="var(--surface)" />
                    </button>
                    <button class="main-btn" onclick={onskipNext} title="Siguiente">
                      <Icon name="skipForward" size={38} />
                    </button>
                    <button class="sub-btn" class:active={$repeatMode !== 'none'} onclick={toggleRepeat} title="Repetición">
                      <Icon name="repeat" size={22} />
                      {#if $repeatMode === 'one'}<span class="repeat-one-badge">1</span>{/if}
                    </button>
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
                    <div 
                      class="queue-item" 
                      in:fly={{ y: 20, delay: i * 40, duration: 400 }}
                      onclick={() => currentIndex.set($currentIndex + 1 + i)}
                      onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); currentIndex.set($currentIndex + 1 + i); }}}
                      role="button"
                      tabindex="0"
                    >
                      <span class="q-index">{i + 1}</span>
                      <Artwork 
                        src="/api/cover/{next.id}?v={$coverRefreshTrigger}" 
                        alt="" 
                        size="44px" 
                        borderRadius="8px" 
                      />
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
              <div class="view-panel lyrics-panel" in:fade>
                {#if parsedLyrics.length > 0}
                  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                  <div 
                    class="lyrics-list" 
                    bind:this={lyricContainerMobile}
                    onwheel={handleUserInteraction}
                    onmousedown={handleUserInteraction}
                    ontouchstart={handleUserInteraction}
                    role="region"
                    aria-label="Letras de la canción"
                  >
                    {#each parsedLyrics as line, i}
                      <button 
                        class="lyric-line" 
                        class:active={i === currentLyricIndex}
                        onclick={(e) => { e.stopPropagation(); if(audioRef) audioRef.currentTime = line.time; }}
                        in:fade={{ delay: Math.min(i * 20, 800), duration: 500 }}
                      >
                        {line.text}
                      </button>
                    {/each}
                  </div>
                {:else if lyrics === "Loading..."}
                   <div class="no-lyrics loading-state">
                      <div class="loading-spinner"></div>
                      <p>Buscando letra...</p>
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
          {:else if subView === 'cinema'}
            <div 
                role="presentation"
                class="cinema-view-container" 
                class:hide-ui={!showControls}
                bind:this={cinemaContainer}
                in:fade={{ duration: 800 }}
                onmousemove={handleCinemaMouseMove}
                onclick={togglePlay}
            >
              <!-- Liquid Ambient Background -->
              <div class="cinema-ambient-scene">
                <div class="ambient-blob blob-1" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
                <div class="ambient-blob blob-2" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
                <div class="ambient-blob blob-3" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
                <div class="ambient-noise"></div>
                <div class="ambient-vignette"></div>
              </div>

              <!-- Top UI: Actions -->
              <div class="cinema-top-ui" class:visible={showControls}>
                <div class="cinema-top-btns">
                  <button class="cinema-action-btn" title="Pantalla Completa" onclick={toggleFullscreen}>
                    <Icon name={isFullscreen ? "minimize" : "maximize"} size={20} />
                  </button>
                  <button class="cinema-exit-btn" onclick={(e) => { e.stopPropagation(); subView = 'cover'; }}>
                    <Icon name="x" size={20} />
                    <span>Salir</span>
                  </button>
                </div>
              </div>

              <div class="cinema-main-layout">
                <!-- Left: 3D Artwork & Dynamic Meta -->
                <div class="cinema-artwork-pane">
                  <div class="artwork-3d-container">
                    <div class="artwork-card-wrapper" style="transform: rotateX({tiltY}deg) rotateY({tiltX}deg)">
                       <img src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" alt="" class="artwork-main-img" />
                       <div class="artwork-reflection"></div>
                    </div>
                    <div class="artwork-shadow-glow" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
                  </div>
                  
                  <div class="cinema-track-info" class:visible={showControls}>
                    <h1 class="cinema-title-text">{currentPlaying.title}</h1>
                    <div class="cinema-artist-row">
                       <span class="cinema-artist-text">{currentPlaying.artist}</span>
                       {#if currentPlaying.album}
                         <span class="cinema-separator">•</span>
                         <span class="cinema-album-text">{currentPlaying.album}</span>
                       {/if}
                    </div>
                  </div>
                </div>

                <!-- Right: Immersive Lyrics -->
                <div class="cinema-lyrics-pane">
                  {#if parsedLyrics.length > 0}
                    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                    <div class="cinema-lyrics-scroller" 
                         bind:this={lyricContainerCinema}
                         onwheel={handleUserInteraction}
                         onmousedown={handleUserInteraction}
                         ontouchstart={handleUserInteraction}
                         role="region"
                         aria-label="Letras inmersivas"
                    >
                      {#each parsedLyrics as line, i}
                        <button class="cinema-lyric-item" 
                             class:active={i === currentLyricIndex}
                             class:nearby={Math.abs(i - currentLyricIndex) <= 2 && i !== currentLyricIndex}
                             class:distant={Math.abs(i - currentLyricIndex) > 2}
                             onclick={(e) => { e.stopPropagation(); if(audioRef) audioRef.currentTime = line.time; }}>
                          {line.text}
                        </button>
                      {/each}
                    </div>
                  {:else if lyrics === "Loading..."}
                     <div class="cinema-empty-state">
                        <div class="loading-pulse"></div>
                        <p>Sincronizando letras...</p>
                     </div>
                  {:else}
                    <div class="cinema-empty-state">
                      <div style="opacity: 0.2">
                        <Icon name="music" size={64} />
                      </div>
                      <p>Solo música, sin palabras</p>
                    </div>
                  {/if}
                </div>
              </div>

              <!-- Bottom UI: Glass Capsule Controls -->
              <div class="cinema-bottom-ui" class:visible={showControls} onclick={(e) => e.stopPropagation()} role="presentation">
                <div class="glass-capsule">
                  <div class="capsule-progress-bar">
                    <input type="range" class="capsule-range" min="0" max="100" value={progress} oninput={handleSeek} />
                    <div class="capsule-fill" style="width: {progress}%"></div>
                  </div>
                  
                  <div class="capsule-main-row">
                    <div class="capsule-time">{formatTime(currentTime)}</div>
                    
                    <div class="capsule-btns">
                      <button class="capsule-btn" onclick={(e) => { e.stopPropagation(); onskipBack(); }}><Icon name="skipBack" size={24} /></button>
                      <button class="capsule-btn-play" onclick={(e) => { e.stopPropagation(); togglePlay(); }}>
                        <Icon name={isPaused ? 'play' : 'pause'} size={32} />
                      </button>
                      <button class="capsule-btn" onclick={(e) => { e.stopPropagation(); onskipNext(); }}><Icon name="skipForward" size={24} /></button>
                    </div>

                    <div class="capsule-time">{formatTime(duration)}</div>
                  </div>
                </div>
              </div>
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
    height: 100%;
    background: var(--surface);
    color: white;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  /* Immersive Background */
  .immersive-bg {
    position: absolute;
    inset: 0;
    z-index: 1;
    overflow: hidden;
    pointer-events: none;
  }

  .bg-layer {
    position: absolute;
    inset: -10%;
    width: 120%;
    height: 120%;
    background-size: cover;
    background-position: center;
    background-repeat: no-repeat;
    filter: blur(80px) saturate(1.5) brightness(0.3);
    opacity: 0.5;
  }

  .bg-layer.layer-1 {
    animation: drift 30s infinite alternate ease-in-out;
  }
  .bg-layer.layer-2 {
    animation: drift 45s infinite alternate-reverse ease-in-out;
    filter: blur(100px) saturate(2) brightness(0.2);
    opacity: 0.3;
  }

  @keyframes drift {
    from { transform: scale(1) translate(-2%, -2%) rotate(0deg); }
    to { transform: scale(1.2) translate(2%, 2%) rotate(3deg); }
  }

  .bg-overlay {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at center, transparent 0%, rgba(0,0,0,0.4) 100%),
                linear-gradient(to bottom, rgba(0,0,0,0.2) 0%, rgba(0,0,0,0.6) 100%);
  }

  .view-content {
    position: relative;
    z-index: 3;
    flex: 1;
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
  }

  .player-body {
    flex: 1;
    display: flex;
    flex-direction: column;
    width: 100%;
  }


  .main-content-flow {
    width: 100%;
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    position: relative;
    justify-content: center;
  }

  .player-body.compact-mode .main-content-flow {
    justify-content: flex-start;
  }

  .dynamic-area {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  /* Responsive Layout Logic */
  .desktop-split-layout {
    display: none;
    width: 100%;
    flex: 1;
    min-height: 0;
    max-width: 1400px;
    margin: 0 auto;
    padding: 0rem 2rem 1rem 2rem;
    gap: 4rem;
    align-items: center;
  }
  @media (min-width: 1025px) {
    .desktop-split-layout { display: flex; }
  }

  .mobile-stack-layout {
    display: flex;
    flex-direction: column;
    width: 100%;
    max-width: 500px;
    margin: 0 auto;
    padding: 1rem 1rem 6rem 1rem;
    gap: 1.5rem;
  }
  @media (min-width: 1025px) {
    .mobile-stack-layout { display: none; }
  }

  .player-header {
    width: 100%;
    max-width: 500px;
    margin: 0 auto;
    padding: 1rem 1rem 1rem 1rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    background: rgba(0,0,0,0.25);
    backdrop-filter: blur(15px);
    flex-shrink: 0;
    z-index: 1000;
  }

  header.player-header.compact-mode {
    height: 90px !important;
    padding: 0 !important;
    margin: 0 !important;
    max-width: none;
    background: rgba(255,255,255,0.03);
    backdrop-filter: blur(25px) saturate(1.8);
    -webkit-backdrop-filter: blur(25px) saturate(1.8);
    border-bottom: 1px solid rgba(255,255,255,0.08);
    box-shadow: 0 4px 30px rgba(0,0,0,0.1);
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    z-index: 1000;
    display: block !important;
  }

  .header-inner {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    padding: 0 1.5rem;
    gap: 1.2rem;
    box-sizing: border-box;
  }

  @media (min-width: 1025px) {
    /* Header is removed from DOM via Svelte logic in cover mode */
  }

  .player-header.compact-mode .cover-wrapper {
    width: 48px;
    height: 48px;
    min-width: 48px;
    min-height: 48px;
    max-width: none !important;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 0 !important;
  }

  /* Removed unused selector .player-header.compact-mode .cover-wrapper-container */

  .compact-meta {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1.5rem;
    min-width: 0;
  }

  .compact-controls {
    display: flex;
    align-items: center;
    gap: 1.5rem;
    flex: 1;
    max-width: 600px;
  }

  .mini-timeline {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    flex: 1;
  }

  .time-mini {
    font-size: 0.75rem;
    font-weight: 700;
    opacity: 0.5;
    font-variant-numeric: tabular-nums;
  }

  .progress-slider-mini {
    flex: 1;
    height: 4px !important;
  }

  .mini-btns {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .mini-btn {
    background: none;
    border: none;
    color: white;
    opacity: 0.6;
    cursor: pointer;
    padding: 0.4rem;
    display: flex;
    align-items: center;
    transition: all 0.2s;
  }
  .mini-btn:hover { opacity: 1; transform: scale(1.1); }
  .play-btn-prominent, .mini-btn.play, .main-btn, .sub-btn {
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }

  .play-btn-prominent:hover, .mini-btn.play:hover {
    transform: scale(1.08);
    filter: brightness(1.1);
  }

  .play-btn-prominent:active, .mini-btn.play:active, .main-btn:active, .sub-btn:active {
    transform: scale(0.92);
    filter: brightness(0.9);
    transition: all 0.1s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .mini-btn.play { 
    opacity: 1; 
    background: white; 
    color: black; 
    border-radius: 50%; 
    padding: 0.6rem;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .player-header .cover-wrapper {
    width: 70vw;
    max-width: 320px;
    margin-bottom: 1rem;
  }

  @media (min-width: 1025px) {
    .desktop-split-layout { display: flex; }
    .mobile-stack-layout { display: none; }
    /* .player-header is now used as a unified sticky bar, don't hide it */
  }

  .artwork-column {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    gap: 2rem;
    min-width: 0;
    min-height: 0;
    max-height: 100%;
  }

  .content-column {
    flex: 1.2;
    display: flex;
    flex-direction: column;
    background: rgba(255,255,255,0.03);
    border-radius: 32px;
    border: 1px solid rgba(255,255,255,0.08);
    backdrop-filter: blur(20px);
    overflow: hidden;
    height: 100%;
    max-height: 70vh;
  }

  .desktop-split-layout.is-centered {
    max-width: 800px;
    justify-content: center;
    gap: 0;
  }

  /* Lyrics Mode Desktop: Header layout */
  .desktop-split-layout.lyrics-mode {
    flex-direction: column;
    justify-content: flex-start;
    padding-top: 0;
    gap: 0;
    max-width: none;
  }

  .desktop-split-layout.lyrics-mode .cover-wrapper-container {
    width: auto;
    margin: 0;
  }

  .desktop-split-layout.lyrics-mode .artwork-column {
    display: none;
  }

  .compact-meta .meta-texts {
    min-width: 0;
    flex: 0 1 auto;
    text-align: left;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
  }

  .compact-meta h2 {
    font-size: 1rem;
    font-weight: 600;
    margin: 0;
    line-height: 1.2;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 250px;
  }

  .compact-meta p {
    font-size: 0.85rem;
    opacity: 0.7;
    margin: 0;
    line-height: 1.2;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 250px;
  }

  .desktop-split-layout.lyrics-mode .desktop-meta {
    flex: 0 1 auto;
    text-align: left;
    margin: 0;
    min-width: 150px;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .desktop-split-layout.lyrics-mode .desktop-meta h1 {
    font-size: 1.1rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .desktop-split-layout.lyrics-mode .desktop-meta .artist-name {
    font-size: 0.85rem;
    margin: 0;
  }

  .desktop-split-layout.lyrics-mode .desktop-controls {
    flex: 1;
    max-width: none;
  }

  .desktop-split-layout.lyrics-mode .playback-controls {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
    height: 100%;
    flex: 1;
  }

  .desktop-split-layout.lyrics-mode .timeline-container {
    flex: 1;
    margin-bottom: 0;
  }

  .desktop-split-layout.lyrics-mode .main-btns {
    width: auto;
    gap: 0.5rem;
  }

  .desktop-split-layout.lyrics-mode .play-btn-prominent {
    width: 44px;
    height: 44px;
  }
  .desktop-split-layout.lyrics-mode .play-btn-prominent :global(svg) {
    width: 24px;
    height: 24px;
  }
  .desktop-split-layout.lyrics-mode .main-btn :global(svg) {
    width: 28px;
    height: 28px;
  }
  .desktop-split-layout.lyrics-mode .sub-btn { display: none; }
  .desktop-split-layout.lyrics-mode .time-row { display: none; }
  .desktop-split-layout.lyrics-mode .bottom-controls { display: none; }

  .desktop-split-layout.lyrics-mode .content-column {
    flex: 1;
    max-height: none;
    background: none;
    border: none;
    backdrop-filter: none;
    display: flex;
    padding: 0;
    min-height: 0;
    overflow: hidden;
  }
  .desktop-split-layout.is-centered .content-column {
    display: none;
  }

  .artwork-column {
    flex: 1;
    min-width: 0;
  }

  .desktop-meta {
    text-align: center;
    width: 100%;
    min-width: 0;
  }

  .desktop-meta h1 {
    font-size: 3rem;
    font-weight: 900;
    letter-spacing: -1.5px;
    margin: 0;
    line-height: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
    padding: 0 1rem;
  }

  .desktop-meta .artist-name {
    font-size: 1.4rem;
    opacity: 0.6;
    margin-top: 0.5rem;
  }

  .desktop-controls {
    width: 100%;
    max-width: 500px;
  }

  /* Common UI Elements */
  .cover-wrapper.main-artwork {
    width: 40vh;
    height: auto;
    aspect-ratio: 1;
    max-width: min(100%, 450px);
    max-height: 450px;
    box-shadow: 0 50px 100px -20px rgba(0,0,0,0.8);
    transition: transform 0.6s cubic-bezier(0.16, 1, 0.3, 1);
  }
  .cover-wrapper.main-artwork:hover {
    transform: scale(1.02) translateY(-10px);
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
  }
  .play-btn-prominent:hover {
    box-shadow: 0 15px 40px rgba(255,255,255,0.3);
  }

  .slider-glow {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background: white;
    filter: blur(8px);
    opacity: 0.3;
    z-index: 0;
    border-radius: 20px;
  }

  .progress-slider:hover .slider-glow {
    opacity: 0.6;
  }

  .playback-controls {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .timeline-container {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }

  .time-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.85rem;
    font-weight: 600;
    opacity: 0.5;
    font-variant-numeric: tabular-nums;
  }

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

  .cover-wrapper-container {
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 3;
    margin: 0;
    flex: 1;
    min-height: 0;
    width: 100%;
  }

  .cover-wrapper {
    aspect-ratio: 1;
    width: 100%;
    max-width: min(100%, 70vh);
    margin: 0 auto;
    flex-shrink: 1;
    min-height: 0;
    /* border-radius set via style binding in template */
    overflow: hidden;
    background: var(--surface-container-high);
    border: 1px solid rgba(255,255,255,0.1);
  }

  .player-island {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .track-meta {
    display: flex;
    justify-content: center;
    width: 100%;
  }

  .meta-texts {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 0;
    width: 100%;
  }

  .meta-texts h1 {
    font-size: 2rem;
    font-weight: 850;
    letter-spacing: -1px;
    margin: 0;
    text-align: center;
    line-height: 1.1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    width: 100%;
  }

  .artist-name {
    font-size: 1.1rem;
    font-weight: 700;
    opacity: 0.5;
    text-align: center;
    width: 100%;
    margin: 0.4rem 0 0 0;
  }

  /* Panels */
  .view-panel { 
    width: 100%; 
    display: flex; 
    flex-direction: column; 
    gap: 1.5rem; 
    animation: fadeInQuick 0.2s cubic-bezier(0,0,0.2,1); 
    flex: 1;
    min-height: 0;
  }

  .lyrics-panel {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .dynamic-area {
    position: relative;
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .panel-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  @keyframes fadeInQuick { 
    from { opacity: 0; transform: translateY(4px); } 
    to { opacity: 1; transform: translateY(0); } 
  }

  .queue-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    height: 100%;
    overflow-y: auto;
    padding: 1.5rem;
    scrollbar-width: none;
  }
  .queue-list::-webkit-scrollbar { display: none; }

  .queue-item {
    display: flex;
    align-items: center;
    gap: 1.2rem;
    padding: 1rem;
    border-radius: 20px;
    background: rgba(255,255,255,0.03);
    border: 1px solid transparent;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    cursor: pointer;
  }
  .queue-item:hover {
    background: rgba(255,255,255,0.08);
    border-color: rgba(255,255,255,0.1);
    transform: translateX(4px) scale(1.01);
  }

  .q-index {
    font-size: 0.8rem;
    font-weight: 900;
    opacity: 0.3;
    width: 20px;
    text-align: center;
  }

  .q-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .q-title {
    font-weight: 800;
    font-size: 1rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .q-artist {
    font-size: 0.85rem;
    font-weight: 600;
    opacity: 0.5;
  }

  .empty-queue {
    text-align: center;
    padding: 4rem;
    opacity: 0.4;
    font-weight: 700;
    font-size: 0.9rem;
    letter-spacing: 1px;
    text-transform: uppercase;
  }

  .lyrics-list {
    display: block;
    flex: 1;
    width: 100%;
    overflow-y: auto;
    scrollbar-width: none;
    position: relative;
    padding: 50vh 2rem;
    padding-top: 100px; /* Extra padding for the absolute header */
    pointer-events: auto !important;
    user-select: none;
    touch-action: pan-y;
    -webkit-overflow-scrolling: touch;
    z-index: 10;
    min-height: 0;
  }
  .lyrics-list::-webkit-scrollbar { display: none; }

  .lyric-line {
    font-size: 2.2rem;
    font-weight: 900;
    color: rgba(255,255,255,0.15);
    background: none;
    border: none;
    cursor: pointer;
    text-align: center;
    transition: all 0.5s cubic-bezier(0.16, 1, 0.3, 1);
    line-height: 1.4;
    letter-spacing: -1px;
    padding: 1rem 0;
    width: 100%;
    margin-bottom: 1.5rem;
    display: block;
  }
  .lyric-line.active {
    color: white;
    transform: scale(1.05);
    filter: drop-shadow(0 0 20px rgba(255,255,255,0.3));
  }
  .lyric-line:hover {
    color: rgba(255,255,255,0.6);
  }

  .no-lyrics {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    gap: 1.5rem;
    opacity: 0.15;
    min-height: 50vh;
  }
  .no-lyrics p { font-size: 1.2rem; font-weight: 800; letter-spacing: 1px; text-transform: uppercase; }

  .loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255,255,255,0.1);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* --- CINEMA MODE REDESIGN (FROM SCRATCH) --- */
  .cinema-view-container {
    position: absolute;
    inset: 0;
    z-index: 9999;
    background: #000;
    color: #fff;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    cursor: default;
    min-height: 100%;
  }

  .cinema-view-container:fullscreen {
    width: 100vw;
    height: 100vh;
    position: fixed;
  }

  .cinema-view-container:-webkit-full-screen {
    width: 100vw;
    height: 100vh;
  }

  .cinema-view-container.hide-ui {
    cursor: none;
  }

  /* Liquid Ambient Background */
  .cinema-ambient-scene {
    position: absolute;
    inset: -10%;
    z-index: 1;
    pointer-events: none;
    overflow: hidden;
    filter: blur(120px) saturate(1.8) brightness(0.4);
  }

  .ambient-blob {
    position: absolute;
    width: 100%;
    height: 100%;
    background-size: cover;
    background-position: center;
    border-radius: 50%;
    opacity: 0.6;
  }

  .blob-1 {
    width: 120%;
    height: 120%;
    top: -10%;
    left: -10%;
    animation: liquid-move-1 40s infinite alternate ease-in-out;
  }
  .blob-2 {
    width: 100%;
    height: 100%;
    bottom: -10%;
    right: -10%;
    animation: liquid-move-2 30s infinite alternate-reverse ease-in-out;
    opacity: 0.4;
  }
  .blob-3 {
    width: 80%;
    height: 80%;
    top: 20%;
    left: 20%;
    animation: liquid-move-3 25s infinite alternate ease-in-out;
    opacity: 0.3;
  }

  @keyframes liquid-move-1 {
    0% { transform: translate(0, 0) scale(1) rotate(0deg); }
    100% { transform: translate(10%, 5%) scale(1.1) rotate(10deg); }
  }
  @keyframes liquid-move-2 {
    0% { transform: translate(0, 0) scale(1.1) rotate(0deg); }
    100% { transform: translate(-8%, -10%) scale(1) rotate(-8deg); }
  }
  @keyframes liquid-move-3 {
    0% { transform: translate(0, 0) scale(0.9); }
    100% { transform: translate(15%, -15%) scale(1.2); }
  }

  .ambient-noise {
    position: absolute;
    inset: 0;
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.65' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)'/%3E%3C/svg%3E");
    opacity: 0.05;
    mix-blend-mode: overlay;
    z-index: 4;
  }

  .ambient-vignette {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at center, transparent 0%, rgba(0,0,0,0.4) 60%, rgba(0,0,0,0.8) 100%);
    z-index: 5;
  }

  /* Top UI */
  .cinema-top-ui {
    position: absolute;
    top: 3rem;
    left: 4rem;
    right: 4rem;
    display: flex;
    justify-content: flex-end;
    z-index: 100;
    opacity: 0;
    transform: translateY(-20px);
    transition: all 0.6s cubic-bezier(0.16, 1, 0.3, 1);
    pointer-events: none;
  }

  .cinema-top-ui.visible {
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }

  .cinema-top-btns {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: rgba(255, 255, 255, 0.08);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    padding: 0.5rem;
    border-radius: 100px;
  }

  .cinema-action-btn, .cinema-exit-btn {
    background: transparent;
    border: none;
    color: #fff;
    height: 42px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .cinema-action-btn {
    width: 42px;
    border-radius: 50%;
  }

  .cinema-exit-btn {
    padding: 0 1.2rem;
    border-radius: 100px;
    gap: 0.5rem;
    font-weight: 600;
    font-size: 0.9rem;
    background: rgba(255, 255, 255, 0.1);
  }

  .cinema-action-btn:hover, .cinema-exit-btn:hover {
    background: rgba(255, 255, 255, 0.2);
    transform: scale(1.05);
  }

  .cinema-exit-btn:hover {
    background: rgba(255, 255, 255, 0.15);
    transform: scale(1.05);
  }

  /* Main Layout */
  .cinema-main-layout {
    flex: 1;
    display: flex;
    width: 100%;
    max-width: 1600px;
    margin: 0 auto;
    padding: clamp(2rem, 5vh, 6rem) 4rem;
    align-items: center;
    justify-content: center;
    gap: clamp(2rem, 5vw, 6rem);
    z-index: 10;
    position: relative;
    box-sizing: border-box;
  }

  /* Left Pane: Artwork */
  .cinema-artwork-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    max-width: 600px;
  }

  .artwork-3d-container {
    position: relative;
    width: 100%;
    max-width: clamp(180px, 35vh, 450px);
    aspect-ratio: 1;
    perspective: 1200px;
    margin-bottom: 1.5rem;
  }

  .artwork-card-wrapper {
    width: 100%;
    height: 100%;
    position: relative;
    z-index: 2;
    border-radius: 24px;
    overflow: hidden;
    box-shadow: 0 30px 60px rgba(0,0,0,0.8);
    transform: rotateX(2deg) rotateY(-5deg);
    transition: transform 0.8s cubic-bezier(0.16, 1, 0.3, 1);
    animation: artwork-float 12s infinite alternate ease-in-out;
  }

  @keyframes artwork-float {
    0% { transform: rotateX(2deg) rotateY(-5deg) translateY(0); }
    100% { transform: rotateX(4deg) rotateY(3deg) translateY(-20px); }
  }

  .artwork-main-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .artwork-reflection {
    position: absolute;
    inset: 0;
    background: linear-gradient(135deg, rgba(255,255,255,0.15) 0%, transparent 40%, rgba(255,255,255,0.05) 100%);
    z-index: 3;
    pointer-events: none;
  }

  .artwork-shadow-glow {
    position: absolute;
    inset: -5%;
    background-size: cover;
    background-position: center;
    filter: blur(50px) saturate(2);
    opacity: 0.5;
    z-index: 1;
    border-radius: 40px;
    transform: translateY(20px) scale(0.95);
  }

  .cinema-track-info {
    text-align: center;
    width: 100%;
    opacity: 0;
    transform: translateY(20px);
    transition: all 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.2s;
  }

  .cinema-track-info.visible {
    opacity: 1;
    transform: translateY(0);
  }

  .cinema-title-text {
    font-size: clamp(2rem, 5vh, 3.5rem);
    font-weight: 900;
    letter-spacing: -2px;
    margin: 0 0 0.5rem 0;
    line-height: 1.1;
    text-shadow: 0 10px 30px rgba(0,0,0,0.6);
  }

  .cinema-artist-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.8rem;
    font-size: clamp(1rem, 2.5vh, 1.6rem);
    font-weight: 600;
    opacity: 0.7;
  }

  .cinema-separator {
    opacity: 0.3;
  }

  /* Right Pane: Lyrics */
  .cinema-lyrics-pane {
    flex: 1.2;
    height: 100%;
    max-height: 60vh;
    position: relative;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .cinema-lyrics-scroller {
    height: 100%;
    overflow-y: auto;
    scrollbar-width: none;
    padding: 30vh 0;
    scroll-behavior: smooth;
    mask-image: linear-gradient(to bottom, transparent, black 25%, black 75%, transparent);
    -webkit-mask-image: linear-gradient(to bottom, transparent, black 25%, black 75%, transparent);
  }

  .cinema-lyrics-scroller::-webkit-scrollbar { display: none; }

  .cinema-lyric-item {
    font-size: 3.2rem;
    font-weight: 800;
    line-height: 1.25;
    padding: 1.2rem 0;
    margin: 0;
    color: rgba(255,255,255,0.25);
    text-align: left;
    background: none;
    border: none;
    cursor: pointer;
    width: 100%;
    transition: all 0.7s cubic-bezier(0.16, 1, 0.3, 1);
    transform-origin: left center;
    letter-spacing: -1.5px;
    display: block;
    filter: blur(2px);
    transform: scale(0.95);
  }

  .cinema-lyric-item:hover {
    color: rgba(255,255,255,0.6);
    filter: blur(0);
  }

  .cinema-lyric-item.active {
    color: #fff;
    transform: scale(1.05) translateX(20px);
    filter: blur(0);
    text-shadow: 0 0 40px rgba(255,255,255,0.4), 0 10px 20px rgba(0,0,0,0.5);
    padding: 2.5rem 0;
  }

  .cinema-lyric-item.nearby {
    color: rgba(255,255,255,0.5);
    filter: blur(0.5px);
    transform: scale(0.98);
  }

  .cinema-empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 2rem;
    color: rgba(255,255,255,0.4);
  }

  .cinema-empty-state p {
    font-size: 1.8rem;
    font-weight: 700;
    letter-spacing: 1px;
    text-transform: uppercase;
  }

  .loading-pulse {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    background: #fff;
    opacity: 0.1;
    animation: pulse 2s infinite ease-in-out;
  }

  @keyframes pulse {
    0% { transform: scale(0.8); opacity: 0.1; }
    50% { transform: scale(1.2); opacity: 0.3; }
    100% { transform: scale(0.8); opacity: 0.1; }
  }

  /* Bottom UI: Capsule */
  .cinema-bottom-ui {
    position: absolute;
    bottom: 3rem;
    left: 0;
    width: 100%;
    display: flex;
    justify-content: center;
    z-index: 200;
    opacity: 0;
    transform: translateY(30px);
    transition: all 0.6s cubic-bezier(0.16, 1, 0.3, 1);
    pointer-events: none;
  }

  .cinema-bottom-ui.visible {
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }

  .glass-capsule {
    background: rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(30px) saturate(2);
    -webkit-backdrop-filter: blur(30px) saturate(2);
    border: 1px solid rgba(255, 255, 255, 0.15);
    padding: 1.2rem 2rem;
    border-radius: 40px;
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    width: auto;
    min-width: 500px;
    box-shadow: 0 25px 60px rgba(0,0,0,0.6);
  }

  .capsule-progress-bar {
    position: relative;
    width: calc(100% - 1rem);
    margin: 0 auto;
    height: 6px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 10px;
    overflow: hidden;
  }

  .capsule-fill {
    position: absolute;
    height: 100%;
    background: #fff;
    box-shadow: 0 0 15px rgba(255,255,255,0.8);
    transition: width 0.1s linear;
  }

  .capsule-range {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
    z-index: 10;
  }

  .capsule-main-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 2rem;
  }

  .capsule-time {
    font-size: 0.85rem;
    font-weight: 700;
    opacity: 0.5;
    font-variant-numeric: tabular-nums;
    min-width: 60px;
    text-align: center;
  }

  .capsule-btns {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  .capsule-btn {
    background: none;
    border: none;
    color: #fff;
    opacity: 0.6;
    cursor: pointer;
    transition: all 0.3s;
    padding: 0.5rem;
    display: flex;
    align-items: center;
  }

  .capsule-btn:hover {
    opacity: 1;
    transform: scale(1.15);
  }

  .capsule-btn-play {
    background: #fff;
    color: #000;
    border: none;
    width: 56px;
    height: 56px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
    box-shadow: 0 8px 20px rgba(0,0,0,0.3);
  }

  .capsule-btn-play:hover {
    transform: scale(1.1);
    box-shadow: 0 15px 30px rgba(255,255,255,0.2);
  }

  /* Responsive Adjustments */
  @media (max-width: 1400px) {
    .cinema-title-text { font-size: 2.8rem; }
    .cinema-lyric-item { font-size: 2.6rem; }
    .cinema-main-layout { gap: 4rem; }
  }

  @media (max-width: 1024px) {
    .cinema-main-layout {
      flex-direction: column;
      padding: 5rem 2rem 10rem 2rem;
      gap: 2rem;
      justify-content: flex-start;
      overflow-y: auto;
    }

    .cinema-artwork-pane {
      max-width: none;
      flex: none;
    }

    .artwork-3d-container {
      max-width: 300px;
      margin-bottom: 2rem;
    }

    .cinema-title-text { font-size: 2.2rem; }
    .cinema-artist-row { font-size: 1.2rem; }

    .cinema-lyrics-pane {
      width: 100%;
      height: auto;
      flex: none;
    }

    .cinema-lyrics-scroller {
      padding: 0;
      mask-image: none;
      -webkit-mask-image: none;
      height: auto;
    }

    .cinema-lyric-item {
      font-size: 1.8rem;
      text-align: center;
      padding: 1rem 0;
    }
    
    .cinema-lyric-item.active {
      transform: scale(1.05) translateX(0);
      padding: 1.5rem 0;
    }

    .glass-capsule {
      min-width: 0;
      width: 92vw;
      padding: 0.8rem 1.2rem;
    }
    .capsule-main-row {
      gap: 0.5rem;
    }

    .cinema-top-ui {
      top: 2rem;
      left: 1rem;
      right: 1rem;
    }
  }

  @media (max-width: 480px) {
    .cinema-title-text { font-size: 1.8rem; }
    .cinema-lyric-item { font-size: 1.5rem; }
    .capsule-time { display: none; }
    .glass-capsule { padding: 1rem; }
    .cinema-bottom-ui { bottom: 1.5rem; }
  }

  @media (max-width: 400px) {
    .play-btn-prominent {
      width: 64px;
      height: 64px;
    }
    .main-btn :global(svg) {
      width: 32px !important;
      height: 32px !important;
    }
    .play-btn-prominent :global(svg) {
      width: 32px !important;
      height: 32px !important;
    }
    .sub-btn :global(svg) {
      width: 18px !important;
      height: 18px !important;
    }
    .main-btns {
      padding: 0 0.5rem;
    }
    .player-header {
      padding: 0 1rem;
    }
    .player-header .cover-wrapper {
      max-width: 240px;
    }
  }

  @media (max-width: 1024px) {
    .player-header.compact-mode {
      height: 60px !important;
      padding: 0 1rem;
      gap: 0.5rem;
    }
    .player-header.compact-mode .cover-wrapper {
      width: 40px;
      height: 40px;
      min-width: 40px;
      min-height: 40px;
      max-width: none !important;
    }
    .compact-meta {
      gap: 0.5rem;
    }
    .compact-controls {
      gap: 0.5rem;
    }
    .mini-timeline {
      display: flex;
      gap: 0.4rem;
    }
    .time-mini { display: none; } /* Hide times on mobile to save space */
    .compact-meta .meta-texts h2 { font-size: 0.85rem; max-width: 120px; }
    .compact-meta .meta-texts p { font-size: 0.75rem; max-width: 120px; }
  }

  @media (max-height: 700px) {
    .artwork-column { gap: 1.5rem; }
    .cover-wrapper.main-artwork { width: 30vh; }
    .desktop-meta h1 { font-size: 2rem; }
  }
</style>
