# Factor Backend Redesign Plan
**Date:** 2026-04-19  
**Author:** sk11  
**Status:** Draft

## 1. Problem Statement
The current Factor backtesting platform suffers from three core limitations:

1. **UI is outdated and hacky** – built pre-AI era with inconsistent design patterns.
2. **Backtesting experience is shallow** – natural‑language→equation was a gimmick, unable to handle real‑world queries for missing data or complex indicators.
3. **Not agent‑friendly** – the web UI model doesn’t align with the future of AI‑driven investment research, where agents need programmatic access.

This plan focuses on the **backend and agent‑first foundation** that must be laid before UI/UX improvements can be meaningful.

## 2. Vision
Factor becomes the **default platform for AI‑driven investment strategy testing**. Any agent (or human) can describe a strategy in natural language, automatically acquire the necessary data, run a rigorous backtest, and receive structured, interpretable results—all through a clean API.

## 3. Core Principles
- **API‑first** – every feature is exposed via a well‑documented, versioned API.
- **Extensible data layer** – pluggable adapters for any data source (Yahoo, Alpha Vantage, custom CSVs, etc.).
- **Agent‑native** – SDKs for Python/JS, CLI tools, webhook/event‑driven results.
- **Progressive enhancement** – keep the existing UI working while building the new foundation underneath.

## 4. Proposed Architecture
```
┌─────────────────────────────────────────────────────────┐
│                    Client Layer                         │
│  • Web UI (React)                                       │
│  • Python/JS SDKs                                       │
│  • Headless CLI                                         │
│  • Direct API calls (agents)                            │
└─────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────┐
│                    API Gateway Layer                    │
│  • REST/GraphQL (TBD)                                   │
│  • Authentication (API keys, OAuth)                     │
│  • Rate limiting, logging, metrics                      │
└─────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────┐
│                 Core Backtesting Engine                 │
│  • Strategy parser (natural language → AST)             │
│  • Data acquisition & caching service                   │
│  • Factor/indicator library                             │
│  • Portfolio simulation & trade execution               │
└─────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────┐
│                  Pluggable Data Layer                   │
│  • Adapters: Yahoo Finance, Alpha Vantage, Tiingo, etc. │
│  • Custom CSV/JSON upload                               │
│  • Real‑time streaming (future)                         │
└─────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────┐
│                    Storage Layer                        │
│  • PostgreSQL (existing schema)                         │
│  • Redis (caching)                                      │
│  • S3 (raw data blobs, results)                         │
└─────────────────────────────────────────────────────────┘
```

## 5. Phase 1: API‑First Foundation (4–6 weeks)
**Goal:** Establish a robust, versioned API that can serve both the existing UI and future agent clients.

### Deliverables
1. **API specification** – OpenAPI 3.0 definition covering:
   - Strategy creation & management
   - Backtest execution & status polling
   - Results retrieval (structured JSON)
   - Data source listing & health checks
2. **API gateway implementation** – Extend existing Go HTTP server with:
   - Structured error responses
   - Request/response validation
   - Authentication middleware (API keys + OAuth)
   - Comprehensive logging (Datadog integration)
3. **Python SDK (v0.1)** – Simple client library that wraps the API.
4. **CLI tool (factorctl)** – Basic commands to run backtests, list strategies, fetch results.
5. **Agent‑friendly webhooks** – Subscribe to backtest completion events via HTTP callbacks.

### Technical Steps
- Design and document API endpoints (start with `/v1/` prefix).
- Implement new endpoints in `api/` (follow existing generator pattern).
- Add API‑key management table (`api_key`) and middleware.
- Build Python SDK using `httpx`/`pydantic`.
- Create `factorctl` using Click or Typer.
- Add webhook subscription table and notification service.

## 6. Phase 2: Natural Language Pipeline (3–4 weeks)
**Goal:** Turn the “gimmick” into a first‑class feature that can handle real‑world queries, including data acquisition.

### Deliverables
1. **Enhanced NLP‑to‑AST parser** – Replace simple GPT prompt with a two‑stage pipeline:
   - Intent recognition & entity extraction (what indicator, what assets, what time range)
   - AST generation with validation against available data sources
2. **Data‑source discovery & fallback** – When a requested ticker/indicator isn’t in the local DB:
   - Automatically attempt to fetch from configured adapters (Yahoo, etc.)
   - If unavailable, suggest similar alternatives to the user/agent
   - Allow custom data upload via CSV/JSON
3. **Indicator library** – Catalog of common financial indicators (SMA, RSI, Sharpe, etc.) with pre‑built adapters.

### Technical Steps
- Extend `gpt/prompt.md` into a multi‑step pipeline (possibly using OpenCode or local LLM).
- Create `DataAdapter` interface and implement Yahoo/Alpha Vantage adapters.
- Add `indicator` package with registry and parameter validation.
- Build “data‑availability” service that checks and recommends alternatives.

## 7. Phase 3: Agent SDK & CLI (2–3 weeks)
**Goal:** Make Factor the easiest platform for AI agents to run backtests.

### Deliverables
1. **Full‑featured Python SDK** – Async support, type hints, comprehensive docs.
2. **Agent‑oriented examples** – Jupyter notebooks showing how to:
   - Describe a strategy in plain English
   - Monitor backtest progress
   - Parse and visualize results
3. **CLI enhancements** – Support for batch runs, configuration files, output formats (JSON, CSV, Markdown).
4. **Result streaming** – Option to stream intermediate results (e.g., daily returns) during long backtests.

### Technical Steps
- Expand Python SDK with async methods and result objects.
- Create `examples/` directory with notebook templates.
- Add `factorctl run --config strategy.yaml` and `factorctl stream <backtest-id>`.
- Implement server‑sent events (SSE) for progress updates.

## 8. Phase 4: Data Adaptors & Extensibility (ongoing)
**Goal:** Enable users (and agents) to bring their own data and define custom indicators.

### Deliverables
1. **Plugin system for data adapters** – Load external adapters at runtime.
2. **Custom indicator DSL** – Simple language (or Python snippets) for user‑defined calculations.
3. **Data‑quality monitoring** – Alert when a data source goes stale or returns anomalies.
4. **Community adapter repository** – Curated list of third‑party adapters (Crypto, ETFs, Alternative data).

### Technical Steps
- Define plugin interface (Go plugins or embedded JavaScript).
- Create `factor‑dsl` for indicator definitions (subset of Python math syntax).
- Add health‑check cron jobs for each active data source.
- Set up GitHub repo for community adapters.

## 9. Migration Strategy
- **Backwards compatibility** – Keep all existing API endpoints working; deprecate gradually.
- **Dual‑run during transition** – New API runs alongside old one; UI can be migrated piecemeal.
- **Data migration** – Existing strategies and backtest results remain accessible via new API.

## 10. Success Metrics
- **API adoption** – >50% of backtest runs originate via API/SDK within 3 months.
- **Data‑source coverage** – Support top‑10 requested tickers/indicators without manual intervention.
- **Agent‑friendly** – At least 3 external projects/build‑a‑thons using Factor’s SDK.
- **Performance** – Backtest execution time under 30s for 10‑year daily data.

## Next Immediate Actions
1. Finalize API specification with stakeholder review.
2. Implement Phase 1 endpoints and Python SDK prototype.
3. Update CI/CD to deploy lambda only when backend code changes (see separate PR).