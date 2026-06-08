<script>
  let {
    audioRef,
    progress,
    duration,
    currentTime
  } = $props();

  function formatTime(secs) {
    if (isNaN(secs)) return "0:00";
    const mins = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${mins}:${s.toString().padStart(2, '0')}`;
  }

  function handleSeek(e) {
    if (!audioRef || !duration || isNaN(duration)) return;
    const seekTime = (parseFloat(e.target.value) / 100) * duration;
    audioRef.currentTime = seekTime;
  }
</script>

<div class="timeline-container">
  <div class="modern-slider progress-slider">
      <input type="range" class="knobless-range" min="0" max="100" value={progress} oninput={handleSeek} />
      <div class="slider-fill" style="width: {progress}%"></div>
      <div class="slider-glow" style="width: {progress}%"></div>
  </div>
  <div class="time-row">
    <span class="time">{formatTime(currentTime)}</span>
    <span class="time">{formatTime(duration)}</span>
  </div>
</div>

<style>
  .timeline-container {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }

  .time-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.85rem;
    font-weight: 600;
    opacity: 0.5;
    font-variant-numeric: tabular-nums;
  }

  .modern-slider {
    position: relative;
    width: 100%;
    height: 6px;
    background: rgba(255,255,255,0.1);
    border-radius: 20px;
    cursor: pointer;
    transition: height 0.2s;
  }
  .modern-slider:hover {
    height: 8px;
  }

  .slider-fill {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background: white;
    border-radius: 20px;
    z-index: 1;
    transition: background 0.3s;
  }

  .knobless-range {
    position: absolute;
    top: -10px;
    bottom: -10px;
    left: 0;
    width: 100%;
    opacity: 0;
    cursor: pointer;
    z-index: 100;
    margin: 0;
  }

  .slider-glow {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background: white;
    filter: blur(8px);
    opacity: 0.3;
    z-index: 0;
    border-radius: 20px;
  }

  .modern-slider:hover .slider-glow {
    opacity: 0.6;
  }
</style>