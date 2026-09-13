<!-- SPDX-License-Identifier: MPL-2.0 -->

<script lang="ts">
  import { onMount } from 'svelte'
  import type { Following, History } from '../bindings/animeportable/apps/desktop/backend/models.js'
  import { createHomeController, formatPosition, homeLimit, initialHomeState, latestHistory, serviceErrorMessage, titleFor, type HomeController, type HomeState, type LoadName } from './home.js'

  let state: HomeState = initialHomeState()
  let controller: HomeController | undefined
  let serviceUnavailable = false
  let serviceGeneration = 0

  function update(next: HomeState): void {
    state = next
  }

  function loadService(): void {
    const generation = ++serviceGeneration
    serviceUnavailable = false
    import('../bindings/animeportable/apps/desktop/backend/service.js').then((binding) => {
      if (generation !== serviceGeneration) return
      controller?.dispose()
      controller = createHomeController(binding, update)
      controller.loadAll()
    }).catch(() => {
      if (generation !== serviceGeneration) return
      serviceUnavailable = true
      state = {
        ...state,
        loadStatus: { library: 'error', history: 'error', following: 'error' },
        loadErrors: { library: serviceErrorMessage, history: serviceErrorMessage, following: serviceErrorMessage },
      }
    })
  }

  onMount(() => {
    loadService()
    return () => {
      serviceGeneration += 1
      controller?.dispose()
      controller = undefined
    }
  })

  function retry(name: LoadName): void {
    if (controller) {
      controller.retry(name)
    } else {
      loadService()
    }
  }

  function play(item: History): void {
    controller?.play(item)
  }

  function title(animeId: string): string {
    return titleFor(state.library, animeId)
  }

  function followingItems(items: Following[]): Following[] {
    return items.slice(0, homeLimit)
  }

  $: continueItems = latestHistory(state.history)
</script>

<div class="home-sections">
  {#if state.loadStatus.library === 'error' || serviceUnavailable}
    <div class="service-notice" role="alert">
      <span>{state.loadErrors.library || serviceErrorMessage}</span>
      <button class="retry" type="button" onclick={() => retry('library')}>重試</button>
    </div>
  {/if}

  <section class="home-section" aria-labelledby="continue-title">
    <div class="section-heading">
      <div>
        <h2 id="continue-title">繼續觀看</h2>
        <p>從上次停下的位置接著播放。</p>
      </div>
    </div>
    {#if state.loadStatus.history === 'loading'}
      <p class="state" role="status">載入中…</p>
    {:else if state.loadStatus.history === 'error'}
      <p class="state" role="alert">{state.loadErrors.history}</p>
      <button class="retry" type="button" onclick={() => retry('history')}>重試</button>
    {:else if continueItems.length === 0}
      <p class="state" role="status">還沒有可繼續觀看的內容。</p>
    {:else}
      <div class="card-list">
        {#each continueItems as item (item.animeId)}
          <article class="card">
            <div class="card-copy">
              <h3>{title(item.animeId)}</h3>
              <p>上次播放位置 {formatPosition(item.position)}</p>
            </div>
            <button
              class="play-button"
              type="button"
              aria-label={`播放 ${title(item.animeId)}`}
              disabled={state.playStatus === 'playing'}
              onclick={() => play(item)}
            >{state.playStatus === 'playing' && state.playEpisodeId === item.episodeId ? '播放中…' : '播放'}</button>
          </article>
        {/each}
      </div>
    {/if}
    {#if state.playStatus === 'success'}
      <p class="feedback success" role="status">{state.playMessage}</p>
    {:else if state.playStatus === 'error'}
      <p class="feedback error" role="alert">{state.playMessage}</p>
    {/if}
  </section>

  <section class="home-section" aria-labelledby="following-title">
    <div class="section-heading">
      <div>
        <h2 id="following-title">追蹤中</h2>
        <p>快速查看你收藏的作品。</p>
      </div>
    </div>
    {#if state.loadStatus.following === 'loading'}
      <p class="state" role="status">載入中…</p>
    {:else if state.loadStatus.following === 'error'}
      <p class="state" role="alert">{state.loadErrors.following}</p>
      <button class="retry" type="button" onclick={() => retry('following')}>重試</button>
    {:else if state.following.length === 0}
      <p class="state" role="status">尚未追蹤任何作品。</p>
    {:else}
      <div class="card-list">
        {#each followingItems(state.following) as item (item.animeId)}
          <article class="card compact-card">
            <h3>{title(item.animeId)}</h3>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <section class="home-section" aria-labelledby="updated-title">
    <div class="section-heading">
      <div>
        <h2 id="updated-title">最近更新</h2>
        <p>整理本機收藏中的新內容。</p>
      </div>
    </div>
    <p class="state" role="status">目前沒有本機更新資料。</p>
  </section>

  <section class="home-section" aria-labelledby="today-title">
    <div class="section-heading">
      <div>
        <h2 id="today-title">今日播出</h2>
        <p>掌握今天值得留意的播出安排。</p>
      </div>
    </div>
    <p class="state" role="status">目前沒有本機播出資料。</p>
  </section>
</div>

<style>
  .home-sections {
    display: grid;
    gap: 2rem;
    margin-top: 3rem;
  }

  .home-section {
    padding-top: 1.25rem;
    border-top: 1px solid #2b303b;
  }

  .section-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }

  .section-heading h2,
  .section-heading p {
    margin: 0;
  }

  .section-heading h2 {
    font-size: 1.1rem;
  }

  .section-heading p {
    margin-top: 0.35rem;
    color: #7f899b;
    font-size: 0.85rem;
    line-height: 1.5;
  }

  .state,
  .feedback {
    margin: 0.75rem 0 0;
    color: #7f899b;
    font-size: 0.9rem;
    line-height: 1.6;
  }

  .feedback.error,
  .service-notice {
    color: #f0a8a8;
  }

  .feedback.success {
    color: #a9d8b5;
  }

  .service-notice {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem 1rem;
    border: 1px solid #633c46;
    border-radius: 0.65rem;
    background: #261c24;
    font-size: 0.85rem;
    line-height: 1.5;
  }

  .card-list {
    display: grid;
    gap: 0.55rem;
    margin-top: 0.8rem;
  }

  .card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem 1rem;
    border: 1px solid #2b303b;
    border-radius: 0.65rem;
    background: #191d25;
  }

  .compact-card {
    min-height: 3rem;
  }

  .card-copy {
    min-width: 0;
  }

  .card h3 {
    overflow: hidden;
    margin: 0;
    color: #e8ebf2;
    font-size: 0.95rem;
    font-weight: 650;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card p {
    margin: 0.3rem 0 0;
    color: #7f899b;
    font-size: 0.8rem;
  }

  .play-button,
  .retry {
    flex: 0 0 auto;
    border: 1px solid #536789;
    border-radius: 0.45rem;
    background: #293653;
    color: #e5ebfb;
    cursor: pointer;
    font-size: 0.8rem;
    padding: 0.45rem 0.7rem;
  }

  .play-button:disabled {
    cursor: wait;
    opacity: 0.7;
  }

  .retry {
    margin-top: 0.65rem;
    background: transparent;
  }

  .service-notice .retry {
    margin-top: 0;
  }
</style>
