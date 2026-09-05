# wechat-analyzer

A local-first Go CLI for turning exported WeChat conversations into useful statistics, visual reports, and optional LLM-assisted personality analysis.

`wechat-analyzer` reads JSON exported by `wechat-export`, computes conversation statistics locally, and can generate self-contained HTML reports. AI analysis is optional and is routed through a user-configured LLM provider.

## What it does

- **Conversation statistics**
  - active-day and calendar-day averages with explicit denominators
  - session reconstruction, initiator/ending ratios, and longest session
  - per-person response-time median/P90 based on speaker turns
  - active streaks, monthly trends, and period-over-period comparison
  - stable JSON output for scripts and downstream visualizations
- **Optional AI analysis**
  - aggregate-only analysis by default: no message text is sent
  - opt-in evidence analysis with deterministic sampling and local redaction
  - locally validated claims that cite stable evidence IDs
  - a no-network preview of the exact redacted Evidence Bundle
- **HTML reports**
  - responsive, dependency-free visualizations
  - a single HTML file that can be viewed locally without network requests
- **Provider abstraction**
  - DeepSeek, Kimi / Moonshot, Qwen, Doubao, Zhipu GLM, and Anthropic
  - provider selection is configuration-driven through environment variables and `--provider`

## Privacy model

The tool is local-first, but the privacy boundary depends on the command you run:

- `stats` and local HTML report generation can run entirely on your machine; the exported conversation file does not need to be sent to a third party.
- HTML and JSON output exclude message text by default. `--include-content` explicitly adds the longest-message text; do not use it for a report you intend to share unless you have reviewed the output.
- The default `analyze` mode sends aggregate statistics, not message text, to the configured LLM provider.
- `analyze --evidence` is an explicit opt-in to send a deterministic sample of locally redacted text excerpts. Use `--preview` first to inspect the exact bundle without an API key or network request.
- In evidence mode, `--include-content` affects only local JSON/HTML output: it embeds the already-redacted excerpts, never the original `TopMessages` text.
- API keys are read from environment variables. `wechat-analyzer` does not provide a hosted backend of its own.

Redaction is best-effort. v0.3 recognizes mainland China phone numbers, email addresses,
18-digit ID-card patterns, `wxid` values, and URL query strings. It cannot reliably find
every name, address, account number, or context-specific identifier. Prompt data is JSON
encoded and the system instruction treats excerpts as untrusted input, but this reduces
rather than eliminates prompt-injection risk. Review the preview and your provider's data
policy before enabling evidence mode.

If you want a fully local workflow, use statistics/reporting or evidence preview without a
live AI request.

## Report preview

![Example HTML report generated from anonymous sample data](docs/report-preview.jpg)

The screenshot is generated from the repository's synthetic [`examples/sample-conversation.json`](examples/sample-conversation.json) fixture; it contains no real conversation data.

## Installation

### Install with Go

```bash
go install github.com/superShen0916/wechat-analyzer@latest
```

### Build from source

```bash
git clone https://github.com/superShen0916/wechat-analyzer.git
cd wechat-analyzer
go build -o wechat-analyzer ./cmd
```

## Usage

### Prepare data

Export a conversation with `wechat-export` as JSON.

### Statistics

```bash
# Analyze one conversation
wechat-analyzer stats ./output/张三.json

# Generate a local HTML statistics report
wechat-analyzer stats ./output/张三.json --html

# Analyze all exported conversations in a directory
wechat-analyzer stats ./output/

# Use a 45-minute session boundary
wechat-analyzer stats ./output/张三.json --session-gap 45m

# Produce machine-readable output (message text is omitted by default)
wechat-analyzer stats ./output/张三.json --format json

# Explicitly include longest-message text in JSON or HTML
wechat-analyzer stats ./output/张三.json --html --include-content
```

Try it without supplying personal data:

```bash
go run ./cmd stats examples/sample-conversation.json --html
```

### Compare two periods

`compare` accepts exactly two `--period` values. A period can be a year (`YYYY`), a
month (`YYYY-MM`), or an inclusive date range (`YYYY-MM-DD..YYYY-MM-DD`).

```bash
# Compare two years
wechat-analyzer compare ./output/张三.json --period 2025 --period 2026

# Compare a month with a custom date range and emit JSON
wechat-analyzer compare ./output/张三.json \
  --period 2026-01 \
  --period 2026-02-01..2026-02-28 \
  --format json
```

Both periods must contain messages. Available metrics include an absolute change;
percentage changes are `null` when the first-period baseline is zero. A response
median and its delta are `null` when either period has no response sample for
that person, so missing data is not presented as a zero-second response.

### AI personality analysis

Set an API key for at least one supported provider:

```bash
export DEEPSEEK_API_KEY=your_key_here
```

Then run:

```bash
# Auto-select an available configured provider
wechat-analyzer analyze ./output/张三.json

# Select a provider explicitly
wechat-analyzer analyze ./output/张三.json --provider deepseek

# Generate an HTML analysis report
wechat-analyzer analyze ./output/张三.json --html

# Write reports to a specific directory
wechat-analyzer analyze ./output/张三.json --html --output ./reports

# The AI result plus local statistics can also be emitted as JSON
wechat-analyzer analyze ./output/张三.json --provider deepseek --format json
```

There are two AI modes:

| Mode | Sent to the provider | Result contract |
| --- | --- | --- |
| default | aggregate counts, ratios, periods, and activity windows | compatible free-text personality summary |
| `--evidence` | aggregate facts plus locally redacted sampled excerpts | strict JSON, validated claims, evidence IDs, limitations, and prompt version |

Preview evidence locally before sending anything:

```bash
# No API key is required; no provider client or network request is created
wechat-analyzer analyze ./output/张三.json --evidence --preview

# Produce a machine-readable v0.3 preview envelope
wechat-analyzer analyze ./output/张三.json --evidence --preview --format json
```

Run evidence-backed analysis after reviewing the preview:

```bash
# Defaults: at most 80 text messages and 12,000 Unicode characters
wechat-analyzer analyze ./output/张三.json --evidence --provider deepseek

# Tune the deterministic sampling budget
wechat-analyzer analyze ./output/张三.json --evidence \
  --evidence-messages 120 --evidence-chars 20000

# JSON/HTML contains evidence IDs but hides excerpt text by default
wechat-analyzer analyze ./output/张三.json --evidence --html

# Explicitly embed only the redacted excerpts in local JSON/HTML output
wechat-analyzer analyze ./output/张三.json --evidence --html --include-content
```

Evidence selection keeps non-empty text messages, stably orders them by time, and uses an
even timeline sample when the budget is exceeded. If both speakers have candidate messages,
the sample includes both. Local IDs such as `m0001` are created after sampling; original
WeChat message IDs, source paths, and contact identifiers are not included in the evidence
payload. A model-reported `low`, `medium`, or `high` confidence is a self-assessment, not a
statistical confidence interval, and every conclusion applies only to the selected sample.

## Relationship metric definitions

- Messages are copied and stably sorted by `create_time` before analysis; the input data is never reordered in place.
- A new session begins when the gap from the preceding message is **greater than** 30 minutes. Exactly 30 minutes stays in the same session. Change this with `--session-gap`.
- Consecutive messages from the same person form one turn. A response sample is recorded only when the speaker changes, from the previous turn's last message to the new turn's first message.
- Response P50 is the median (the mean of the two middle values for an even sample count). P90 uses the nearest-rank definition.
- `msgs_per_day` is retained for compatibility and means messages per active day. `msgs_per_active_day` names the same metric explicitly; `msgs_per_calendar_day` uses the inclusive date span.
- Monthly sessions belong to the month containing the session's first message. The longest active streak counts consecutive local calendar dates with at least one message.
- All dates, hours, periods, and month boundaries use the machine's local timezone. The timezone is included in statistics JSON output.

Response speed describes chat rhythm only. It should not be interpreted as a score of relationship quality or personal commitment.

List available providers:

```bash
wechat-analyzer providers
```

## Supported LLM providers

| Provider | `--provider` | API key environment variable |
| --- | --- | --- |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` |
| Kimi / Moonshot | `moonshot` | `MOONSHOT_API_KEY` |
| Qwen | `qwen` | `DASHSCOPE_API_KEY` |
| Doubao | `doubao` | `DOUBAO_API_KEY` |
| Zhipu GLM | `zhipu` | `ZHIPU_API_KEY` |
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` |

## Input format

The current parser expects the JSON format produced by `wechat-export`, for example:

```json
{
  "talker": {
    "user_name": "wxid_xxx",
    "nick_name": "张三",
    "is_group": false
  },
  "total": 12345,
  "exported_at": "2024-01-01T00:00:00+08:00",
  "messages": [
    {
      "local_id": 12345,
      "type": 1,
      "type_name": "text",
      "is_sender": true,
      "create_time": 1704067200,
      "content": "你好",
      "display_content": "你好"
    }
  ]
}
```

## Reports

Statistics and AI analysis can both produce a single self-contained HTML file. The report embeds its visualizations and does not load scripts, fonts, or other assets from the network. Evidence reports connect each validated claim to local evidence IDs and state the sample coverage and limitations.

The report includes session, response, initiator, active-streak, and monthly-trend views. Message text is hidden by default. In statistics and default aggregate reports, `--include-content` embeds the longest original messages. In evidence reports it suppresses those originals and embeds only the redacted evidence excerpts. Either form remains readable by anyone who receives the report, so review it before sharing.

Default output directories are:

- `./wechat_analyze_stats`
- `./wechat_analyze_ai`

## Development and testing

The project is a Go CLI with separate packages for loading exported data, statistics, LLM integration, and report generation.

```bash
# Build
go build ./...

# Run package tests / compile checks
go test ./...

# Static checks
go vet ./...
```

CI runs tests and static checks on Linux, macOS, and Windows. The test suite covers JSON loading, statistics, provider detection, deterministic evidence sampling and redaction, strict response validation, fake-provider request contracts, privacy projections, and self-contained report generation. AI tests use a local fake completion client and never access a provider network.

## Compatibility

- `wechat-export` v0.1.0+ JSON exports
- Go 1.25+
- macOS / Linux / Windows

## License

MIT
