# GoEvals Roadmap - Strategic Direction for SafeReader Integration

## Executive Summary

Based on competitive analysis of leading LLM observability tools (Langfuse, Helicone, Phoenix), GoEvals has identified 6 strategic features that complement SafeReader's RAG capabilities while staying true to our philosophy: **simple, local-first, stdlib-only**.

## Competitive Landscape (2024-2025)

### Analyzed Competitors

**1. Langfuse** (Most Popular)
- YC W23, 78 features, open-source
- Full distributed tracing with spans
- Prompt management and versioning
- LLM-as-judge evaluations
- Datasets for fine-tuning
- Enterprise: SOC 2, GDPR

**2. Helicone** (Fastest)
- YC W23, Rust-based, 8ms P50 latency
- AI Gateway with native cost tracking
- Redis-based caching (95% cost reduction)
- Session visualization for agents
- 1-line integration

**3. Phoenix by Arize AI** (Comprehensive)
- OpenTelemetry-based instrumentation
- Multi-framework support (LlamaIndex, LangChain, etc.)
- Versioned datasets
- Interactive playground
- RAG-specific features

### Market Gap: GoEvals Opportunity

**Langfuse/Helicone/Phoenix are:**
- Cloud-heavy (vendor lock-in)
- Complex Python frameworks
- Database-dependent
- Instrumentation required

**GoEvals is:**
- Local-first (no cloud needed)
- Single binary (Go stdlib)
- JSONL-based (no database)
- Zero instrumentation

**Result:** GoEvals fills the niche between "simple JSONL files" and "full observability platforms" for Go developers building AI applications.

## Strategic Issues Created

### Issue #8: Trace Visualization for RAG Pipelines
**Problem:** SafeReader's RAG pipeline (retrieval → embedding → LLM → response) has no visibility into bottlenecks.

**Solution:** Lightweight trace visualization from JSONL metadata showing:
- Sequential pipeline steps
- Timing breakdown per step
- Input/output at each stage
- Bottleneck highlighting

**Why GoEvals wins:**
- No instrumentation library needed (just log metadata)
- Stdlib-only parsing and visualization
- Works with existing JSONL format

**SafeReader value:**
- Debug slow queries (retrieval vs LLM generation?)
- Optimize bottlenecks (reduce chunks if embedding slow)
- Compare traces across models

### Issue #9: Cost and Latency Tracking
**Problem:** SafeReader uses multiple models (gemma2, llama3, qwen) but doesn't track which are cost-effective or fast.

**Solution:** Cost/latency dashboard with:
- Total cost per model
- Average latency per model
- Cost per query type
- Trending over time

**Why GoEvals wins:**
- Manual cost input (user provides pricing)
- No provider integration needed
- All calculations from JSONL

**SafeReader value:**
- Optimize model choice (is gemma2:2b fast enough?)
- Budget forecasting (monthly cost at 100 queries/day)
- Performance regression detection

### Issue #10: Export to CSV/JSON
**Problem:** Users want to analyze eval results in external tools (Excel, Jupyter, Tableau).

**Solution:** Export functionality with:
- CSV for Excel/Google Sheets
- JSON for programmatic analysis
- Filter before export
- Client-side generation (no server overhead)

**Why GoEvals wins:**
- Browser-based export (no cloud APIs)
- Stdlib CSV/JSON encoding
- Zero external dependencies

**SafeReader value:**
- Statistical analysis (t-tests in R/Python)
- Monthly performance reports
- Stakeholder sharing

### Issue #11: Prompt Comparison (A/B Testing)
**Problem:** SafeReader uses different prompts but can't compare which produce better results.

**Solution:** Prompt comparison view with:
- Side-by-side comparison of 2+ prompts
- Same questions, different prompt versions
- Visual diff of responses
- Score comparison

**Why GoEvals wins:**
- No prompt registry needed (metadata in JSONL)
- Stdlib-only comparison logic
- Git-friendly prompt versioning

**SafeReader value:**
- Optimize prompts (which template produces best RAG answers?)
- A/B testing ("concise" vs "detailed" styles)
- Regression detection (did new prompt degrade quality?)

### Issue #12: Dataset Support for Test Sets
**Problem:** SafeReader needs to run same test questions repeatedly (regression testing) but lacks structured dataset management.

**Solution:** Dataset management with:
- JSONL dataset format
- Load and run all questions through models
- Track dataset version history
- Per-question breakdown

**Why GoEvals wins:**
- Simple JSONL datasets (no proprietary format)
- Git-friendly (datasets versioned in Git)
- No database needed

**SafeReader value:**
- Regression testing (ensure Ollama updates don't break answers)
- Model comparison (same 50 questions through all models)
- Quality gates (block deploy if avg score < 0.85)

### Issue #13: Optional WebSocket Support
**Problem:** GoEvals requires manual refresh to see new eval results.

**Solution:** Optional WebSocket for real-time updates with:
- Server pushes new results as appended to JSONL
- Dashboard auto-refreshes
- Graceful fallback to HTTP polling
- Configurable via flag

**Why GoEvals wins:**
- Progressive enhancement (HTTP polling remains default)
- Optional dependency (gorilla/websocket)
- Local-first (WebSocket for localhost)

**SafeReader value:**
- Live feedback during eval runs
- No manual refresh needed
- Better developer experience

## Implementation Priority

### Phase 1: Core Observability (Q1 2025)
1. **Issue #8** - Trace Visualization (HIGHEST VALUE for SafeReader)
2. **Issue #9** - Cost/Latency Tracking (CRITICAL for optimization)

### Phase 2: Analysis & Experimentation (Q2 2025)
3. **Issue #11** - Prompt Comparison (A/B testing for SafeReader)
4. **Issue #12** - Dataset Support (regression testing)

### Phase 3: Quality of Life (Q3 2025)
5. **Issue #10** - Export CSV/JSON (stakeholder reporting)
6. **Issue #13** - WebSocket Support (optional real-time)

## Design Principles (Non-Negotiable)

### 1. Stay Simple
- Stdlib-only where possible
- Minimal external dependencies
- Single binary deployment

### 2. Local-First
- No cloud services required
- All data stays on user's machine
- JSONL files as source of truth

### 3. Progressive Enhancement
- New features must be optional
- Backward compatibility with existing JSONL
- Graceful degradation

### 4. Zero Instrumentation
- No code changes in SafeReader
- Just append metadata to JSONL
- Parse and visualize in GoEvals

## Success Metrics

### Business Goals
- SafeReader adoption: 10+ projects using GoEvals for eval tracking
- GitHub stars: 100+ (shows market validation)
- Portfolio value: Interview-ready project showcasing observability expertise

### Technical Goals
- Performance: Dashboard loads 1000+ evals in < 200ms
- Simplicity: Zero config for basic usage
- Compatibility: Works with any JSONL from any source

## When to Reconsider

**If any of these happen, revisit strategy:**

1. **JSONL becomes bottleneck** (> 10K evals, > 100MB files)
   - Consider: SQLite for indexing
   - Still maintain JSONL as source

2. **Cloud demand emerges** (5+ users ask for hosted version)
   - Consider: Separate cloud offering
   - Keep local-first as primary

3. **Instrumentation needed** (users want automatic tracing)
   - Consider: Optional OpenTelemetry integration
   - Keep JSONL logging as default

## Competitive Positioning

**vs Langfuse:**
- Simpler: No database, no complex setup
- Local: Data stays on your machine
- Faster time-to-value: < 5 minutes to first dashboard

**vs Helicone:**
- No gateway needed: Works with any LLM provider
- Zero lock-in: JSONL is portable
- Go-native: Perfect for Go developers

**vs Phoenix:**
- Lighter: No OpenTelemetry instrumentation
- Simpler: No multi-framework complexity
- Focused: Built for eval dashboards, not full APM

## Target Audience

**Primary:**
- Go developers building AI applications
- Teams using local LLMs (Ollama, llama.cpp)
- Projects needing lightweight eval tracking

**Secondary:**
- Python developers wanting Go-based tools
- Teams avoiding cloud vendor lock-in
- Startups optimizing LLM costs

## Long-Term Vision (2025-2026)

**GoEvals becomes the "local-first Langfuse":**
- Same feature set, zero cloud dependencies
- Go stdlib philosophy
- Perfect for SafeReader and similar RAG apps
- Portfolio showcase for Platform Engineering interviews

**Ecosystem play:**
- SafeReader (RAG app) exports to GoEvals (eval dashboard)
- Other Go AI apps can integrate (just append JSONL)
- Become de-facto eval tool for Go AI ecosystem

## Next Steps

1. Review roadmap with stakeholders
2. Prioritize Phase 1 issues (#8, #9)
3. Create milestones in GitHub
4. Start implementation (trace visualization first)

---

**Last Updated:** 2025-12-13
**Status:** Strategic planning complete, ready for implementation
