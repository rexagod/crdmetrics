#!/usr/bin/env bash

set -o pipefail

# Timeout duration in seconds
TIMEOUT=${TIMEOUT:-300}

# Start the controller in the background with a timeout.
timeout --signal SIGINT "$TIMEOUT" make setup apply apply-testdata local &
PID=$!
echo -e "\n[Running controller in background with PID: $PID.]\n"

# Wait until telemetry is up.
while ! nc -z localhost "$CRSM_SELF_PORT"; do sleep 1; done

# Run tests. Comment this block out to use the command manually while still preserving cleanup behaviour on SIGINT.
echo -e "\n[Running tests with timeout: $TIMEOUT seconds.]\n"
"$GO" test -v -timeout "${TIMEOUT}s" -run "$TEST_RUN_PATTERN" -race "$TEST_PKG"
TEST_EXIT_CODE=$?

# Don't wait for the controller to finish and fail fast.
kill -2 $PID
wait $PID

# Trap on SIGINT to terminate the controller.
function terminate() {
  kill -2 $PID
  wait $PID
}
trap terminate SIGINT

# Wait for signal.
wait $PID

# Cleanup.
make delete delete-testdata

# Exit with the test exit code.
exit $TEST_EXIT_CODE
