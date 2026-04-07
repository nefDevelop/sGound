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
    min-width: 220px;
    background: rgba(30, 41, 59, 0.8);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5);
    overflow: hidden;
    padding: 6px;
  }

  .menu-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    color: #e2e8f0;
    font-size: 0.9rem;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;
    user-select: none;
  }

  .menu-item:hover {
    background: rgba(255, 255, 255, 0.1);
    color: white;
    transform: translateX(4px);
  }

  .menu-item.danger {
    color: #f87171;
  }

  .menu-item.danger:hover {
    background: rgba(239, 68, 68, 0.2);
    color: #fca5a5;
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.7;
    width: 20px;
  }

  .menu-item:hover .icon {
    opacity: 1;
  }

  .label {
    flex: 1;
    font-weight: 500;
  }

  .divider {
    height: 1px;
    background: rgba(255, 255, 255, 0.05);
    margin: 4px 6px;
  }
</style>
