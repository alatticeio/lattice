import json, os

history_file = os.environ.get('HISTORY_FILE', 'docs/benchmarks/history.json')
with open(history_file) as f:
    history = json.load(f)
history.append({
    "date": os.environ['TODAY'],
    "handshake_ms": int(os.environ['HANDSHAKE_MS']),
    "throughput_mbps": int(os.environ['THROUGHPUT_MBPS']),
    "api_p99_ms": int(os.environ['API_P99_MS']),
})
with open(history_file, 'w') as f:
    json.dump(history, f, indent=2)
