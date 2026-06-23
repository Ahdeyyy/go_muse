<script>
  let { label, value, hint, accent = 'var(--accent)' } = $props();
</script>

<div class="stat card">
  <span class="bar" style="background:{accent}"></span>
  <div class="label">{label}</div>
  <div class="value">{value}</div>
  {#if hint}<div class="hint muted">{hint}</div>{/if}
</div>

<style>
  .stat { position: relative; padding: 16px 18px; overflow: hidden; }
  .bar { position: absolute; left: 0; top: 0; bottom: 0; width: 3px; }
  .label { font-size: 12.5px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; font-weight: 600; }
  .value { font-size: 28px; font-weight: 700; letter-spacing: -0.02em; margin-top: 4px; }
  .hint { font-size: 12.5px; margin-top: 2px; }
</style>
