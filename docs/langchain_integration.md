# LangChain Integration for EK-1 Data Analysis

This document explains how to integrate and use LangChain for optimized AI data querying in your EK-1 system.

## Overview

The LangChain integration provides:
- **Intelligent SQL Generation** - Automatic optimization of database queries
- **Multi-step Reasoning** - Complex analysis workflows with chain-of-thought
- **Pattern Detection** - Advanced correlation and trend analysis
- **Fallback Support** - Graceful degradation to basic analysis if LangChain unavailable

## Setup

### 1. Install Python Dependencies

```bash
# Run the setup script
chmod +x scripts/setup_langchain.sh
./scripts/setup_langchain.sh

# Or install manually
pip3 install langchain langchain-community sqlite3
```

### 2. Update Your Main.go

Replace the chat handler initialization with the LangChain-enhanced version:

```go
// OLD:
baseHandler := chat.NewSimpleHandler(aiClient, profileStore, checkInStore, notifsStore, signalsStore, chatHistory)
analysisHandler := chat.NewAnalysisHandler(baseHandler, db)

// NEW:
baseHandler := chat.NewSimpleHandler(aiClient, profileStore, checkInStore, notifsStore, signalsStore, chatHistory)
langchainHandler := chat.NewLangChainAnalysisHandler(baseHandler, db, "./ek1.db")
langchainHandler.RegisterRoutes(api)
```

### 3. Verify Installation

```bash
# Check LangChain status
curl -X GET http://localhost:3000/api/v1/chat/langchain/status

# Expected response:
{
  "enabled": true,
  "script_path": "/path/to/scripts/langchain_optimizer.py",
  "status": "Available"
}
```

## API Endpoints

### Enhanced Analysis (Auto-Detection)
**POST** `/api/v1/chat/analyze`

Automatically chooses between basic and LangChain analysis based on query complexity.

```bash
curl -X POST http://localhost:3000/api/v1/chat/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Find correlations between my stress levels and email volume",
    "time_frame": "month"
  }'
```

### Force LangChain Analysis
**POST** `/api/v1/chat/langchain`

Forces use of LangChain for complex analysis workflows.

```bash
curl -X POST http://localhost:3000/api/v1/chat/langchain \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Comprehensive analysis of productivity patterns",
    "time_frame": "quarter"
  }'
```

### LangChain Status
**GET** `/api/v1/chat/langchain/status`

Returns the availability and configuration of LangChain integration.

## Query Types That Benefit from LangChain

### 1. Correlation Analysis
```json
{
  "message": "Show correlations between stress levels and email volume over the last month"
}
```

### 2. Multi-dimensional Pattern Detection
```json
{
  "message": "Analyze relationships between sleep, productivity signals, and calendar density"
}
```

### 3. Trend Forecasting
```json
{
  "message": "Based on current patterns, predict areas that will need attention next week"
}
```

### 4. Complex Comparative Analysis
```json
{
  "message": "Compare my productivity patterns between high-stress and low-stress periods"
}
```

## Sample LangChain Response

```json
{
  "reply": "🚀 **LangChain Enhanced Analysis**\n\n## TOP 3 PRIORITY AREAS\n\n1. **EMAIL-STRESS CORRELATION** ⚠️\n   - Strong correlation (r=0.74) between inbox volume and stress\n   - Peak stress occurs 2-3 hours after email spikes\n   - Action: Implement email batching strategy\n\n2. **SLEEP-PRODUCTIVITY PATTERN**\n   - 28% productivity drop when sleep <6.5h\n   - Compounding effect over 3+ consecutive low-sleep days\n   - Action: Protect 7+ hour sleep window\n\n## 🔍 Query Strategy\nUsed multi-table joins to correlate biometric data with signal patterns, applied statistical analysis for correlation coefficients.\n\n## 📊 SQL Queries Generated\n**Query 1:**\n```sql\nSELECT DATE(b.recorded_at) as date,\n       AVG(b.stress_level) as avg_stress,\n       COUNT(s.id) as email_count\nFROM check_in_history b\nLEFT JOIN signals s ON DATE(b.recorded_at) = DATE(s.processed_at)\nWHERE b.recorded_at >= ? AND s.service_slug = 'gmail'\nGROUP BY DATE(b.recorded_at)\nORDER BY date DESC\n```\n\n*Powered by LangChain with optimized multi-step reasoning*",
  "langchain_data": {
    "query_explanation": "Generated optimized queries using statistical correlation analysis",
    "generated_queries": ["SELECT ...", "SELECT ..."],
    "timestamp": "2026-03-07T15:30:00Z"
  }
}
```

## Benefits Over Basic Analysis

| Feature | Basic Analysis | LangChain Analysis |
|---------|---------------|-------------------|
| **Query Optimization** | Pre-defined queries | AI-generated optimal SQL |
| **Multi-step Reasoning** | Single-pass analysis | Chain-of-thought workflows |
| **Pattern Detection** | Simple aggregations | Statistical correlations |
| **Adaptability** | Fixed analysis paths | Dynamic query generation |
| **Complex Insights** | Basic summaries | Deep relationship analysis |

## Troubleshooting

### Python Dependencies Missing
```bash
# Error: "LangChain analyzer not available"
# Solution: Install dependencies
pip3 install langchain langchain-community

# Verify
python3 -c "import langchain; print('OK')"
```

### Script Not Found
```bash
# Error: "failed to execute LangChain script"
# Solution: Ensure script is executable
chmod +x scripts/langchain_optimizer.py

# Verify path
ls -la scripts/langchain_optimizer.py
```

### Ollama Connection Issues
```bash
# Error: Connection to Ollama failed
# Solution: Ensure Ollama is running and accessible
export OLLAMA_HOST="http://localhost:11434"
curl $OLLAMA_HOST/api/version
```

## Performance Considerations

- **Cold Start**: First LangChain query takes 2-3s (model loading)
- **Warm Queries**: Subsequent queries complete in 500ms-1s
- **Fallback**: If LangChain fails, system falls back to basic analysis
- **Memory**: Python process uses ~200MB additional RAM
- **CPU**: Complex queries may use 50-100% CPU briefly

## Security Notes

- LangChain runs locally using your Ollama instance
- Database access is read-only through safe SQL generation
- No external API calls or data transmission
- Python script runs in isolated subprocess
- All analysis happens on your machine

## Future Enhancements

- [ ] Support for custom analysis templates
- [ ] Integration with additional LangChain tools
- [ ] Caching for repeated complex queries
- [ ] Real-time streaming analysis results
- [ ] Integration with external knowledge bases
