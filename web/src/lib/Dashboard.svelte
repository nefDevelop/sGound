<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import ContextMenu from './ContextMenu.svelte';
  import { inboxCount } from './stores.js';
  
  let { 
    onplay, 
    onqueue, 
    onedit, 
    onhide, 
    ondeletePhysical, 
    onselectInbox, 
    onselectView, 
    onselectPlaylist, 
    onselectMaintenance,
    onsmartAlbums,
    onsmartSearch,
    onCleanupLibrary,
    scanStatus
  } = $props();

  let stats = $state({
    totalTracks: 0,
    totalAlbums: 0,
    totalArtists: 0,
    totalPlaylists: 0,
    totalGenres: 0,
    inboxCount: 0,
    forgottenGems: [],
    mostPlayed: [],
    discovery: [],
    trends: []
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
        { label: 'Play', icon: 'play', action: () => onplay?.(track) },
        { label: 'Add to queue', icon: 'plus', action: () => onqueue?.(track) },
        { divider: true },
        { label: 'Edit tags', icon: 'edit', action: () => onedit?.(track) },
        { divider: true },
        { label: 'Copiar título', icon: 'fileText', action: () => navigator.clipboard.writeText(track.title) },
        { divider: true },
        { label: 'Ocultar de la biblioteca', icon: 'eyeOff', action: () => onhide?.(track) },
        { label: 'Borrar físicamente (Disco)', icon: 'trash', color: 'danger', action: () => ondeletePhysical?.(track) }
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
        data.forgottenGems = data.forgottenGems || [];
        data.mostPlayed = data.mostPlayed || [];
        data.trends = data.trends || [];
        stats = data;
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
          <button class="inbox-notification-badge" onclick={onselectInbox} title="View pending tracks">
            <Icon name="inbox" size={18} />
            <span class="badge-number">{$inboxCount}</span>
          </button>
        {/if}
      </div>
      <div class="welcome-chips">
        <button class="chip primary" onclick={() => onselectView?.('tracks')} title="Songs">
          <Icon name="music" size={14} /> {stats.totalTracks}
        </button>
        <button class="chip secondary" onclick={() => onselectView?.('artists')} title="Artists">
          <Icon name="user" size={14} /> {stats.totalArtists}
        </button>
        <button class="chip tertiary" onclick={() => onselectView?.('albums')} title="Albums">
          <Icon name="disc" size={14} /> {stats.totalAlbums}
        </button>
        <button class="chip primary" onclick={() => onselectView?.('genres')} title="Genres">
          <Icon name="hash" size={14} /> {stats.totalGenres}
        </button>
        <button class="chip secondary" onclick={() => onselectView?.('playlists')} title="Playlists">
          <Icon name="list" size={14} /> {stats.totalPlaylists}
        </button>
      </div>

      {#if scanStatus?.scanning}
        <div class="welcome-scan-status">
          <div class="welcome-spinner"></div>
          <div class="scan-info">
            <span class="label">Indexando...</span>
            <span class="count"><b>{scanStatus.processed}</b> canciones</span>
          </div>
        </div>
      {/if}
    </section>

    <!-- Health Tile -->
    {#if health.missingTags > 0 || health.orphanCount > 0 || health.duplicateCount > 0}
    <section class="bento-item health-tile">
      <div class="tile-header">
        <Icon name="shield" size={18} color="var(--secondary)" />
        <h3>Health</h3>
      </div>
      <div class="health-summary">
        <div class="h-row" class:warning={health.missingTags > 0}>
          <span class="lbl">No Tags</span>
          <span class="val">{health.missingTags}</span>
        </div>
        <div class="h-row" class:warning={health.orphanCount > 0}>
          <span class="lbl">Orphans</span>
          <span class="val">{health.orphanCount}</span>
        </div>
      </div>
      <button class="maint-btn" onclick={onselectMaintenance}>Manage <Icon name="settings" size={12} /></button>
    </section>
    {/if}

    <!-- Top Plays Tile -->
    {#if stats.mostPlayed && stats.mostPlayed.length > 0}
    <section class="bento-item most-played-tile">
      <div class="tile-header">
        <Icon name="activity" size={20} color="var(--secondary)" />
        <h3>Top Plays</h3>
      </div>
      <div class="top-list scroll-shadow">
        {#each stats.mostPlayed as top}
          <button class="top-item" onclick={() => onplay?.(top)} aria-label={`Play ${top.title}`}>
            <img 
              src={`/api/cover/${top.id}`} 
              alt="" 
              onerror={(e) => { 
                const target = e.currentTarget;
                if (target instanceof HTMLImageElement) target.style.opacity='0'; 
              }} 
            />
            <div class="top-info">
              <span class="title">{top.title}</span>
              <span class="artist">{top.artist}</span>
            </div>
            <span class="play-count">{(top.playCount || 0)} <small>plays</small></span>
          </button>
        {/each}
      </div>
    </section>
    {/if}

    <!-- Gems Tile -->
    {#if stats.forgottenGems && stats.forgottenGems.length > 0}
    <section class="bento-item gems-tile">
      <div class="tile-header">
          <Icon name="archive" size={20} color="var(--primary)" />
          <h3>Forgotten Gems</h3>
      </div>
      <div class="gems-grid" 
           class:lone-gem={stats.forgottenGems.length === 1}
           class:few-gems={stats.forgottenGems.length > 1 && stats.forgottenGems.length <= 4}>
        {#each stats.forgottenGems as gem}
          <button 
            class="gem-cover" 
            onclick={() => onplay?.(gem)} 
            oncontextmenu={(e) => showContextMenu(gem, e)}
            title={gem.title}
          >
            <img src={`/api/cover/${gem.id}`} alt={gem.title} onerror={(e) => { 
              const target = e.currentTarget;
              if (target instanceof HTMLImageElement) target.style.opacity='0'; 
            }} />
            <div class="overlay"><Icon name="play" size={14} color="white" /></div>
          </button>
        {/each}
      </div>
    </section>
    {/if}

    <!-- Discovery Tile -->
    {#if stats.discovery && stats.discovery.length > 0}
    <section class="bento-item discovery-tile">
      <div class="tile-header">
        <Icon name="compass" size={20} color="var(--tertiary)" />
        <h3>Discovery</h3>
      </div>
      <div class="discovery-visual">
        <div class="mosaic-grid">
          {#each stats.discovery as album, i}
              <div class={`mosaic-item i${i}`}>
                <img 
                  src={`/api/cover/${album.id}`} 
                  alt={album.title} 
                  onerror={(e) => { 
                    const target = e.currentTarget;
                    if (target instanceof HTMLImageElement) target.style.opacity='0'; 
                  }} 
                />
                <div class="tag-badge" class:primary={album.tagColor === 'primary'} class:secondary={album.tagColor === 'secondary'} class:tertiary={album.tagColor === 'tertiary'}>
                  {album.tag}
                </div>
                <button class="album-overlay" onclick={() => onselectView?.('albums', album.id)}>
                   <div class="album-info">
                     <span class="a-title">{album.title}</span>
                     <span class="a-artist">{album.artist}</span>
                   </div>
                   <div class="play-indicator"><Icon name="play" size={14} /></div>
                </button>
              </div>
            {/each}
        </div>
      </div>
    </section>
    {/if}

    <!-- Trends Tile -->
    {#if stats.trends && stats.trends.length > 0}
    <section class="bento-item trends-tile">
      <div class="tile-header">
        <Icon name="flame" size={20} color="var(--tertiary)" />
        <h3>Trends</h3>
      </div>
      <div class="genre-bars">
        {#each stats.trends as t, i}
          <div class="bar-item">
            <div class="bar-info"><span>{t.genre}</span> <small>{t.percentage}%</small></div>
            <div class="progress">
              <div 
                class="fill" 
                class:sec={i === 1} 
                class:ter={i === 2} 
                style="width: {t.percentage}%"
              ></div>
            </div>
          </div>
        {/each}
      </div>
    </section>
    {/if}
  </div>

  <ContextMenu 
    bind:visible={contextMenu.visible}
    x={contextMenu.x}
    y={contextMenu.y}
    items={contextMenu.items}
    onclose={() => contextMenu.visible = false}
  />
</div>

<style>
  .dashboard { 
    padding: 2rem 0.5rem; 
    max-width: 100%;
    box-sizing: border-box;
    animation: fadeIn 0.5s ease;
  }
  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

  .bento-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-auto-rows: minmax(180px, auto);
    gap: 1.5rem;
    padding-top: 10px; /* Evita que el primer item se corte al subir con hover */
  }

  .bento-item {
    background: var(--surface-container);
    border-radius: 32px;
    padding: 1.8rem;
    border: 1px solid var(--border-color);
    transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    display: flex;
    flex-direction: column;
    overflow: visible; /* Cambiado de hidden para evitar cortes */
    text-align: left;
    color: inherit;
    font-family: inherit;
    position: relative;
  }
  .bento-item:not(button) { cursor: default; }
  .bento-item:hover { border-color: var(--primary-30); transform: translateY(-8px); box-shadow: 0 12px 24px rgba(0,0,0,0.2); z-index: 10; }

  .tile-header { display: flex; align-items: center; gap: 0.8rem; margin-bottom: 1.2rem; }
  .tile-header h3 { margin: 0; font-size: 1.1rem; font-weight: 800; letter-spacing: -0.2px; }

  /* Welcome Tile */
  .welcome-tile { grid-column: span 3; background: linear-gradient(135deg, var(--surface-container) 0%, var(--surface-container-high) 100%); overflow: hidden; }
  .welcome-tile::after { content: ''; position: absolute; top: -50%; right: -20%; width: 280px; height: 280px; background: var(--primary-10); filter: blur(60px); border-radius: 50%; z-index: 0; }
  .welcome-tile h1 { 
    font-size: clamp(2rem, 5vw, 3rem); 
    font-weight: 950; 
    letter-spacing: -0.04em; 
    margin: 0; 
    color: var(--text-main); 
  }
  .welcome-header { 
    display: flex; 
    flex-direction: row; 
    align-items: center; 
    justify-content: space-between;
    width: 100%;
    flex-wrap: nowrap;
    gap: 1.5rem; 
    margin-bottom: 2.5rem;
    position: relative; 
    z-index: 1; 
  }
  .welcome-tile h1 span { color: var(--primary); }

  .inbox-notification-badge { 
    background: var(--surface-container-highest); 
    color: var(--text-main); 
    border: 1px solid var(--border-color); 
    width: 42px;
    height: 42px;
    border-radius: 12px; 
    display: flex; 
    align-items: center; 
    justify-content: center; 
    cursor: pointer; 
    transition: all 0.2s; 
    position: relative; 
    box-shadow: none;
  }
  .inbox-notification-badge:hover { background: var(--surface-container-high); transform: scale(1.05); border-color: var(--primary); }
  
  .badge-number { 
    position: absolute; 
    top: -8px; 
    right: -8px; 
    background: #ff4d4f; 
    color: white;
    font-size: 0.65rem;
    font-weight: 950;
    min-width: 20px;
    height: 20px;
    padding: 0 4px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px solid var(--surface-container-high);
    box-shadow: none;
  }

  /* Top Plays Tile */
  .most-played-tile { grid-column: span 1; grid-row: span 2; }
  .top-list { 
    display: flex; 
    flex-direction: column; 
    gap: 0.5rem; 
    overflow-y: auto; 
    flex: 1; 
    padding-right: 0.4rem; 
    overflow-x: hidden;
    scrollbar-width: none;
    -ms-overflow-style: none;
  }
  .top-list::-webkit-scrollbar { display: none; }
  .top-item { background: transparent; border: none; display: flex; align-items: center; gap: 0.8rem; padding: 0.5rem; border-radius: 16px; cursor: pointer; transition: background 0.2s; text-align: left; width: 100%; color: inherit; }
  .top-item:hover { background: var(--surface-container-highest); }
  .top-item img { width: 40px; height: 40px; border-radius: 8px; object-fit: cover; }
  .top-info { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
  .top-info .title { font-weight: 800; font-size: 0.85rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--text-main); }
  .top-info .artist { font-size: 0.7rem; color: var(--text-muted); font-weight: 700; }
  .play-count { font-size: 0.8rem; font-weight: 950; color: var(--secondary); text-align: right; line-height: 1.1; }
  .play-count small { font-size: 0.55rem; color: var(--text-muted); display: block; }

  /* Gems Tile */
  .gems-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.6rem; overflow: visible; }
  .gems-grid.lone-gem { grid-template-columns: 1fr; }
  .gems-grid.few-gems { grid-template-columns: repeat(2, 1fr); }
  
  .gem-cover { background: var(--surface-container-highest); padding: 0; border: 1px solid var(--border-color); aspect-ratio: 1/1; border-radius: 16px; overflow: hidden; cursor: pointer; position: relative; }
  .gems-grid.lone-gem .gem-cover { aspect-ratio: 16/9; max-height: 280px; }
  .gem-cover img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.3s; }
  .gem-cover:hover img { transform: scale(1.1); }
  .gem-cover .overlay { position: absolute; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity 0.2s; backdrop-filter: blur(2px); }
  .gem-cover:hover .overlay { opacity: 1; }

  /* Discovery Mosaic */
  .discovery-tile { grid-column: span 2; grid-row: span 2; }
  .mosaic-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.6rem; flex: 1; }
  .mosaic-item { position: relative; border-radius: 16px; overflow: hidden; background: var(--surface-container-highest); border: 1px solid rgba(255,255,255,0.05); aspect-ratio: 1 / 1; }
  .mosaic-item img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.4s; }
  .mosaic-item:hover img { transform: scale(1.1); }
  .mosaic-item.i0 { grid-column: span 2; grid-row: span 2; }
  .mosaic-item.i1 { grid-column: span 1; grid-row: span 1; }
  .mosaic-item.i2 { grid-column: span 1; grid-row: span 1; }
  .mosaic-item.i3 { grid-column: span 1; grid-row: span 1; }
  .mosaic-item.i4 { grid-column: span 1; grid-row: span 1; }
  .mosaic-item.i5 { grid-column: span 2; grid-row: span 2; }

  .tag-badge {
    position: absolute;
    top: 1rem;
    left: 1rem;
    padding: 0.4rem 0.9rem;
    border-radius: 100px;
    font-size: 0.65rem;
    font-weight: 950;
    z-index: 4;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    box-shadow: 0 4px 12px rgba(0,0,0,0.4);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .tag-badge.primary { background: var(--primary); color: #fff; border-color: rgba(255,255,255,0.2); }
  .tag-badge.secondary { background: var(--secondary); color: #fff; border-color: rgba(255,255,255,0.2); }
  .tag-badge.tertiary { background: var(--tertiary); color: #fff; border-color: rgba(255,255,255,0.2); }

  .album-overlay {
    position: absolute;
    inset: 0;
    background: linear-gradient(to top, rgba(0,0,0,0.9), transparent 60%);
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    padding: 1.2rem;
    opacity: 0;
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
    border: none;
    color: white;
    cursor: pointer;
    text-align: left;
    z-index: 5;
    transform: translateY(10px);
  }
  .mosaic-item:hover .album-overlay { opacity: 1; transform: translateY(0); }
  
  .album-info { display: flex; flex-direction: column; overflow: hidden; margin-right: 1rem; }
  .a-title { font-weight: 900; font-size: 0.9rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; letter-spacing: -0.2px; }
  .a-artist { font-size: 0.75rem; opacity: 0.7; font-weight: 700; }
  
  .play-indicator { 
    width: 32px; height: 32px; 
    background: white; color: black; 
    border-radius: 50%; 
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
    box-shadow: 0 4px 12px rgba(0,0,0,0.5);
  }

  .overlay-chip { position: absolute; padding: 0.4rem 0.8rem; border-radius: 100px; font-size: 0.65rem; font-weight: 800; border: 1px solid rgba(255,255,255,0.1); color: white; cursor: pointer; backdrop-filter: blur(10px); background: rgba(0,0,0,0.5); transition: all 0.2s; z-index: 5; }
  .overlay-chip:hover { transform: scale(1.05); background: var(--primary); border-color: white; }

  .chip { background: var(--surface-container-highest); border: 1px solid var(--border-color); color: var(--text-main); font-size: 0.75rem; font-weight: 800; padding: 0.45rem 1rem; border-radius: 100px; cursor: pointer; transition: all 0.2s; white-space: nowrap; display: flex; align-items: center; gap: 0.5rem; }
  .chip:hover { border-color: var(--primary); transform: translateY(-2px); box-shadow: 0 4px 12px rgba(0,0,0,0.2); }
  .chip.primary { background: var(--primary-10); color: var(--primary); }
  .chip.secondary { background: var(--secondary-10); color: var(--secondary); }
  .chip.tertiary { background: var(--tertiary-10); color: var(--tertiary); }

  .welcome-chips { margin-top: auto; padding-top: 1.5rem; display: flex; gap: 0.5rem; flex-wrap: wrap; position: relative; z-index: 1; padding-right: 180px; }

  .welcome-scan-status {
    position: absolute;
    bottom: 1.5rem;
    right: 1.8rem;
    background: linear-gradient(
      110deg, 
      var(--surface-container-highest) 30%, 
      rgba(var(--primary-rgb), 0.2) 50%, 
      var(--surface-container-highest) 70%
    );
    background-size: 200% 100%;
    border: 1px solid var(--primary-20);
    padding: 0.6rem 1.2rem;
    border-radius: 20px;
    display: flex;
    align-items: center;
    gap: 1rem;
    z-index: 5;
    animation: slideIn 0.3s cubic-bezier(0.2, 0, 0, 1), shimmer 2s infinite linear;
    box-shadow: 0 8px 16px rgba(0,0,0,0.2);
    overflow: hidden;
  }

  @keyframes shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }

  @keyframes slideIn {
    from { opacity: 0; transform: translateX(20px); }
    to { opacity: 1; transform: translateX(0); }
  }

  .welcome-spinner {
    width: 18px;
    height: 18px;
    border: 2px solid var(--primary-20);
    border-top-color: var(--primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin { to { transform: rotate(360deg); } }

  .scan-info {
    display: flex;
    flex-direction: column;
    line-height: 1.2;
  }

  .scan-info .label {
    font-size: 0.7rem;
    font-weight: 800;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .scan-info .count {
    font-size: 0.85rem;
    font-weight: 700;
    color: var(--text-main);
  }

  .genre-bars { display: flex; flex-direction: column; gap: 0.8rem; }
  .bar-item { display: flex; flex-direction: column; gap: 0.3rem; }
  .bar-info { display: flex; justify-content: space-between; font-size: 0.75rem; font-weight: 800; }
  .progress { height: 6px; background: var(--surface-container-highest); border-radius: 10px; overflow: hidden; }
  .fill { height: 100%; background: var(--primary); border-radius: 10px; }

  .health-summary { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 0.8rem; }
  .h-row { display: flex; justify-content: space-between; font-size: 0.78rem; font-weight: 700; color: var(--text-muted); }
  .h-row .val { color: var(--text-main); font-weight: 950; }
  .maint-btn { margin-top: auto; background: var(--surface-container-highest); border: 1px solid var(--border-color); color: var(--text-muted); padding: 0.6rem; border-radius: 16px; font-weight: 800; font-size: 0.7rem; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 0.4rem; transition: all 0.2s; }
  .maint-btn:hover { border-color: var(--primary); color: var(--primary); }

  @media (max-width: 1400px) {
    .bento-grid { grid-template-columns: repeat(3, 1fr); }
    .welcome-tile { grid-column: span 2; }
  }
  @media (max-width: 1100px) {
    .bento-grid { grid-template-columns: repeat(2, 1fr); }
    .welcome-tile { grid-column: span 2; }
  }
  @media (max-width: 768px) {
    .bento-grid { grid-template-columns: 1fr; }
    .welcome-tile { grid-column: span 1; }
    .bento-item { padding: 1.2rem; border-radius: 24px; }
  }
</style>
