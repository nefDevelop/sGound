<script>
  import { onMount } from 'svelte';
  import { 
    queue, currentIndex, effectiveQuality, isPlaying, 
    volume, isShuffle, repeatMode 
  } from './stores.js';
  import { get } from 'svelte/store';
  
  let { 
    audioRef = $bindable(null),
    progress = $bindable(0),
    duration = $bindable(0),
    currentTime = $bindable(0),
    onSkipNext = $bindable(),
    onSkipBack = $bindable()
  } = $props();

  // Dual audio elements for gapless
  let audioA = $state(), audioB = $state();
  let activeAudio = $state('A');
  let preloadedId = $state(null);

  // Expose methods
  onSkipNext = skipNext;
  onSkipBack = skipBack;

  let currentPlaying = $derived($currentIndex >= 0 && $currentIndex < $queue.length ? $queue[$currentIndex] : null);

  onMount(() => {
    audioRef = audioA;
    setupAudio(audioA);
    setupAudio(audioB);
    audioA.volume = $volume;
    audioB.volume = $volume;
  });

  function setupAudio(el) {
    el.onended = handleTrackEnded;
    el.ontimeupdate = () => {
      if (el === audioRef) {
        currentTime = el.currentTime;
        duration = el.duration;
        progress = (currentTime / duration) * 100 || 0;
        checkPreload();
      }
    };
    el.onplay = () => isPlaying.set(true);
    el.onpause = () => isPlaying.set(false);
    
    // Sync volume when it changes globally
    volume.subscribe(v => {
      if (el) el.volume = v;
    });
  }

  function checkPreload() {
    if (duration > 0 && currentTime / duration > 0.85) {
      const q = get(queue);
      const idx = get(currentIndex);
      if (idx < q.length - 1) {
        const next = q[idx + 1];
        if (next && next.id !== preloadedId) {
          const nextEl = activeAudio === 'A' ? audioB : audioA;
          const quality = get(effectiveQuality);
          const url = `/api/stream/${next.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
          nextEl.src = url;
          nextEl.load();
          preloadedId = next.id;
        }
      }
    }
  }

  function handleTrackEnded() {
     const q = get(queue);
     const idx = get(currentIndex);
     const mode = get(repeatMode);
     const shuffle = get(isShuffle);

     if (mode === 'one') {
        audioRef.currentTime = 0;
        audioRef.play().catch(() => {});
        return;
     }

     if (shuffle && q.length > 1) {
        let nextIdx;
        do {
          nextIdx = Math.floor(Math.random() * q.length);
        } while (nextIdx === idx);
        currentIndex.set(nextIdx);
        switchAudio();
        return;
     }

     if (idx < q.length - 1) {
        currentIndex.set(idx + 1);
        switchAudio();
     } else {
        if (mode === 'all') {
           currentIndex.set(0);
           switchAudio();
        } else {
           isPlaying.set(false);
        }
     }
  }

  function switchAudio() {
    const prevAudio = activeAudio === 'A' ? audioA : audioB;
    activeAudio = activeAudio === 'A' ? 'B' : 'A';
    audioRef = activeAudio === 'A' ? audioA : audioB;
    
    prevAudio.pause();
    // Wait for the new audio element to be ready if needed
    audioRef.play().catch((e) => console.error("Play failed", e));
    preloadedId = null;
  }

  // Effect to handle track changes (when triggered from outside, e.g. clicking an album)
  $effect(() => {
    if (currentPlaying && audioRef && audioRef.src.indexOf(currentPlaying.id) === -1 && !preloadedId) {
      const quality = get(effectiveQuality);
      const url = `/api/stream/${currentPlaying.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.src = url;
      audioRef.play().catch(() => {});
    }
  });

  function skipNext() {
    const q = get(queue);
    const idx = get(currentIndex);
    const shuffle = get(isShuffle);

    let nextIdx = idx + 1;
    if (shuffle && q.length > 1) {
       do {
         nextIdx = Math.floor(Math.random() * q.length);
       } while (nextIdx === idx);
    }

    if (nextIdx < q.length && nextIdx >= 0) {
      currentIndex.set(nextIdx);
      const next = q[nextIdx];
      if (preloadedId === next.id) {
         switchAudio();
      } else {
         const quality = get(effectiveQuality);
         audioRef.src = `/api/stream/${next.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
         audioRef.play().catch(() => {});
         preloadedId = null;
      }
    } else if (get(repeatMode) === 'all' && q.length > 0) {
      currentIndex.set(0);
      switchAudio();
    }
  }

  function skipBack() {
    const idx = get(currentIndex);
    if (idx > 0) {
      currentIndex.set(idx - 1);
      const q = get(queue);
      const prev = q[idx-1];
      const quality = get(effectiveQuality);
      audioRef.src = `/api/stream/${prev.id}` + (quality !== "original" ? `?bitrate=${quality}` : "");
      audioRef.play().catch(() => {});
      preloadedId = null;
    }
  }
</script>

<audio bind:this={audioA} style="display:none"></audio>
<audio bind:this={audioB} style="display:none"></audio>
