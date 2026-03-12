#!/bin/bash

echo "🔧 Privacy Gateway Demo Setup"
echo "================================"

# Check if API key is provided as argument
if [ -n "$1" ]; then
    export OPENAI_API_KEY="$1"
    echo "✅ API key set from argument"
elif [ -n "$OPENAI_API_KEY" ]; then
    echo "✅ API key found in environment"
else
    echo "❌ No API key found!"
    echo ""
    echo "Usage options:"
    echo "1. ./demos/run_demo.sh sk-your-actual-key-here"
    echo "2. export OPENAI_API_KEY='sk-your-key' && ./demos/run_demo.sh"
    echo "3. OPENAI_API_KEY='sk-your-key' ./demos/run_demo.sh"
    echo ""
    echo "Get your API key from: https://platform.openai.com/account/api-keys"
    exit 1
fi

echo "🚀 Running Privacy Gateway demo..."
echo "Gateway URL: http://localhost/v1"
echo ""

python demos/openai_demo.py

echo ""
echo "💡 Check gateway logs for PII detection:"
echo "docker compose logs gateway --tail=10"