<script>
  // A compact horizontal bar list — better than a crowded axis chart for
  // ranked "top N" data with long labels (artists, songs).
  let { items, accent = 'var(--accent)' } = $props();
  // items: [{ label, value, sub? }]
  const max = $derived(Math.max(1, ...items.map((d) => d.value)));
</script>

<ul class="barlist">
  {#each items as it, i}
    <li>
      <span class="rank">{i + 1}</span>
      <div class="meta">
        <div class="row">
          <span class="name" title={it.label}>{it.label}</span>
          <span class="val">{it.value}</span>
        </div>
        {#if it.sub}<div class="sub muted">{it.sub}</div>{/if}
        <div class="track">
          <div class="fill" style="width:{(it.value / max) * 100}%; background:{accent}"></div>
        </div>
      </div>
    </li>
  {/each}
</ul>

<style>
  .barlist { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 12px; }
  li { display: flex; gap: 12px; align-items: flex-start; }
  .rank { width: 20px; flex: none; text-align: right; color: var(--faint); font-variant-numeric: tabular-nums; font-weight: 600; padding-top: 1px; }
  .meta { flex: 1; min-width: 0; }
  .row { display: flex; justify-content: space-between; gap: 10px; }
  .name { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 550; }
  .val { color: var(--muted); font-variant-numeric: tabular-nums; flex: none; }
  .sub { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .track { height: 6px; border-radius: 4px; background: var(--surface-3); margin-top: 6px; overflow: hidden; }
  .fill { height: 100%; border-radius: 4px; transition: width 0.5s ease; }
</style>
