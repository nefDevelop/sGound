<script>
  import Icon from './Icon.svelte';
  import { onMount } from 'svelte';

  let { pattern = $bindable(), availableTags = ['artist', 'album', 'title', 'year', 'genre', 'track', 'bitrate'] } = $props();

  let tokens = $state([]);
  let dragIndex = $state(null);

  // Parse pattern string into tokens
  function parsePattern(p) {
    if (!p) return [];
    // We want to extract {tags}, / (as folders), and remaining text
    const regex = /\{([^}]+)\}|(\/)|([^{/]+)/g;
    let match;
    const newTokens = [];
    while ((match = regex.exec(p)) !== null) {
      if (match[1]) {
        newTokens.push({ id: Math.random(), type: 'tag', value: match[1] });
      } else if (match[2]) {
        newTokens.push({ id: Math.random(), type: 'folder', value: '/' });
      } else {
        newTokens.push({ id: Math.random(), type: 'text', value: match[3] });
      }
    }
    return newTokens;
  }

  // Update pattern string from tokens
  function updatePattern() {
    pattern = tokens.map(t => {
      if (t.type === 'tag') return `{${t.value}}`;
      return t.value;
    }).join('');
  }

  onMount(() => {
    tokens = parsePattern(pattern);
  });

  // Watch for external pattern changes (only if it doesn't match our tokens)
  $effect(() => {
    const currentPattern = tokens.map(t => t.type === 'tag' ? `{${t.value}}` : t.value).join('');
    if (pattern !== currentPattern) {
      tokens = parsePattern(pattern);
    }
  });

  function addTag(tag) {
    tokens.push({ id: Math.random(), type: 'tag', value: tag });
    updatePattern();
  }

  function addText() {
    tokens.push({ id: Math.random(), type: 'text', value: ' - ' });
    updatePattern();
  }

  function addFolder() {
    tokens.push({ id: Math.random(), type: 'folder', value: '/' });
    updatePattern();
  }

  function removeToken(index) {
    tokens.splice(index, 1);
    updatePattern();
  }

  function handleDragStart(index) {
    dragIndex = index;
  }

  function handleDragOver(e, index) {
    e.preventDefault();
    if (dragIndex === null || dragIndex === index) return;
    
    const draggedItem = tokens[dragIndex];
    tokens.splice(dragIndex, 1);
    tokens.splice(index, 0, draggedItem);
    dragIndex = index;
    updatePattern();
  }

  function handleDragEnd() {
    dragIndex = null;
  }

  function handleTextChange(index, e) {
    tokens[index].value = e.target.value;
    updatePattern();
  }
</script>

<div class="pattern-builder">
  <div class="pattern-dropzone">
    <div class="tokens-container">
      {#each tokens as token, i (token.id)}
        <div 
          class="token {token.type}"
          draggable="true"
          ondragstart={() => handleDragStart(i)}
          ondragover={(e) => handleDragOver(e, i)}
          ondragend={handleDragEnd}
          role="listitem"
          aria-grabbed={dragIndex === i}
          class:dragging={dragIndex === i}
        >
          <div class="drag-handle">
            <Icon name="menu" size={12} />
          </div>

          {#if token.type === 'tag'}
            <span class="tag-label">{token.value}</span>
          {:else if token.type === 'folder'}
            <div class="folder-badge">
              <Icon name="folder" size={14} />
              <span>/</span>
            </div>
          {:else}
            <input 
              type="text" 
              value={token.value} 
              oninput={(e) => handleTextChange(i, e)}
              style="width: {Math.max(token.value.length, 1) + 1}ch"
              class="text-input"
              placeholder="..."
            />
          {/if}

          <button class="remove-btn" onclick={() => removeToken(i)} aria-label="Remove token">
            <Icon name="x" size={10} />
          </button>
        </div>
      {/each}
      
      {#if tokens.length === 0}
        <p class="placeholder">Add tags or text to build your pattern...</p>
      {/if}
    </div>
  </div>

  <div class="palette-zone">
    <div class="palette-group">
      <h4>Placeholders</h4>
      <div class="palette-chips">
        {#each availableTags as tag}
          <button class="chip primary" onclick={() => addTag(tag)}>
            <Icon name="plus" size={14} />
            {tag}
          </button>
        {/each}
      </div>
    </div>

    <div class="palette-group">
      <h4>Structure & Text</h4>
      <div class="palette-chips">
        <button class="chip secondary" onclick={addFolder}>
          <Icon name="folderPlus" size={14} />
          New Folder
        </button>
        <button class="chip tertiary" onclick={addText}>
          <Icon name="plus" size={14} />
          Custom Text
        </button>
      </div>
    </div>
  </div>
</div>

<style>
  .pattern-builder {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    width: 100%;
  }

  .pattern-dropzone {
    background: var(--surface-container-high);
    border: 2px dashed var(--border-color);
    border-radius: 20px;
    padding: 1.5rem;
    min-height: 80px;
    display: flex;
    align-items: center;
    transition: all 0.2s;
  }

  .pattern-dropzone:focus-within {
    border-color: var(--primary);
    background: var(--surface-container-highest);
  }

  .tokens-container {
    display: flex;
    flex-wrap: wrap;
    gap: 0.8rem;
    align-items: center;
    width: 100%;
  }

  .placeholder {
    color: var(--text-muted);
    font-size: 0.9rem;
    margin: 0;
    font-style: italic;
  }

  .token {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.6rem;
    border-radius: 14px;
    background: var(--surface-container-highest);
    border: 1px solid var(--border-color);
    box-shadow: 0 4px 15px rgba(0,0,0,0.1);
    user-select: none;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    cursor: default;
  }

  .token:hover {
    transform: translateY(-3px) scale(1.02);
    box-shadow: 0 12px 30px rgba(0,0,0,0.3);
    border-color: var(--primary);
  }

  .token:active {
    transform: scale(0.92);
    background: var(--primary-20);
    box-shadow: 0 2px 10px rgba(0,0,0,0.1);
  }

  .token.tag {
    background: var(--primary-10);
    border-color: var(--primary-30);
    color: var(--primary);
  }

  .token.folder {
    background: var(--secondary-10);
    border-color: var(--secondary-30);
    color: var(--secondary);
  }

  .folder-badge {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-weight: 900;
    font-size: 1.1rem;
  }

  .token.text {
    background: var(--surface-container-low);
  }

  .token.dragging {
    opacity: 0.4;
    transform: scale(0.9);
    pointer-events: none;
  }

  .drag-handle {
    cursor: grab;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    padding: 2px;
    border-radius: 4px;
  }

  .drag-handle:hover {
    background: rgba(255,255,255,0.1);
    color: var(--text-main);
  }

  .drag-handle:active {
    cursor: grabbing;
  }

  .tag-label {
    font-weight: 800;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .text-input {
    background: var(--surface-container-lowest);
    border: 1px solid transparent;
    color: var(--text-main);
    font-weight: 700;
    font-size: 0.9rem;
    padding: 0.2rem 0.4rem;
    border-radius: 6px;
    width: auto;
    min-width: 2ch;
    text-align: center;
    outline: none;
    font-family: 'JetBrains Mono', monospace;
    transition: all 0.2s;
  }

  .text-input:focus {
    background: white;
    color: black;
    border-color: var(--primary);
    box-shadow: 0 0 10px var(--primary-20);
  }

  .remove-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    border-radius: 50%;
    transition: all 0.2s;
  }

  .remove-btn:hover {
    background: #ff555522;
    color: #ff5555;
    transform: rotate(90deg);
  }

  .palette-zone {
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
  }

  .palette-group h4 {
    margin: 0 0 0.8rem 0;
    font-size: 0.75rem;
    text-transform: uppercase;
    color: var(--text-muted);
    letter-spacing: 1px;
    font-weight: 800;
  }

  .palette-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.8rem;
  }

  .chip {
    border: none;
    padding: 0.6rem 1.2rem;
    border-radius: 100px;
    font-weight: 700;
    font-size: 0.85rem;
    cursor: pointer;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .chip.primary {
    background: var(--primary-10);
    color: var(--primary);
  }

  .chip.secondary {
    background: var(--secondary-10);
    color: var(--secondary);
  }

  .chip.tertiary {
    background: var(--tertiary-10);
    color: var(--tertiary);
  }

  .chip:hover {
    filter: brightness(1.2);
    transform: translateY(-2px);
  }
</style>
