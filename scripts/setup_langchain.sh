#!/bin/bash

# Setup script for LangChain integration with EK-1
# This script installs Python dependencies and verifies the setup

set -e

echo "🚀 Setting up LangChain integration for EK-1..."

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is required but not installed."
    echo "Please install Python 3 first:"
    echo "  - Ubuntu/Debian: sudo apt-get install python3 python3-pip"
    echo "  - macOS: brew install python3"
    echo "  - Windows: Download from python.org"
    exit 1
fi

echo "✅ Python 3 found: $(python3 --version)"

# Check if pip is available
if ! command -v pip3 &> /dev/null; then
    echo "❌ pip3 is required but not installed."
    echo "Please install pip3 first:"
    echo "  - Ubuntu/Debian: sudo apt-get install python3-pip"
    echo "  - macOS: python3 -m ensurepip --upgrade"
    exit 1
fi

echo "✅ pip3 found: $(pip3 --version)"

# Install Python dependencies
echo "📦 Installing Python dependencies..."

pip3 install --user \
    langchain>=0.1.0 \
    langchain-community>=0.0.10 \
    langchain-experimental>=0.0.40 \
    sqlite3

echo "✅ Python dependencies installed"

# Verify installation
echo "🔍 Verifying installation..."

python3 -c "
import langchain
import sqlite3
import json
print('✅ All required modules imported successfully')
print(f'LangChain version: {langchain.__version__}')
"

# Make the script executable
chmod +x scripts/langchain_optimizer.py

echo ""
echo "🎉 LangChain integration setup complete!"
echo ""
echo "You can now use enhanced AI analysis with:"
echo "  - Automatic query optimization"
echo "  - Multi-step reasoning"
echo "  - Complex pattern detection"
echo ""
echo "API endpoints:"
echo "  POST /api/v1/chat/analyze       - Enhanced analysis (auto-detects LangChain need)"
echo "  POST /api/v1/chat/langchain     - Force LangChain analysis"
echo "  GET  /api/v1/chat/langchain/status - Check LangChain status"
echo ""
echo "Test it with: curl -X POST http://localhost:3000/api/v1/chat/langchain/status"
