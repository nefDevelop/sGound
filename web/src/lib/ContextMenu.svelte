<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import Icon from './Icon.svelte';

  export let x = 0;
  export let y = 0;
  export let items = []; // { label, icon, action, color }
  export let visible = false;

  const dispatch = createEventDispatcher();
  let menuElement;

  onMount(() => {
    const handleClick = (e) => {
      if (visible && menuElement && !menuElement.contains(e.target)) {
        dispatch('close');
      }
    };
    window.addEventListener('click', handleClick);
    return () => window.removeEventListener('click', handleClick);
  });

  // Ajustar posición para que no se salga de la pantalla
  let adjustedX = x;
  let adjustedY = y;

  $: if (visible && menuElement) {
    const rect = menuElement.getBoundingClientRect();
    const padding = 10;
    
    if (x + rect.width > window.innerWidth) {
      adjustedX = window.innerWidth - rect.width - padding;
    } else {
      adjustedX = x;
    }

    if (y + rect.height > window.innerHeight) {
      adjustedY = window.innerHeight - rect.height - padding;
    } else {
      adjustedY = y;
    }
  }

  function handleItemClick(item) {
    if (item.action) item.action();
    dispatch('close');
  }
</script>

{#if visible}
  <div 
    class="context-menu"
    bind:this={menuElement}
    style="left: {adjustedX}px; top: {adjustedY}px;"
    in:scale={{ duration: 150, start: 0.95 }}
    out:fade={{ duration: 100 }}
  >
    <ul class="menu-list">
      {#each items as item}
        {#if item.divider}
          <li class="divider"></li>
        {:else}
          <li 
            class="menu-item" 
            class:danger={item.color === 'danger'}
            on:click|stopPropagation={() => handleItemClick(item)}
          >
            <div class="icon">
              <Icon name={item.icon} size={18} />
            </div>
            <span class="label">{item.label}</span>
          </li>
        {/if}
      {/each}
    </ul>
  </div>
{/if}

<style>
  .context-menu {
    position: fixed;
    z-index: 10000;
    min-width: 240px;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    border-radius: 16px;
    box-shadow: 0 20px 40px rgba(0,0,0,0.4), 0 0 0 1px rgba(255,255,255,0.05);
    overflow: hidden;
    padding: 8px;
  }

  .menu-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 10px 16px;
    color: var(--text-main);
    font-size: 0.95rem;
    border-radius: 10px;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
    user-select: none;
    font-weight: 600;
  }

  .menu-item:hover {
    background: var(--primary);
    color: #ffffff;
  }

  .menu-item.danger {
    color: #f87171;
  }

  .menu-item.danger:hover {
    background: #ef4444;
    color: white;
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.8;
    width: 20px;
  }

  .menu-item:hover .icon {
    opacity: 1;
    color: white;
  }

  .label {
    flex: 1;
  }

  .divider {
    height: 1px;
    background: var(--border-color);
    margin: 6px 8px;
  }
</style>
