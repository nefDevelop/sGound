<script>
  let { width = "100%", height = "1em", borderRadius = "4px", margin = "0", style = "" } = $props();
</script>

<div class="skeleton" style:width style:height style:border-radius={borderRadius} style:margin style={style}></div>

<style>
  .skeleton {
    background: linear-gradient(90deg, 
      var(--surface-container-highest) 25%, 
      var(--surface-container-high) 50%, 
      var(--surface-container-highest) 75%);
    background-size: 200% 100%;
    animation: loading 1.5s infinite;
  }

  @keyframes loading {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }
</style>
