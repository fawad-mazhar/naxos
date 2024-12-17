#!/bin/bash

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

# Loop to send 100 POST requests
for i in {1..100}
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
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$random_data" http://localhost:8080/api/v1/jobs/sample-2/execute)

    # Print response
    echo "Job $i Response: $response"

    # Optional: add a small delay to avoid overwhelming the server
    sleep 0.1
done

echo "All jobs submitted!"
