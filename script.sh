#!/bin/bash


# Create standard Go project layout directories
mkdir -p \
    cmd/api \
    cmd/orchestrator \
    internal/api/handlers \
    internal/api/routes \
    internal/orchestrator \
    internal/worker \
    internal/storage/postgres \
    internal/storage/leveldb \
    internal/queue \
    internal/models \
    internal/config \
    pkg/logger \
    pkg/utils \
    scripts \
    deployments \
    test

# Create main application files
touch cmd/api/main.go
touch cmd/orchestrator/main.go

# Create internal package files
touch internal/api/handlers/job_handler.go
touch internal/api/handlers/status_handler.go
touch internal/api/routes/routes.go
touch internal/orchestrator/orchestrator.go
touch internal/orchestrator/job_processor.go
touch internal/worker/worker.go
touch internal/worker/task_executor.go
touch internal/storage/postgres/client.go
touch internal/storage/leveldb/client.go
touch internal/queue/rabbitmq.go
touch internal/models/job.go
touch internal/models/task.go
touch internal/models/status.go
touch internal/config/config.go

# Create config files
touch config.yaml
touch .env.example

# Create Docker related files
touch Dockerfile
touch docker-compose.yml

# Create documentation files
touch README.md
touch API.md

# Create deployment scripts
touch scripts/setup-db.sh
touch scripts/setup-rabbitmq.sh

# Initialize go module
go mod init github.com/fawad-mazhar/naxos

# Create a basic .gitignore
cat > .gitignore << EOL
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/

# Test binary, built with 'go test -c'
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Environment variables
.env

# IDE specific files
.idea/
.vscode/
.context/
*.swp
*.swo

# OS specific files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db

# LevelDB files
*.ldb

# Log files
*.log
EOL

echo "Project structure created successfully!"