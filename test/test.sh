#!/bin/bash

# Configuration
URL="http://localhost:8080/v1/log"
TOTAL_REQUESTS=10000
CONCURRENCY_LIMIT=10 # Number of parallel requests at a time

echo "Starting regressive test against $URL ($TOTAL_REQUESTS total requests)..."

# JSON Payload
PAYLOAD='{
    "eventid": "abc",
    "tenant": "abc",
    "host": "h1",
    "loglevel": "ERROR",
    "message": "error: panic at line 12"
}'

# Loop to spin up concurrent background curl processes
for ((i=1; i<=TOTAL_REQUESTS; i++)); do
    # Execute curl in the background (-w extracts the HTTP status code)
    curl -s -o /dev/null -w "Request $i: HTTP %{http_code}\n" \
         -X POST \
         -H "Content-Type: application/json" \
         -d "$PAYLOAD" "$URL" &

    # Throttle background processes to respect the concurrency limit
    if [[ $(jobs -r -p | wc -l) -ge $CONCURRENCY_LIMIT ]]; then
        wait -n
    fi
done

# Wait for any remaining background tasks to finish
wait
echo "Regressive test complete."
