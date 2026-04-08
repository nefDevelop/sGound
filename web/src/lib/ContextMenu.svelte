<script>
  import { onMount } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import Icon from './Icon.svelte';

  let { x = 0, y = 0, items = [], visible = $bindable(false), onclose } = $props();

  let menuElement = $state();
  let adjustedX = $state(0);
  let adjustedY = $state(0);

  $effect(() => {
    adjustedX = x;
    adjustedY = y;
  });

  onMount(() => {
    const handleClick = (e) => {
      if (visible && menuElement && !menuElement.contains(e.target)) {
        visible = false;
        onclose?.();
      }
    };
    window.addEventListener('click', handleClick);
    return () => window.removeEventListener('click', handleClick);
  });

  $effect(() => {
    if (visible && menuElement) {
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
  });

  function handleItemClick(item) {
    if (item.action) item.action();
    visible = false;
    onclose?.();
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
          <li class="divider" role="separator"></li>
        {:else}
          <li role="none">
            <button 
              type="button"
              class="menu-item" 
              class:danger={item.color === 'danger'}
              onclick={(e) => { e.stopPropagation(); handleItemClick(item); }}
              role="menuitem"
            >
              <div class="icon">
                <Icon name={item.icon} size={18} />
              </div>
              <span class="label">{item.label}</span>
            </button>
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
    width: 100%;
    border: none;
    background: transparent;
    text-align: left;
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
