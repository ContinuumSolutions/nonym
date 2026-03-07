# EK-1 Data Analysis Usage Guide

The EK-1 system now supports intelligent data analysis using your local Ollama instance. This prevents hallucinations by grounding AI responses in your actual database records.

## How It Works

1. **Data Grounding**: The system queries your SQLite database for actual historical data
2. **Structured Analysis**: Ollama receives only real data with strict prompts to prevent fabrication
3. **Actionable Insights**: Responses focus on specific patterns and actionable recommendations

## API Endpoints

### Enhanced Chat with Auto-Detection

**POST `/chat`** - Now automatically detects analysis requests

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What should I prioritize today?",
    "history": []
  }'
```

### Dedicated Analysis Endpoint

**POST `/chat/analyze`** - For explicit data analysis

```bash
curl -X POST http://localhost:8080/chat/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Show me my stress patterns and urgent signals",
    "time_frame": "week",
    "focus": ["biometrics", "signals"]
  }'
```

#### Parameters:
- `message`: Your analysis question
- `time_frame`: `"week"` (default), `"month"`, or `"quarter"`
- `focus`: Array of `["signals", "biometrics", "notifications"]` (optional, analyzes all if empty)

## Example Queries

### Daily Prioritization
```json
{
  "message": "What should I prioritize today based on last week's data?",
  "time_frame": "week"
}
```

### Health Pattern Analysis
```json
{
  "message": "Are there concerning patterns in my sleep and stress?",
  "time_frame": "month",
  "focus": ["biometrics"]
}
```

### Communication Backlog
```json
{
  "message": "What emails and messages need urgent attention?",
  "time_frame": "week",
  "focus": ["signals"]
}
```

### Comprehensive Review
```json
{
  "message": "Give me a complete overview of areas needing attention",
  "time_frame": "week"
}
```

## Auto-Detection Keywords

The system automatically triggers data analysis when your message contains:

- "what should i prioritize"
- "what needs attention"
- "analyze my..."
- "show me patterns"
- "what's urgent"
- "summary" / "overview"
- "trending"

## Sample Response

```json
{
  "reply": "## Priority Analysis - Last 7 Days\n\n**TOP 3 AREAS NEEDING ATTENTION:**\n\n1. **HIGH STRESS PATTERN** ⚠️\n   - Average stress: 7.2/10 (concerning)\n   - 5 days above 6/10 threshold\n   - Action: Schedule stress reduction activities\n\n2. **Pending High-Priority Signals** (23 items)\n   - Gmail: 8 unread (3 high priority)\n   - Slack: 12 unread (5 high priority) \n   - Action: Review marked high-priority items first\n\n3. **Sleep Deficit**\n   - Average: 5.8h (below 6h target)\n   - 4 nights with <6h sleep\n   - Action: Implement earlier bedtime routine\n\n**IMMEDIATE ACTIONS:**\n- Address 3 high-priority Gmail messages today\n- Block 30min for stress management\n- Aim for 7h sleep tonight\n\nYour biometric shield is currently active due to stress levels.",
  "timestamp": "2026-03-07T10:30:00Z"
}
```

## Integration Notes

1. **No Hallucinations**: All insights are based on actual database records
2. **Context Aware**: Considers your current biometric state and preferences
3. **Actionable**: Provides specific, time-bound recommendations
4. **Secure**: Runs entirely on your local machine with Ollama

## Troubleshooting

- Ensure Ollama is running locally (`ollama serve`)
- Verify your model is available (`ollama list`)
- Check that you have historical data in your SQLite database
- For no data responses, try expanding the time frame parameter
