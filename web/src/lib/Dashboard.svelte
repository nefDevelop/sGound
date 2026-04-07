<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';
  
  const dispatch = createEventDispatcher();
  let stats = {
    totalTracks: 0,
    totalAlbums: 0,
    totalArtists: 0,
    totalPlaylists: 0,
    forgottenGems: []
  };
  let health = { missingTags: 0, missingLyrics: 0 };
  let loading = true;

  onMount(async () => {
    try {
      const [sRes, hRes] = await Promise.all([
        fetch('/api/dashboard'),
        fetch('/api/library/health')
      ]);
      stats = await sRes.json();
      health = await hRes.json();
    } catch (e) { console.error(e); }
    finally { loading = false; }
  });
</script>

<div class="dashboard">
  <header class="dash-header">
    <h1>Welcome back to <span>Sgound</span></h1>
    <div class="chips-row">
      <div class="chip"><Icon name="music" size={14} color="var(--primary)" /> {stats.totalTracks} Songs</div>
      <div class="chip"><Icon name="disc" size={14} color="var(--primary)" /> {stats.totalAlbums} Albums</div>
      <div class="chip"><Icon name="user" size={14} color="var(--primary)" /> {stats.totalArtists} Artists</div>
      <div class="chip"><Icon name="list" size={14} color="var(--primary)" /> {stats.totalPlaylists} Playlists</div>
    </div>
  </header>

  <div class="dash-grid">
    <!-- Joyas Olvidadas -->
    <section class="dash-card gems-card">
      <div class="card-header">
        <h3><Icon name="sparkles" size={18} color="var(--tertiary)" /> Forgotten Gems</h3>
        <button class="text-btn">Shuffle All</button>
      </div>
      <div class="gems-list">
        {#each stats.forgottenGems as gem}
          <!-- svelte-ignore a11y-click-events-have-key-events -->
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <div class="gem-item" on:click={() => dispatch('play', gem)}>
            <img src={`/api/cover/${gem.id}`} alt="" on:error={(e) => (e.currentTarget.style.display='none')} />
            <div class="gem-info">
              <span class="title">{gem.title}</span>
              <span class="artist">{gem.artist}</span>
            </div>
          </div>
        {/each}
      </div>
    </section>

    <!-- Saneamiento de Biblioteca -->
    <section class="dash-card health-card">
      <div class="card-header">
        <h3><Icon name="shield" size={18} color="var(--secondary)" /> Library Health</h3>
      </div>
      <div class="health-stats">
        <div class="health-item" class:warning={health.missingTags > 0}>
          <span class="val">{health.missingTags}</span>
          <span class="lbl">Missing Genre/Year</span>
          <button class="fix-btn">Fix Now</button>
        </div>
        <div class="health-item">
          <span class="val">{health.missingLyrics}</span>
          <span class="lbl">Missing Lyrics</span>
          <button class="fix-btn">Auto-Download</button>
        </div>
      </div>
    </section>

    <!-- Categorías Más Escuchadas -->
    <section class="dash-card charts-card">
      <div class="card-header">
        <h3><Icon name="flame" size={18} color="var(--primary)" /> Top Genres</h3>
      </div>
      <div class="genres-pills">
        <div class="pill-bar" style="width: 80%">Rock <span class="perc">80%</span></div>
        <div class="pill-bar" style="width: 60%; background: var(--secondary)">Electronic <span class="perc">60%</span></div>
        <div class="pill-bar" style="width: 40%; background: var(--tertiary)">Jazz <span class="perc">40%</span></div>
      </div>
    </section>
  </div>
</div>

<style>
  .dashboard { padding: 1rem 0; animation: fadeIn 0.5s ease; }
  @keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

  .dash-header { margin-bottom: 3.5rem; }
  .dash-header h1 { 
    font-size: var(--font-size-h1); 
    font-weight: var(--font-weight-h1); 
    letter-spacing: var(--letter-spacing-h1); 
    margin-bottom: 1.5rem; 
  }
  .dash-header h1 span { color: var(--primary); }

  .chips-row { display: flex; gap: 1rem; flex-wrap: wrap; }
  .chip {
    background: var(--surface-container-high); 
    color: var(--text-main); 
    padding: 0.7rem 1.4rem;
    border-radius: 100px; 
    font-weight: 700; 
    font-size: 0.9rem;
    border: 1px solid var(--border-color); 
    display: flex; 
    align-items: center; 
    gap: 0.6rem;
    transition: transform 0.2s;
  }
  .chip:hover { transform: scale(1.05); background: var(--primary); color: white; }

  .dash-grid {
    display: grid; grid-template-columns: 1fr 350px; grid-template-rows: auto auto; gap: 2rem;
  }

  .dash-card {
    background: var(--surface-container-high); 
    border-radius: var(--radius); 
    padding: 2.5rem;
    border: 1px solid var(--border-color);
  }

  .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem; }
  .card-header h3 { 
    margin: 0; 
    font-size: 1.4rem; 
    font-weight: 800; 
    font-family: var(--font-headline);
  }

  .gems-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 1.2rem; }
  .gem-item {
    background: var(--surface); 
    padding: 1.2rem; 
    border-radius: calc(var(--radius) / 1.5);
    display: flex; 
    align-items: center; 
    gap: 1.2rem; 
    cursor: pointer; 
    transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
    border: 1px solid var(--border-color);
  }
  .gem-item:hover { border-color: var(--primary); transform: scale(1.03); box-shadow: 0 10px 25px rgba(0,0,0,0.2); }
  
  .gem-item img { 
    width: 56px; 
    height: 56px; 
    border-radius: 12px; 
    object-fit: cover;
    box-shadow: 0 4px 8px rgba(0,0,0,0.3);
  }
  
  .gem-info { display: flex; flex-direction: column; overflow: hidden; gap: 0.2rem; }
  .gem-info .title { font-weight: 800; font-size: 1rem; color: var(--text-main); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .gem-info .artist { font-size: 0.85rem; color: var(--text-muted); font-weight: 600; }

  .health-stats { display: flex; flex-direction: column; gap: 1.5rem; }
  .health-item {
    background: var(--surface); 
    padding: 1.8rem; 
    border-radius: calc(var(--radius) / 1.5);
    display: flex; 
    flex-direction: column; 
    align-items: center; 
    text-align: center; 
    gap: 0.8rem;
    border: 1px solid var(--border-color);
  }
  .health-item.warning { border-color: var(--secondary); }
  .health-item .val { font-size: 2.5rem; font-weight: 900; color: var(--primary); font-family: var(--font-headline); }
  .health-item .lbl { font-size: 0.9rem; color: var(--text-muted); font-weight: 700; }
  
  .fix-btn {
    background: var(--surface-container-high); 
    color: var(--text-main); 
    border: 1px solid var(--border-color); 
    padding: 0.6rem 1.5rem;
    border-radius: 100px; 
    font-weight: 800; 
    font-size: 0.8rem; 
    cursor: pointer;
    transition: all 0.2s;
  }
  .fix-btn:hover { background: var(--primary); color: white; border-color: var(--primary); }

  .genres-pills { display: flex; flex-direction: column; gap: 1rem; }
  .pill-bar {
    background: var(--primary); 
    color: white; 
    padding: 0.9rem 1.5rem; 
    border-radius: 100px;
    font-weight: 800; 
    font-size: 0.95rem; 
    display: flex; 
    justify-content: space-between;
    box-shadow: 0 4px 10px var(--primary-10);
  }
  .perc { opacity: 0.8; font-family: monospace; }

  .text-btn { background: transparent; border: none; color: var(--primary); font-weight: 800; cursor: pointer; text-transform: uppercase; letter-spacing: 1px; font-size: 0.8rem; }
  .text-btn:hover { text-decoration: underline; }
</style>
