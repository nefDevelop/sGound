<script>
  import Icon from './Icon.svelte';
  
  let { 
    rating = $bindable(0), 
    interactive = true, 
    size = 16 
  } = $props();
  
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  let hoverRating = $state(0);

  function setRating(val) {
    if (!interactive) return;
    rating = val;
    dispatch('change', rating);
  }

  function handleMouseEnter(val) {
    if (!interactive) return;
    hoverRating = val;
  }

  function handleMouseLeave() {
    hoverRating = 0;
  }
</script>

<!-- svelte-ignore a11y_mouse_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div 
    class="star-container" 
    class:interactive 
    onmouseleave={handleMouseLeave}
    style="--star-size: {size}px"
>
  {#each [1, 2, 3, 4, 5] as star}
    <button 
      class="star-btn"
      class:filled={star <= (hoverRating || rating)}
      onclick={() => setRating(star)}
      onmouseenter={() => handleMouseEnter(star)}
      type="button"
    >
      <Icon name="award" size={size} />
    </button>
  {/each}
</div>

<style>
  .star-container {
    display: flex;
    gap: 2px;
    align-items: center;
  }

  .star-btn {
    background: none;
    border: none;
    padding: 0;
    cursor: default;
    color: var(--text-muted);
    opacity: 0.3;
    transition: all 0.1s ease;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .interactive .star-btn {
    cursor: pointer;
  }

  .star-btn.filled {
    color: var(--primary);
    opacity: 1;
    filter: drop-shadow(0 0 4px var(--primary-10));
    transform: scale(1.1);
  }

  .interactive .star-btn:hover {
    transform: scale(1.2);
  }
</style>
