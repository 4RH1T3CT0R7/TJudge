#!/bin/bash
# Script to initialize Docker secrets for production deployment
# Usage: ./scripts/init-secrets.sh

set -e

SECRETS_DIR="./secrets"

echo "Initializing secrets directory..."

# Create secrets directory if it doesn't exist
mkdir -p "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"

# Function to generate random password
generate_password() {
    openssl rand -base64 32 | tr -d '/+=' | head -c 32
}

# Function to create secret file if it doesn't exist
create_secret() {
    local name=$1
    local file="$SECRETS_DIR/${name}.txt"

    if [ -f "$file" ]; then
        echo "Secret '$name' already exists, skipping..."
    else
        echo "Generating secret '$name'..."
        generate_password > "$file"
        chmod 600 "$file"
        echo "Created $file"
    fi
}

# Create required secrets
create_secret "db_password"
create_secret "jwt_secret"
create_secret "redis_password"

echo ""
echo "Secrets initialized successfully!"
echo ""
echo "IMPORTANT: Review and customize the secrets in $SECRETS_DIR/"
echo "For production, replace generated passwords with secure values."
echo ""
echo "Files created:"
ls -la "$SECRETS_DIR/"
echo ""
echo "Add $SECRETS_DIR to .gitignore to prevent committing secrets!"

# Check if secrets directory is in .gitignore
if ! grep -q "^secrets/" .gitignore 2>/dev/null; then
    echo ""
    echo "WARNING: 'secrets/' is not in .gitignore!"
    echo "Adding it now..."
    echo "" >> .gitignore
    echo "# Docker secrets" >> .gitignore
    echo "secrets/" >> .gitignore
    echo "Added 'secrets/' to .gitignore"
fi
