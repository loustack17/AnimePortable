# Anime Client MVP Engineering Plan

This directory is the source of truth for implementing the MVP.

The product is a lightweight, fast, keyboard-friendly Anime1 desktop client focused on immersive playback through MPV. The application handles discovery, metadata, scheduling, following, history, episode selection, playback orchestration, and progress tracking. MPV handles actual media playback and all playback UX.

## Required reading order for any AI agent

1. `01_PRODUCT_CONTRACT.md`
2. `02_ARCHITECTURE.md`
3. `03_SECURITY.md`
4. `04_MVP_IMPLEMENTATION_PLAN.md`
5. `05_LOOP_ENGINEERING_RUNBOOK.md`
6. `06_ACCEPTANCE_CRITERIA.md`
7. `07_ADR_BASELINE.md`

An agent must not start implementation before reading all seven documents.

## Source-of-truth rule

If code, comments, or previous implementation conflict with these documents, these documents win unless a new ADR explicitly changes the decision.

## Non-negotiable product principles

- Lightweight
- Fast
- Intuitive
- Minimal and visually calm
- Easy to use
- Keyboard-first on desktop
- Mouse fully supported
- Immersive playback
- No danmaku
- No social features
- No ads
- No telemetry by default
- Local-first privacy
- MPV-first playback
- Modular but not over-engineered
- External dependencies are replaceable
- Anime1 must not be allowed to become a permanent architectural dependency of the core

## MVP technology baseline

- Go
- Wails v3
- Svelte + TypeScript
- SQLite
- MPV as the only MVP player
- MPV JSON IPC
- Anime1 as the initial anime source adapter
- AniList as primary metadata provider
- Bangumi as fallback / cross-check metadata provider
- Windows, Linux, macOS
- No mobile in MVP

## Core design rule

Use a minimal Ports & Adapters architecture.

Only create replaceable ports for real external boundaries:

- `AnimeSource`
- `MetadataProvider`
- `Player`
- `Store`

Do not introduce enterprise-style abstraction layers for every helper or utility.

## MVP completion definition

The MVP is complete only when all functional, security, performance, resource-lifecycle, keyboard-accessibility, and cross-platform acceptance criteria in `06_ACCEPTANCE_CRITERIA.md` pass.
