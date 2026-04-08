<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { inboxCount } from './stores.js';
  
  const dispatch = createEventDispatcher();
  let stats = $state({
    totalTracks: 0,
    totalAlbums: 0,
    totalArtists: 0,
    totalPlaylists: 0,
    totalGenres: 0,
    inboxCount: 0,
    forgottenGems: [],
    mostPlayed: []
  });
  let health = $state({ missingTags: 0, missingLyrics: 0, duplicateCount: 0, orphanCount: 0 });
  let loading = $state(true);

  let contextMenu = $state({
    visible: false,
    x: 0,
    y: 0,
    track: null,
    items: []
  });

  function showContextMenu(track, e) {
    e.preventDefault();
    contextMenu = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      track,
      items: [
        { label: 'Reproducir', icon: 'play', action: () => dispatch('play', track) },
        { label: 'Añadir a cola', icon: 'plus', action: () => dispatch('queue', track) },
        { divider: true },
        { label: 'Editar etiquetas', icon: 'edit', action: () => dispatch('edit', track) },
        { divider: true },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(track.title) },
        { label: 'Eliminar de biblioteca', icon: 'xCircle', color: 'danger', action: () => dispatch('delete', track) }
      ]
    };
  }

  export async function refresh() {
    try {
      const [sRes, hRes] = await Promise.all([
        fetch('/api/dashboard'),
        fetch('/api/library/health')
      ]);
      if (sRes.ok) {
        const data = await sRes.json();
        // Garantizar que siempre sean arrays
        data.forgottenGems = data.forgottenGems || [];
        data.mostPlayed = data.mostPlayed || [];
        stats = data;
        inboxCount.set(data.inboxCount);
      }
      if (hRes.ok) health = await hRes.json();
    } catch (e) { console.error(e); }
    finally { loading = false; }
  }

  onMount(() => {
    refresh();
  });
</script>

<div class="dashboard">
  <div class="bento-grid">
    <!-- Welcome Tile -->
    <section class="bento-item welcome-tile">
      <div class="welcome-header">
        <h1>Welcome back, <span>User</span></h1>
        {#if $inboxCount > 0}
          <button class="inbox-badge" onclick={() => dispatch('selectInbox')} aria-label="Ver Inbox">
            <Icon name="zap" size={14} />
            <span>{$inboxCount} pendientes</span>
          </button>
        {/if}
      </div>
      <div class="welcome-chips">
        <button class="chip primary" onclick={() => dispatch('selectView', 'tracks')} title="Canciones">
          <Icon name="music" size={14} /> {stats.totalTracks}
        </button>
        <button class="chip secondary" onclick={() => dispatch('selectView', 'artists')} title="Artistas">
          <Icon name="user" size={14} /> {stats.totalArtists}
        </button>
        <button class="chip tertiary" onclick={() => dispatch('selectView', 'albums')} title="Álbumes">
          <Icon name="disc" size={14} /> {stats.totalAlbums}
        </button>
        <button class="chip primary" onclick={() => dispatch('selectView', 'genres')} title="Géneros">
          <Icon name="hash" size={14} /> {stats.totalGenres}
        </button>
        <button class="chip secondary" onclick={() => dispatch('selectView', 'playlists')} title="Playlists">
          <Icon name="list" size={14} /> {stats.totalPlaylists}
        </button>
      </div>
    </section>


    <!-- Health Tile -->
    <section class="bento-item health-tile">
      <div class="tile-header">
        <Icon name="shield" size={18} color="var(--secondary)" />
        <h3>Salud</h3>
      </div>
      <div class="health-summary">
        <div class="h-row" class:warning={health.missingTags > 0}>
          <span class="lbl">S/ Etiquetas</span>
          <span class="val">{health.missingTags}</span>
        </div>
        <div class="h-row" class:warning={health.orphanCount > 0}>
          <span class="lbl">Huérfanos</span>
          <span class="val">{health.orphanCount}</span>
        </div>
      </div>
      <button class="maint-btn" onclick={() => dispatch('selectMaintenance')}>Gestionar <Icon name="settings" size={12} /></button>
    </section>

    <!-- Top Plays Tile -->
    <section class="bento-item most-played-tile">
      <div class="tile-header">
        <Icon name="activity" size={20} color="var(--secondary)" />
        <h3>Top Plays</h3>
      </div>
      <div class="top-list scroll-shadow">
        {#each stats.mostPlayed as top}
          <button class="top-item" onclick={() => dispatch('play', top)} aria-label={`Reproducir ${top.title}`}>
            <img src={`/api/cover/${top.id}`} alt="" onerror={(e) => { e.currentTarget.style.opacity='0'; }} />
            <div class="top-info">
              <span class="title">{top.title}</span>
              <span class="artist">{top.artist}</span>
            </div>
            <span class="play-count">{(top.playCount || 0)} <small>repro</small></span>
          </button>
        {/each}
        {#if !stats.mostPlayed || stats.mostPlayed.length === 0}
          <div class="empty">Empieza a escuchar para ver tus tendencias</div>
        {/if}
      </div>
    </section>

    <!-- Gems Tile (3x3 Grid) -->
    <section class="bento-item gems-tile">
      <div class="tile-header">
          <Icon name="archive" size={20} color="var(--primary)" />
          <h3>Forgotten Gems</h3>
      </div>
      <div class="gems-grid">
        {#each stats.forgottenGems as gem}
          <button 
            class="gem-cover" 
            onclick={() => dispatch('play', gem)} 
            oncontextmenu={(e) => showContextMenu(gem, e)}
            title={gem.title}
          >
            <img src={`/api/cover/${gem.id}`} alt={gem.title} onerror={(e) => { e.currentTarget.style.opacity='0'; }} />
            <div class="overlay"><Icon name="play" size={14} color="white" /></div>
          </button>
        {/each}
      </div>
    </section>

    <!-- Discovery Tile -->
    <section class="bento-item discovery-tile">
      <div class="tile-header">
        <Icon name="compass" size={20} color="var(--tertiary)" />
        <h3>Descubrir</h3>
      </div>
      <div class="discovery-visual">
        <div class="mosaic-grid">
          {#if stats.mostPlayed && stats.mostPlayed.length >= 5}
            {#each stats.mostPlayed.slice(0, 5) as track, i}
              <div class={`mosaic-item i${i}`}>
                <img src={`/api/cover/${track.id}`} alt="" />
                {#if i === 0}
                  <button class="overlay-chip bottom-left tertiary" onclick={() => dispatch('smartAlbums')}>Asignar a álbumes</button>
                {:else if i === 2}
                  <button class="overlay-chip bottom-right primary" onclick={() => dispatch('smartSearch')}>Sugerencias</button>
                {:else if i === 4}
                   <button class="overlay-chip center" onclick={() => dispatch('cleanupLibrary')}>Limpieza</button>
                {/if}
              </div>
            {/each}
          {:else}
            <!-- Placeholders si no hay suficientes datos -->
            <div class="mosaic-item i0 placeholder">
              <button class="overlay-chip bottom-left tertiary" onclick={() => dispatch('smartAlbums')}>Asignar a álbumes</button>
            </div>
            <div class="mosaic-item i1 placeholder"></div>
            <div class="mosaic-item i2 placeholder">
              <button class="overlay-chip bottom-right primary" onclick={() => dispatch('smartSearch')}>Sugerencias</button>
            </div>
            <div class="mosaic-item i3 placeholder"></div>
            <div class="mosaic-item i4 placeholder">
              <button class="overlay-chip center" onclick={() => dispatch('cleanupLibrary')}>Limpieza</button>
            </div>
          {/if}
        </div>
      </div>
    </section>

    <!-- Trends Tile -->
    <section class="bento-item trends-tile">
      <div class="tile-header">
        <Icon name="flame" size={20} color="var(--tertiary)" />
        <h3>Trends</h3>
      </div>
      <div class="genre-bars">
        <div class="bar-item">
          <div class="bar-info"><span>Electrónica</span> <small>42%</small></div>
          <div class="progress"><div class="fill" style="width: 42%"></div></div>
        </div>
        <div class="bar-item">
          <div class="bar-info"><span>Jazz</span> <small>28%</small></div>
          <div class="progress"><div class="fill sec" style="width: 28%"></div></div>
        </div>
        <div class="bar-item">
          <div class="bar-info"><span>Lo-Fi</span> <small>15%</small></div>
          <div class="progress"><div class="fill ter" style="width: 15%"></div></div>
        </div>
      </div>
    </section>
  </div>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    on:close={() => contextMenu.visible = false}
  />
</div>

<style>
  .dashboard { 
    padding: 1rem 1.5rem; 
    max-width: 100%;
    box-sizing: border-box;
    animation: fadeIn 0.5s ease;
    overflow-x: hidden;
  }
  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

  .bento-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-auto-rows: minmax(180px, auto);
    gap: 1.5rem;
  }


  .bento-item {
    background: var(--surface-container-high);
    border-radius: 32px;
    padding: 1.8rem;
    border: 1px solid var(--border-color);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    text-align: left;
    color: inherit;
    font-family: inherit;
  }
  .bento-item:not(button) { cursor: default; }
  .bento-item:hover { border-color: var(--primary-30); transform: translateY(-4px); }

  .tile-header { display: flex; align-items: center; gap: 0.8rem; margin-bottom: 1.2rem; }
  .tile-header h3 { margin: 0; font-size: 1.1rem; font-weight: 800; letter-spacing: -0.2px; }

  /* Welcome Tile */
  .welcome-tile { grid-column: span 3; background: linear-gradient(135deg, var(--surface-container-high) 0%, var(--surface-container-highest) 100%); position: relative; }
  .welcome-tile::after { content: ''; position: absolute; top: -50%; right: -20%; width: 280px; height: 280px; background: var(--primary-10); filter: blur(60px); border-radius: 50%; z-index: 0; }
  .welcome-content { position: relative; z-index: 1; }
  .welcome-tile h1 { font-size: 2.1rem; font-weight: 900; letter-spacing: -1.2px; margin: 0 0 0.5rem; }
  .welcome-tile h1 span { color: var(--primary); }

  .welcome-header { display: flex; flex-direction: column; align-items: flex-start; gap: 0.5rem; position: relative; z-index: 1; }
  .inbox-badge { background: var(--tertiary-10); color: var(--tertiary); border: 1px solid var(--tertiary-30); padding: 0.35rem 0.8rem; border-radius: 100px; display: flex; align-items: center; gap: 0.45rem; font-size: 0.72rem; font-weight: 800; cursor: pointer; transition: all 0.2s; }
  .inbox-badge:hover { background: var(--tertiary-20); transform: translateY(-1px); border-color: var(--tertiary); }

  /* Top Plays Tile */
  .most-played-tile { grid-column: span 1; grid-row: span 2; }
  .top-list { display: flex; flex-direction: column; gap: 0.5rem; overflow-y: auto; flex: 1; padding-right: 0.4rem; }
  .top-item { background: transparent; border: none; display: flex; align-items: center; gap: 0.8rem; padding: 0.5rem; border-radius: 16px; cursor: pointer; transition: background 0.2s; text-align: left; width: 100%; color: inherit; }
  .top-item:hover { background: var(--surface-container-highest); }
  .top-item img { width: 40px; height: 40px; border-radius: 8px; object-fit: cover; }
  .top-info { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
  .top-info .title { font-weight: 800; font-size: 0.85rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--text-main); }
  .top-info .artist { font-size: 0.7rem; color: var(--text-muted); font-weight: 700; }
  .play-count { font-size: 0.8rem; font-weight: 950; color: var(--secondary); text-align: right; line-height: 1.1; }
  .play-count small { font-size: 0.55rem; color: var(--text-muted); display: block; }
  .empty { font-size: 0.75rem; color: var(--text-muted); text-align: center; margin-top: 3rem; opacity: 0.5; font-weight: 600; }

  /* Gems Tile (3x3 Grid) */
  .gems-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.5rem; }
  .gem-cover { background: var(--surface-container-highest); padding: 0; border: 1px solid var(--border-color); aspect-ratio: 1/1; border-radius: 12px; overflow: hidden; cursor: pointer; position: relative; }
  .gem-cover img { width: 100%; height: 100%; object-fit: cover; }
  .gem-cover .overlay { position: absolute; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity 0.2s; }
  .gem-cover:hover .overlay { opacity: 1; }

  /* Discovery Tile */
  /* Discovery Mosaic */
  .discovery-tile { grid-column: span 1; grid-row: span 2; padding: 1.2rem; }
  .discovery-visual { flex: 1; display: flex; flex-direction: column; }
  .mosaic-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-template-rows: repeat(4, 1fr);
    gap: 0.6rem;
    flex: 1;
    min-height: 240px;
  }
  .mosaic-item { position: relative; border-radius: 12px; overflow: hidden; background: var(--surface-container-highest); border: 1px solid rgba(255,255,255,0.05); }
  .mosaic-item img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.4s; }
  .mosaic-item:hover img { transform: scale(1.1); }

  /* Spans */
  .mosaic-item.i0 { grid-column: span 2; grid-row: span 2; }
  .mosaic-item.i1 { grid-column: span 2; grid-row: span 1; }
  .mosaic-item.i2 { grid-column: span 1; grid-row: span 1; }
  .mosaic-item.i3 { grid-column: span 1; grid-row: span 1; }
  .mosaic-item.i4 { grid-column: span 4; grid-row: span 1; }

  /* Overlaid Chips */
  .overlay-chip {
    position: absolute;
    padding: 0.4rem 0.8rem;
    border-radius: 100px;
    font-size: 0.65rem;
    font-weight: 800;
    border: 1px solid rgba(255,255,255,0.1);
    color: white;
    cursor: pointer;
    backdrop-filter: blur(10px);
    -webkit-backdrop-filter: blur(10px);
    background: rgba(0,0,0,0.4);
    transition: all 0.2s;
    white-space: nowrap;
    z-index: 5;
  }
  .overlay-chip:hover { transform: scale(1.05); background: rgba(0,0,0,0.6); border-color: var(--primary); }
  .overlay-chip.primary { color: var(--primary); border-color: var(--primary-30); }
  .overlay-chip.tertiary { color: var(--tertiary); border-color: var(--tertiary-30); }

  .bottom-left { bottom: 8px; left: 8px; }
  .bottom-right { bottom: 8px; right: 8px; }
  .center { top: 50%; left: 50%; transform: translate(-50%, -50%); }
  .center:hover { transform: translate(-50%, -50%) scale(1.05); }

  .empty-mosaic { grid-column: span 4; display: flex; align-items: center; justify-content: center; font-size: 0.8rem; color: var(--text-muted); opacity: 0.5; }

  .placeholder { background: linear-gradient(45deg, var(--surface-container-highest), var(--surface-container-high)); position: relative; }
  .placeholder::after { content: ''; position: absolute; inset: 0; background: rgba(255,255,255,0.03); }

  .chip { background: var(--surface-container-highest); border: 1px solid var(--border-color); color: var(--text-main); font-size: 0.75rem; font-weight: 800; padding: 0.45rem 1rem; border-radius: 100px; cursor: pointer; transition: all 0.2s; white-space: nowrap; display: flex; align-items: center; gap: 0.5rem; }
  .chip:hover { border-color: var(--primary); transform: translateY(-2px); box-shadow: 0 4px 12px rgba(0,0,0,0.2); }
  .chip.primary { background: var(--primary-10); color: var(--primary); border-color: var(--primary-30); }
  .chip.secondary { background: var(--secondary-10); color: var(--secondary); border-color: var(--secondary-30); }
  .chip.tertiary { background: var(--tertiary-10); color: var(--tertiary); border-color: var(--tertiary-30); }

  .welcome-chips { margin-top: auto; padding-top: 1.5rem; display: flex; gap: 0.5rem; flex-wrap: wrap; position: relative; z-index: 1; }

  /* Trends Tile */
  .genre-bars { display: flex; flex-direction: column; gap: 0.8rem; }
  .bar-item { display: flex; flex-direction: column; gap: 0.3rem; }
  .bar-info { display: flex; justify-content: space-between; font-size: 0.75rem; font-weight: 800; }
  .bar-info small { color: var(--text-muted); }
  .progress { height: 5px; background: var(--surface-container-highest); border-radius: 10px; overflow: hidden; }
  .fill { height: 100%; background: var(--primary); border-radius: 10px; }
  .fill.sec { background: var(--secondary); }
  .fill.ter { background: var(--tertiary); }

  /* Health Tile */
  .health-summary { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 0.8rem; }
  .h-row { display: flex; justify-content: space-between; font-size: 0.78rem; font-weight: 700; color: var(--text-muted); }
  .h-row .val { color: var(--text-main); font-weight: 950; }
  .h-row.warning .val { color: var(--secondary); }
  .maint-btn { margin-top: auto; background: var(--surface-container-highest); border: 1px solid var(--border-color); color: var(--text-muted); padding: 0.6rem; border-radius: 14px; font-weight: 800; font-size: 0.7rem; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 0.4rem; transition: all 0.2s; }
  .maint-btn:hover { border-color: var(--primary); color: var(--primary); }

  /* Scroll Utility */
  .scroll-shadow::-webkit-scrollbar { width: 4px; }
  .scroll-shadow::-webkit-scrollbar-thumb { background: var(--border-color); border-radius: 10px; }

  /* Media Queries - At the end for proper precedence */
  @media (max-width: 1400px) {
    .bento-grid { grid-template-columns: repeat(3, 1fr); gap: 1.2rem; }
    .welcome-tile { grid-column: span 2; }
  }

  @media (max-width: 1100px) {
    .bento-grid { grid-template-columns: repeat(2, 1fr); gap: 1rem; }
    .welcome-tile { grid-column: span 2; }
  }

  @media (max-width: 768px) {
    .bento-grid { grid-template-columns: 1fr; gap: 1rem; }
    .welcome-tile { grid-column: span 1; }
    .bento-item { padding: 1.2rem; border-radius: 20px; }
  }
</style>
