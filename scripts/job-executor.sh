#!/bin/bash
# scripts/task-executor.sh

# Default values
JOB_TYPE="sample-1"  # Default to sequential execution
NUM_JOBS=10         # Default number of jobs

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --job-type|-j)
            JOB_TYPE="$2"
            shift 2
            ;;
        --num-jobs|-n)
            NUM_JOBS="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -j, --job-type TYPE    Job type to execute (sample-1 or sample-2)"
            echo "  -n, --num-jobs NUM     Number of jobs to execute (default: 10)"
            echo "  -h, --help             Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Validate job type
if [[ "$JOB_TYPE" != "sample-1" && "$JOB_TYPE" != "sample-2" ]]; then
    echo "Error: Invalid job type. Must be either 'sample-1' or 'sample-2'"
    exit 1
fi

# Function to generate a random string
random_string() {
    cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w ${1:-32} | head -n 1
}

# Function to generate a random number between 1 and 100
random_number() {
    echo $((RANDOM % 100 + 1))
}

# Function to generate a random boolean
random_boolean() {
    echo $((RANDOM % 2))
}

echo "Executing $NUM_JOBS jobs of type $JOB_TYPE..."

# Loop to send POST requests
for i in $(seq 1 $NUM_JOBS)
do
    # Generate random data
    random_data=$(cat <<EOF
{
    "id": "$(random_string 10)",
    "name": "Job $i",
    "priority": $(random_number),
    "is_urgent": $(random_boolean),
    "tags": ["tag$(random_number)", "tag$(random_number)", "tag$(random_number)"]
}
EOF
)

    # Send POST request
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$random_data" "http://localhost:8080/api/v1/jobs/$JOB_TYPE/execute")

    # Print response
    echo "Job $i Response: $response"

    # Add a small delay to avoid overwhelming the server
    sleep 0.1
done

echo "All jobs submitted!"
