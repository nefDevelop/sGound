<script>
  /** @type {{ src: string, alt?: string, size?: string, borderRadius?: string, onclick?: (e: MouseEvent) => void }} */
  let { src, alt = "Cover", size = "100%", borderRadius = "24px", onclick } = $props();
  
  let loaded = $state(false);
  let error = $state(false);

  function handleLoad() {
    loaded = true;
  }

  function handleError() {
    error = true;
    loaded = true;
  }
</script>

<div 
  class="artwork-container" 
  style:width={size} 
  style:height={size} 
  style:border-radius={borderRadius}
  onclick={onclick}
  role="presentation"
>
  {#if !loaded}
    <div class="skeleton" style:border-radius={borderRadius}></div>
  {/if}

  <img 
    {src} 
    {alt}
    class:loaded
    loading="lazy"
    onload={handleLoad}
    onerror={handleError}
  />

  {#if error}
    <div class="error-fallback" style:border-radius={borderRadius}>
       <span class="fallback-text">{alt.charAt(0).toUpperCase()}</span>
    </div>
  {/if}
</div>

<style>
  .artwork-container {
    position: relative;
    overflow: hidden;
    background: var(--surface-container-low);
    aspect-ratio: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .skeleton {
    position: absolute;
    top: 0; left: 0; width: 100%; height: 100%;
    background: linear-gradient(90deg, 
      var(--surface-container) 0%, 
      var(--surface-container-high) 50%, 
      var(--surface-container) 100%);
    background-size: 200% 100%;
    animation: pulse 1.5s infinite linear;
    z-index: 1;
  }

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    opacity: 0;
    transition: opacity 0.6s cubic-bezier(0.4, 0, 0.2, 1);
  }

  img.loaded {
    opacity: 1;
  }


  .error-fallback {
    position: absolute;
    top: 0; left: 0; width: 100%; height: 100%;
    background: var(--surface-container-highest);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    font-weight: 800;
    font-size: 2rem;
  }

  @keyframes pulse {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }
</style>
