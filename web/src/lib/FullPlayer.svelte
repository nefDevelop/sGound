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
    [lyricContainerDesktop, lyricContainerMobile].forEach(container => {
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
        {#if subView !== 'cover' || !isDesktop}
          <header class="player-header {subView !== 'cover' ? 'compact-mode' : ''}">
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
                class="view-panel cinema-panel" 
                class:hide-cursor={!showControls}
                in:fade 
                onmousemove={resetControlsTimeout}
                onclick={togglePlay}
            >
              <!-- Liquid Background -->
              <div class="cinema-background">
                <img src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" alt="" class="bg-blur-layer layer-1" />
                <img src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" alt="" class="bg-blur-layer layer-2" />
                <img src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" alt="" class="bg-blur-layer layer-3" />
                <div class="bg-vignette"></div>
              </div>

              <button class="cinema-close-btn" class:visible={showControls} onclick={(e) => { e.stopPropagation(); subView = 'cover'; }}>
                <Icon name="x" size={32} />
              </button>

              <div class="cinema-content">
                <!-- Floating Lyrics -->
                {#if currentLyricIndex !== -1 && parsedLyrics[currentLyricIndex]}
                  <div class="cinema-lyrics-floating" in:fade={{duration: 1000}} out:fade>
                    {parsedLyrics[currentLyricIndex].text}
                  </div>
                {/if}

                <div class="cinema-artwork-box">
                   <div class="artwork-glare"></div>
                   <div class="artwork-bloom" style="background-image: url('/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}')"></div>
                   <img src="/api/cover/{currentPlaying.id}?v={$coverRefreshTrigger}" alt="" class="cinema-artwork" />
                </div>
                
                <div class="cinema-meta" class:visible={showControls}>
                  <h1 class="cinema-title">{currentPlaying.title}</h1>
                  <p class="cinema-artist">{currentPlaying.artist}</p>
                </div>

                <div class="cinema-controls" class:visible={showControls}>
                  <div class="cinema-timeline">
                    <span class="time-small">{formatTime(currentTime)}</span>
                    <div class="modern-slider progress-slider-mini">
                      <input type="range" class="knobless-range" min="0" max="100" value={progress} oninput={handleSeek} />
                      <div class="slider-fill" style="width: {progress}%"></div>
                    </div>
                    <span class="time-small">{formatTime(duration)}</span>
                  </div>
                  
                  <div class="cinema-btns">
                    <button class="c-btn" onclick={(e) => { e.stopPropagation(); onskipBack(); }}><Icon name="skipBack" size={28} /></button>
                    <button class="c-btn play-c" onclick={(e) => { e.stopPropagation(); togglePlay(); }}>
                      <Icon name={isPaused ? 'play' : 'pause'} size={44} />
                    </button>
                    <button class="c-btn" onclick={(e) => { e.stopPropagation(); onskipNext(); }}><Icon name="skipForward" size={28} /></button>
                  </div>
                </div>

                <!-- Next Up Hint -->
                {#if progress > 90 && $queue[$currentIndex + 1]}
                  <div class="cinema-next-up" in:fly={{y: 20, duration: 800}}>
                    <span class="next-label">A CONTINUACIÓN</span>
                    <span class="next-title">{$queue[$currentIndex + 1].title}</span>
                  </div>
                {/if}
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
    height: 100%;
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
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 0 !important;
  }

  .player-header.compact-mode .cover-wrapper-container {
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 !important;
  }

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
  .play-btn-prominent, .mini-btn.play, .play-c, .main-btn, .sub-btn, .c-btn {
    transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  }

  .play-btn-prominent:hover, .mini-btn.play:hover, .play-c:hover {
    transform: scale(1.08);
    filter: brightness(1.1);
  }

  .play-btn-prominent:active, .mini-btn.play:active, .play-c:active, .main-btn:active, .sub-btn:active, .c-btn:active {
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
    height: 100%;
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
    overflow: hidden;
  }

  .desktop-meta {
    text-align: center;
    width: 100%;
  }

  .desktop-meta h1 {
    font-size: 3rem;
    font-weight: 900;
    letter-spacing: -1.5px;
    margin: 0;
    line-height: 1;
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
    height: 40vh;
    max-width: 450px;
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
    width: auto;
    max-width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    z-index: 3;
    margin-top: 2rem;
  }

  .cover-wrapper {
    aspect-ratio: 1;
    width: 100%;
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

  /* Cinema Mode Polishing */
  .cinema-panel {
    position: fixed;
    inset: 0;
    z-index: 9999;
    background: black;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    overflow: hidden;
  }

  .cinema-title {
    font-size: 5rem;
    font-weight: 1000;
    letter-spacing: -3px;
    margin: 0;
    line-height: 0.9;
    background: linear-gradient(to bottom, #fff, #aaa);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    text-shadow: 0 30px 60px rgba(0,0,0,0.8);
  }

  @media (max-width: 1400px) {
    .cinema-title { font-size: 4rem; }
  }

  @media (max-width: 1024px) {
    .desktop-split-layout { display: none; }
    .mobile-stack-layout { display: flex; }
    .cinema-title { font-size: 2.5rem; }
    .cinema-artwork-box { width: 35vh; height: 35vh; }
    .lyric-line { font-size: 1.4rem; text-align: center; }
    .main-content-flow { padding-top: 0; }
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
    .cover-wrapper.main-artwork { height: 30vh; width: 30vh; }
    .desktop-meta h1 { font-size: 2rem; }
  }
</style>
