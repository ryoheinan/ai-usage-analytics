# Claude Code OTel Setup

Claude Code can export OpenTelemetry metrics and logs when telemetry is enabled.

Use this local endpoint with the app:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:4318/v1/metrics
export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=http://localhost:4318/v1/logs
```

Keep prompt, assistant response, raw API body, and tool-content logging disabled unless you have a separate privacy review. This app does not store prompt or response text, tool output, command output, or raw OTel payloads, but avoiding export is the safer default.

This project deliberately avoids Claude Code internal storage. Do not add importers for local Claude Code conversation logs or private tool state without a new design decision.
