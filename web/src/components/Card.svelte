<script>
  let { title, subtitle, children, actions } = $props();
</script>

<section class="card chart-card">
  {#if title}
    <header>
      <div>
        <h3>{title}</h3>
        {#if subtitle}<p class="muted">{subtitle}</p>{/if}
      </div>
      {#if actions}<div class="actions">{@render actions()}</div>{/if}
    </header>
  {/if}
  <div class="body">
    {@render children?.()}
  </div>
</section>

<style>
  .chart-card { padding: 18px 20px 20px; display: flex; flex-direction: column; }
  header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 14px; }
  h3 { font-size: 15.5px; }
  p { margin: 3px 0 0; font-size: 12.5px; }
  .body { flex: 1; min-height: 0; }
</style>
