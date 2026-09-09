<!-- SPDX-License-Identifier: MPL-2.0 -->

<svelte:head>
  <title>AnimePortable</title>
</svelte:head>

<script lang="ts">
  type SectionId = 'home' | 'schedule' | 'following' | 'history' | 'search' | 'settings'

  type Section = {
    id: SectionId
    label: string
    description: string
    icon: string
  }

  const sections: Section[] = [
    { id: 'home', label: '首頁', description: '從這裡開始探索你的動畫空間。', icon: '⌂' },
    { id: 'schedule', label: '時間表', description: '查看接下來的播出安排。', icon: '◷' },
    { id: 'following', label: '追蹤', description: '集中管理你正在追蹤的作品。', icon: '♡' },
    { id: 'history', label: '歷史紀錄', description: '回顧你最近看過的內容。', icon: '↺' },
    { id: 'search', label: '搜尋', description: '尋找想看的動畫。', icon: '⌕' },
    { id: 'settings', label: '設定', description: '調整 AnimePortable 的使用方式。', icon: '⚙' },
  ]

  let activeSection = $state<SectionId>('home')
  let mainElement = $state<HTMLElement>()

  function focusMain() {
    mainElement?.focus()
  }

  const activeDetails = $derived(sections.find((section) => section.id === activeSection) ?? sections[0])
</script>

<a class="skip-link" href="#main-content" onclick={focusMain}>跳至主要內容</a>

<div class="app-shell">
  <aside class="sidebar" aria-label="主要導覽">
    <div class="brand">
      <div class="brand-mark" aria-hidden="true">A</div>
      <div>
        <p class="brand-name">AnimePortable</p>
        <p class="brand-caption">桌面動畫收藏</p>
      </div>
    </div>

    <nav class="navigation" aria-label="頁面">
      <p class="navigation-label">瀏覽</p>
      <div class="navigation-list">
        {#each sections as section}
          <button
            class:active={activeSection === section.id}
            type="button"
            aria-current={activeSection === section.id ? 'page' : undefined}
            onclick={() => activeSection = section.id}
          >
            <span class="nav-icon" aria-hidden="true">{section.icon}</span>
            <span>{section.label}</span>
          </button>
        {/each}
      </div>
    </nav>

    <p class="sidebar-note">安靜地整理，專注地觀看。</p>
  </aside>

  <main id="main-content" class="main-content" tabindex="-1" bind:this={mainElement}>
    <div class="content-wrap">
      <p class="overline">AnimePortable / {activeDetails.label}</p>
      <h1>{activeDetails.label}</h1>
      <p class="description">{activeDetails.description}</p>

      <section class="empty-state" aria-labelledby="empty-state-title">
        <div class="empty-state-mark" aria-hidden="true">✦</div>
        <div>
          <h2 id="empty-state-title">這裡很快就會準備好</h2>
          <p>目前沒有可顯示的內容。這個區域會在功能完成後呈現於此。</p>
        </div>
      </section>
    </div>
  </main>
</div>

<style>
  :global(*) {
    box-sizing: border-box;
  }

  :global(html) {
    min-width: 20rem;
    background: #111318;
  }

  :global(body) {
    margin: 0;
    background: #111318;
    color: #f4f5f7;
    font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  }

  :global(button),
  :global(a) {
    font: inherit;
  }

  .skip-link {
    position: fixed;
    top: 0.75rem;
    left: 0.75rem;
    z-index: 10;
    padding: 0.65rem 0.9rem;
    border-radius: 0.55rem;
    background: #f4f5f7;
    color: #111318;
    font-size: 0.875rem;
    font-weight: 700;
    transform: translateY(-180%);
    transition: transform 120ms ease;
  }

  .skip-link:focus {
    transform: translateY(0);
  }

  .app-shell {
    display: grid;
    grid-template-columns: minmax(11rem, 15rem) minmax(0, 1fr);
    min-height: 100vh;
  }

  .sidebar {
    display: flex;
    min-height: 100vh;
    flex-direction: column;
    gap: 2.5rem;
    overflow-y: auto;
    padding: clamp(1.5rem, 4vw, 2.75rem) clamp(1rem, 3vw, 2rem);
    border-right: 1px solid #282d37;
    background: #171a21;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .brand-mark {
    display: grid;
    width: 2.15rem;
    height: 2.15rem;
    flex: 0 0 auto;
    place-items: center;
    border: 1px solid #64718d;
    border-radius: 0.65rem;
    background: #263149;
    color: #e0e7f8;
    font-size: 1rem;
    font-weight: 750;
  }

  .brand-name,
  .brand-caption,
  .navigation-label,
  .sidebar-note,
  .overline,
  .description,
  .empty-state p {
    margin: 0;
  }

  .brand-name {
    color: #f4f5f7;
    font-size: 0.9rem;
    font-weight: 700;
    letter-spacing: 0.01em;
  }

  .brand-caption {
    margin-top: 0.15rem;
    color: #8f98aa;
    font-size: 0.75rem;
  }

  .navigation {
    display: grid;
    gap: 0.85rem;
  }

  .navigation-label,
  .overline {
    color: #8390a8;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .navigation-list {
    display: grid;
    gap: 0.3rem;
  }

  .navigation button {
    display: flex;
    width: 100%;
    align-items: center;
    gap: 0.75rem;
    padding: 0.7rem;
    border: 0;
    border-radius: 0.6rem;
    background: transparent;
    color: #aeb6c6;
    cursor: pointer;
    font-size: 0.9rem;
    text-align: left;
  }

  .navigation button:hover {
    background: #202631;
    color: #f4f5f7;
  }

  .navigation button.active {
    background: #2b3447;
    color: #f4f5f7;
  }

  .nav-icon {
    display: inline-grid;
    width: 1.25rem;
    flex: 0 0 auto;
    place-items: center;
    color: #8997b5;
    font-size: 1.05rem;
    line-height: 1;
  }

  .navigation button.active .nav-icon {
    color: #c9d7f5;
  }

  .sidebar-note {
    margin-top: auto;
    color: #6f788a;
    font-size: 0.75rem;
    line-height: 1.6;
  }

  .main-content {
    min-width: 0;
    min-height: 100vh;
    overflow-y: auto;
    outline: none;
  }

  .main-content:focus-visible {
    box-shadow: inset 0 0 0 3px #a9bce9;
  }

  .content-wrap {
    display: flex;
    min-height: 100vh;
    max-width: 58rem;
    flex-direction: column;
    justify-content: center;
    padding: clamp(2rem, 8vw, 7rem) clamp(1.5rem, 8vw, 7rem);
  }

  h1,
  h2 {
    margin: 0;
    color: #f4f5f7;
    font-weight: 700;
    letter-spacing: -0.025em;
  }

  h1 {
    margin-top: 0.85rem;
    font-size: clamp(2rem, 5vw, 3.5rem);
    line-height: 1.08;
  }

  .description {
    max-width: 34rem;
    margin-top: 1rem;
    color: #aeb6c6;
    font-size: 1rem;
    line-height: 1.6;
  }

  .empty-state {
    display: flex;
    max-width: 38rem;
    align-items: flex-start;
    gap: 1rem;
    margin-top: clamp(3rem, 8vh, 5.5rem);
    padding-top: 1.25rem;
    border-top: 1px solid #2b303b;
  }

  .empty-state-mark {
    display: grid;
    width: 2rem;
    height: 2rem;
    flex: 0 0 auto;
    place-items: center;
    border-radius: 50%;
    background: #252b38;
    color: #a9bce9;
    font-size: 0.9rem;
  }

  h2 {
    font-size: 1.05rem;
    line-height: 1.4;
  }

  .empty-state p {
    margin-top: 0.45rem;
    color: #7f899b;
    font-size: 0.9rem;
    line-height: 1.6;
  }

  :global(:focus-visible) {
    outline: 3px solid #a9bce9;
    outline-offset: 3px;
  }

  @media (prefers-reduced-motion: reduce) {
    .skip-link {
      transition: none;
    }
  }

  @media (max-width: 40rem) {
    .app-shell {
      grid-template-columns: minmax(9.5rem, 11rem) minmax(0, 1fr);
    }

    .sidebar {
      gap: 2rem;
      padding: 1.25rem 0.75rem;
    }

    .brand {
      gap: 0.5rem;
    }

    .brand-mark {
      width: 1.9rem;
      height: 1.9rem;
    }

    .brand-caption,
    .sidebar-note {
      display: none;
    }

    .navigation button {
      gap: 0.5rem;
      padding-inline: 0.55rem;
      font-size: 0.82rem;
    }

    .content-wrap {
      padding: 2rem 1.25rem;
    }
  }
</style>
